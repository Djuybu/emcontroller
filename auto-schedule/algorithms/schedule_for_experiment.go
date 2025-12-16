package algorithms

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	asmodel "emcontroller/auto-schedule/model"
	"emcontroller/models"
)

// -----------------------------------------------------------------------------
// Public API: được AppGroupController gọi trong thực nghiệm
// -----------------------------------------------------------------------------

// Tên thuật toán mới (dùng cho executor.go và switch-case bên dưới)
const PriorityAwareName string = "PriorityAwareGA"

// ScheduleForExperiment chạy các thuật toán scheduling (CompRand, BERand, Amaga,
// Ampga, Diktyoga, Mcssga, MTDP, PriorityAwareGA) trong chế độ "local experiment":
// - Không đụng đến VM, Kubernetes thật
// - Chỉ chọn một tập con ứng dụng (apps) để "accepted" dựa trên GA / random
// - Trả về danh sách models.AppInfo (chỉ dùng AppName, Priority, AutoScheduled)
func ScheduleForExperiment(algoName string, apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	if len(apps) == 0 {
		return nil, false, nil
	}

	switch algoName {
	case CompRandName:
		return runCompRandExperiment(apps)
	case BERandName:
		return runBERandExperiment(apps)
	case AmagaName:
		return runAmagaGAExperiment(apps)
	case AmpgaName:
		return runAmpgaGAExperiment(apps)
	case DiktyogaName:
		return runDiktyoGAGAExperiment(apps)
	case McssgaName:
		return runMcssgaGAExperiment(apps)
	case PriorityAwareName:
		return runPriorityAwareGAExperiment(apps)
	case MTDPName:
		return runMTDPExperiment(apps)
	default:
		return nil, false, fmt.Errorf("unknown algorithm for experiment: %s", algoName)
	}
}

// -----------------------------------------------------------------------------
// Baseline: random algorithms (giữ như cũ, làm baseline giống paper)
// -----------------------------------------------------------------------------

func runCompRandExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	r := newRand()
	indices := make([]int, 0, len(apps))

	for i := range apps {
		if r.Float64() < 0.5 { // ~50% app được chọn
			indices = append(indices, i)
		}
	}

	if len(indices) == 0 {
		indices = append(indices, r.Intn(len(apps)))
	}

	accepted := buildAcceptedAppInfos(apps, indices)
	return accepted, true, nil
}

func runBERandExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	r := newRand()
	indices := make([]int, 0, len(apps))

	for i := range apps {
		if r.Float64() < 0.7 { // ~70% app được chọn
			indices = append(indices, i)
		}
	}

	if len(indices) == 0 {
		indices = append(indices, r.Intn(len(apps)))
	}

	accepted := buildAcceptedAppInfos(apps, indices)
	return accepted, true, nil
}

// -----------------------------------------------------------------------------
// GA core – dùng chung cho Amaga, Ampga, Diktyoga, MCSSGA, MTDP, PriorityAwareGA
// -----------------------------------------------------------------------------

type appRes struct {
	CPU    float64
	MemMi  float64
	StorGi float64
}

type resourceCap struct {
	CPU    float64
	MemMi  float64
	StorGi float64
}

type gaParams struct {
	PopSize        int
	MaxGenerations int
	CrossoverProb  float64
	MutationProb   float64
	EliteCount     int
	TournamentSize int
	PenaltyWeight  float64 // mức phạt khi vượt quá capacity
}

// fitnessMode cho phép điều chỉnh cách tính fitness giữa các thuật toán GA
type fitnessMode int

const (
	fitMaxPriority               fitnessMode = iota // đơn mục tiêu: tối đa tổng priority
	fitPriorityWithLightPenalty                     // ưu tiên priority, phạt nhẹ
	fitPriorityWithStrongPenalty                    // ưu tiên priority, phạt nặng
	fitMultiCriteria                                // multi-criteria (MCS SGA)
	fitMTDP                                         // MTDP: temperature-aware + consolidation-aware
	fitPriorityAware                                // PriorityAwareGA: ưu tiên công bằng theo priority
)

// GA solution
type gaSolution struct {
	Genes   []bool // length = len(apps), true nếu app được chọn
	Fitness float64
}

// -----------------------------------------------------------------------------
// GA wrapper cho từng thuật toán
// -----------------------------------------------------------------------------

func runAmagaGAExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.6) // 60% tổng tài nguyên
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        40,
		MaxGenerations: 40,
		CrossoverProb:  0.8,
		MutationProb:   0.01,
		EliteCount:     2,
		TournamentSize: 3,
		PenaltyWeight:  5, // phạt vừa
	}
	sol := runGA(apps, resList, caps, params, fitMaxPriority)
	return solutionToAcceptedApps(apps, sol)
}

func runAmpgaGAExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.6)
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        50,
		MaxGenerations: 60,
		CrossoverProb:  0.9,
		MutationProb:   0.02, // mutation cao hơn
		EliteCount:     4,    // elitism mạnh hơn
		TournamentSize: 3,
		PenaltyWeight:  6,
	}
	sol := runGA(apps, resList, caps, params, fitPriorityWithLightPenalty)
	return solutionToAcceptedApps(apps, sol)
}

func runDiktyoGAGAExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.6)
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        45,
		MaxGenerations: 60,
		CrossoverProb:  0.85,
		MutationProb:   0.015,
		EliteCount:     3,
		TournamentSize: 3,
		PenaltyWeight:  7,
	}
	// Ở đây mình chưa implement grouping chi tiết như paper DiktyoGA,
	// nhưng có thể xem đây là GA "cẩn thận" hơn với penalty mạnh hơn.
	sol := runGA(apps, resList, caps, params, fitPriorityWithStrongPenalty)
	return solutionToAcceptedApps(apps, sol)
}

func runMcssgaGAExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.6)
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        60,
		MaxGenerations: 80,
		CrossoverProb:  0.9,
		MutationProb:   0.02,
		EliteCount:     4,
		TournamentSize: 4,
		PenaltyWeight:  8,
	}
	sol := runGA(apps, resList, caps, params, fitMultiCriteria)
	return solutionToAcceptedApps(apps, sol)
}

// PriorityAwareGA – thuật toán mới ưu tiên công bằng theo priority
func runPriorityAwareGAExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.6)
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        70,
		MaxGenerations: 80,
		CrossoverProb:  0.9,
		MutationProb:   0.02,
		EliteCount:     4,
		TournamentSize: 4,
		PenaltyWeight:  8,
	}
	sol := runGA(apps, resList, caps, params, fitPriorityAware)
	return solutionToAcceptedApps(apps, sol)
}

// runMTDPExperiment implements MTDP (Minimizing Total Data Center Power) algorithm
// Based on "Minimizing Data Center Cooling and Server Power Costs" (ISLPED 2009)
// Key features:
// 1. Temperature-aware task assignment (considers cooling cost)
// 2. Server consolidation (minimizes number of active servers)
// 3. Optimizes both server power and cooling power
func runMTDPExperiment(apps []models.K8sApp) ([]models.AppInfo, bool, error) {
	resList, caps, err := buildResourcesAndCaps(apps, 0.65) // Slightly higher capacity (65%) for better consolidation
	if err != nil {
		return nil, false, err
	}
	params := gaParams{
		PopSize:        70,        // Larger population for better exploration
		MaxGenerations: 100,       // More generations for convergence
		CrossoverProb:  0.92,      // High crossover for exploitation
		MutationProb:   0.015,     // Lower mutation for stability
		EliteCount:     6,         // More elite preservation
		TournamentSize: 5,         // Larger tournament for selection pressure
		PenaltyWeight:  10,        // Strong penalty for violations
	}
	sol := runGA(apps, resList, caps, params, fitMTDP)
	return solutionToAcceptedApps(apps, sol)
}

// -----------------------------------------------------------------------------
// GA implementation
// -----------------------------------------------------------------------------

func runGA(apps []models.K8sApp, resList []appRes, caps resourceCap, params gaParams, mode fitnessMode) gaSolution {
	n := len(apps)
	if n == 0 {
		return gaSolution{}
	}

	r := newRand()

	// 1. Khởi tạo population (tất cả đều feasible/repair được)
	pop := make([]gaSolution, params.PopSize)
	for i := range pop {
		genes := randomFeasibleGenes(r, resList, caps)
		fit := evaluateFitness(genes, apps, resList, caps, mode, params.PenaltyWeight)
		pop[i] = gaSolution{Genes: genes, Fitness: fit}
	}

	best := bestSolution(pop)

	// 2. Vòng lặp GA
	for gen := 0; gen < params.MaxGenerations; gen++ {
		newPop := make([]gaSolution, 0, params.PopSize)

		// Elitism: giữ lại vài cá thể tốt nhất
		elite := selectElite(pop, params.EliteCount)
		newPop = append(newPop, elite...)

		// Tạo phần còn lại
		for len(newPop) < params.PopSize {
			// chọn bố mẹ bằng tournament selection
			p1 := tournamentSelect(pop, params.TournamentSize, r)
			p2 := tournamentSelect(pop, params.TournamentSize, r)

			c1Genes := make([]bool, n)
			c2Genes := make([]bool, n)
			copy(c1Genes, p1.Genes)
			copy(c2Genes, p2.Genes)

			// crossover
			if r.Float64() < params.CrossoverProb {
				onePointCrossover(c1Genes, c2Genes, r)
			}

			// mutation + repair
			mutateGenes(c1Genes, params.MutationProb, r)
			repairGenes(c1Genes, resList, caps, r)

			mutateGenes(c2Genes, params.MutationProb, r)
			repairGenes(c2Genes, resList, caps, r)

			// đánh giá
			c1Fit := evaluateFitness(c1Genes, apps, resList, caps, mode, params.PenaltyWeight)
			c2Fit := evaluateFitness(c2Genes, apps, resList, caps, mode, params.PenaltyWeight)

			newPop = append(newPop, gaSolution{Genes: c1Genes, Fitness: c1Fit})
			if len(newPop) < params.PopSize {
				newPop = append(newPop, gaSolution{Genes: c2Genes, Fitness: c2Fit})
			}
		}

		pop = newPop
		curBest := bestSolution(pop)
		if curBest.Fitness > best.Fitness {
			best = curBest
		}
	}

	return best
}

// -----------------------------------------------------------------------------
// GA helpers
// -----------------------------------------------------------------------------

func newRand() *rand.Rand {
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

func randomFeasibleGenes(r *rand.Rand, resList []appRes, caps resourceCap) []bool {
	n := len(resList)
	genes := make([]bool, n)

	// random thứ tự
	idxs := r.Perm(n)

	var cpu, mem, stor float64

	for _, i := range idxs {
		// xác suất chọn
		if r.Float64() < 0.5 {
			cpuNew := cpu + resList[i].CPU
			memNew := mem + resList[i].MemMi
			storNew := stor + resList[i].StorGi

			if cpuNew <= caps.CPU && memNew <= caps.MemMi && storNew <= caps.StorGi {
				genes[i] = true
				cpu, mem, stor = cpuNew, memNew, storNew
			}
		}
	}

	// đảm bảo không rỗng
	if !anyTrue(genes) {
		genes[idxs[0]] = true
	}
	return genes
}

func anyTrue(b []bool) bool {
	for _, v := range b {
		if v {
			return true
		}
	}
	return false
}

func bestSolution(pop []gaSolution) gaSolution {
	if len(pop) == 0 {
		return gaSolution{}
	}
	best := pop[0]
	for _, s := range pop[1:] {
		if s.Fitness > best.Fitness {
			best = s
		}
	}
	return best
}

func selectElite(pop []gaSolution, k int) []gaSolution {
	if k <= 0 || len(pop) == 0 {
		return nil
	}
	if k > len(pop) {
		k = len(pop)
	}
	// copy indices
	idx := make([]int, len(pop))
	for i := range idx {
		idx[i] = i
	}
	// selection sort partial
	for i := 0; i < k; i++ {
		best := i
		for j := i + 1; j < len(idx); j++ {
			if pop[idx[j]].Fitness > pop[idx[best]].Fitness {
				best = j
			}
		}
		idx[i], idx[best] = idx[best], idx[i]
	}
	out := make([]gaSolution, 0, k)
	for i := 0; i < k; i++ {
		out = append(out, pop[idx[i]])
	}
	return out
}

func tournamentSelect(pop []gaSolution, tSize int, r *rand.Rand) gaSolution {
	best := pop[r.Intn(len(pop))]
	for i := 1; i < tSize; i++ {
		candidate := pop[r.Intn(len(pop))]
		if candidate.Fitness > best.Fitness {
			best = candidate
		}
	}
	return best
}

func onePointCrossover(a, b []bool, r *rand.Rand) {
	if len(a) != len(b) || len(a) < 2 {
		return
	}
	point := r.Intn(len(a)-1) + 1 // [1, n-1]
	for i := point; i < len(a); i++ {
		a[i], b[i] = b[i], a[i]
	}
}

func mutateGenes(genes []bool, p float64, r *rand.Rand) {
	for i := range genes {
		if r.Float64() < p {
			genes[i] = !genes[i]
		}
	}
}

func repairGenes(genes []bool, resList []appRes, caps resourceCap, r *rand.Rand) {
	// nếu không vi phạm thì thôi
	cpu, mem, stor := totalResources(genes, resList)
	if cpu <= caps.CPU && mem <= caps.MemMi && stor <= caps.StorGi {
		if anyTrue(genes) {
			return
		}
	}

	// nếu vượt quá capacity hoặc rỗng, ta random tắt bớt hoặc bật ít nhất một cái
	n := len(genes)
	if n == 0 {
		return
	}

	// nếu rỗng → bật ngẫu nhiên 1 gene
	if !anyTrue(genes) {
		genes[r.Intn(n)] = true
		return
	}

	// nếu vượt → tắt dần đến khi đủ
	for {
		cpu, mem, stor = totalResources(genes, resList)
		if cpu <= caps.CPU && mem <= caps.MemMi && stor <= caps.StorGi {
			break
		}
		i := r.Intn(n)
		genes[i] = false
		if !anyTrue(genes) {
			genes[i] = true // tránh rỗng
			break
		}
	}
}

func totalResources(genes []bool, resList []appRes) (cpu, mem, stor float64) {
	for i, g := range genes {
		if g {
			cpu += resList[i].CPU
			mem += resList[i].MemMi
			stor += resList[i].StorGi
		}
	}
	return
}

// -----------------------------------------------------------------------------
// Fitness function
// -----------------------------------------------------------------------------

func evaluateFitness(genes []bool, apps []models.K8sApp, resList []appRes, caps resourceCap, mode fitnessMode, penaltyWeight float64) float64 {
	// tổng priority & resource
	var sumPri float64
	for i, g := range genes {
		if g {
			sumPri += float64(apps[i].Priority)
		}
	}
	if sumPri == 0 {
		return 0
	}

	cpu, mem, stor := totalResources(genes, resList)

	// normalized resource usage
	var cpuRatio, memRatio, storRatio float64
	if caps.CPU > 0 {
		cpuRatio = cpu / caps.CPU
	}
	if caps.MemMi > 0 {
		memRatio = mem / caps.MemMi
	}
	if caps.StorGi > 0 {
		storRatio = stor / caps.StorGi
	}
	maxRatio := math.Max(cpuRatio, math.Max(memRatio, storRatio))

	// base fitness theo mode
	switch mode {
	case fitMaxPriority:
		// ưu tiên priority, phạt nhẹ vi phạm
		if maxRatio <= 1 {
			return sumPri
		}
		return sumPri - penaltyWeight*sumPri*(maxRatio-1)

	case fitPriorityWithLightPenalty:
		if maxRatio <= 1 {
			// thêm chút "bonus" cho sử dụng tài nguyên cao nhưng vẫn không vi phạm
			return sumPri * (0.8 + 0.2*maxRatio)
		}
		return sumPri - penaltyWeight*sumPri*(maxRatio-1)

	case fitPriorityWithStrongPenalty:
		if maxRatio <= 1 {
			return sumPri * (0.7 + 0.3*maxRatio)
		}
		return sumPri - penaltyWeight*sumPri*(maxRatio-1.5)

	case fitMultiCriteria:
		// Multi-criteria đơn giản:
		//   Fitness = w1 * PriorityNormalized - w2 * ResourceSlack - w3 * Imbalance
		// Ở đây đơn giản hóa, nhưng vẫn khác những mode trên.
		if maxRatio > 1 {
			// vi phạm nặng
			return sumPri - penaltyWeight*sumPri*(maxRatio-1.2)
		}

		// Priority normalized (giả sử maxPriority ~ 10 * len(apps))
		maxPossible := float64(10 * len(apps))
		priNorm := sumPri / maxPossible

		// Resource slack (càng gần 1 càng tốt, nhưng không vượt)
		slack := 1.0 - maxRatio // 0..1

		// Imbalance giữa CPU/Mem/Stor
		imbalance := math.Abs(cpuRatio-memRatio) + math.Abs(cpuRatio-storRatio) + math.Abs(memRatio-storRatio)

		w1, w2, w3 := 0.6, 0.3, 0.1
		return w1*priNorm + w2*slack - w3*imbalance

	case fitMTDP:
		// MTDP fitness: Temperature-aware + Consolidation-aware
		// Based on "Minimizing Data Center Cooling and Server Power Costs" (ISLPED 2009)
		// Fitness = Priority - ServerPowerCost - CoolingPowerCost
		if maxRatio > 1 {
			// Heavy penalty for resource violations
			return sumPri - penaltyWeight*sumPri*(maxRatio-1.5)
		}

		// Count active "servers" (simplified: count apps as servers)
		// In real scenario, this would count actual servers/chassis
		activeServers := 0
		for _, g := range genes {
			if g {
				activeServers++
			}
		}
		totalApps := len(genes)

		// Server consolidation bonus: fewer active servers = better
		// Normalize: consolidationRatio = 1 - (activeServers / totalApps)
		// More consolidation (fewer servers) = higher bonus
		consolidationRatio := 0.0
		if totalApps > 0 {
			consolidationRatio = 1.0 - float64(activeServers)/float64(totalApps)
		}

		// Temperature-aware penalty (simulated)
		// Higher resource usage = higher temperature = higher cooling cost
		// Use maxRatio as proxy for temperature (higher utilization = hotter)
		avgTemp := 20.0 + maxRatio*10.0 // Simulated: 20-30°C based on utilization
		coolingCost := 0.0
		if avgTemp > 25.0 {
			// Cooling needed when temp > 25°C
			excessTemp := avgTemp - 25.0
			coolingCost = 0.05 * excessTemp * excessTemp // Quadratic cooling cost
		}

		// Server power cost (proportional to active servers)
		serverPowerCost := 0.02 * float64(activeServers)

		// Priority normalized
		maxPossible := float64(10 * len(apps))
		priNorm := sumPri / maxPossible

		// MTDP fitness: maximize priority, minimize power (server + cooling), maximize consolidation
		// w1*Priority + w2*Consolidation - w3*ServerPower - w4*CoolingPower
		w1, w2, w3, w4 := 0.5, 0.3, 0.1, 0.1
		fitness := w1*priNorm + w2*consolidationRatio - w3*serverPowerCost - w4*coolingCost

		// Scale to similar range as other algorithms
		return fitness * maxPossible * 2.0

	case fitPriorityAware:
		// PriorityAwareGA:
		// - Vẫn ưu tiên tổng priority
		// - Thêm term coverage theo priority để tránh priority nào bị acceptance = 0
		// - Phạt nếu coverage(priority thấp) > coverage(priority cao)

		// Nếu vi phạm capacity → phạt nặng
		if maxRatio > 1 {
			return sumPri - penaltyWeight*sumPri*(maxRatio-1.0)
		}

		// 1. Base priority: thích dùng tài nguyên cao nhưng không vi phạm
		basePriority := sumPri * (0.7 + 0.3*maxRatio)

		// 2. Thống kê theo priority
		minPri := asmodel.MinPriority
		maxPri := asmodel.MaxPriority

		priTotal := make(map[int]int)
		priAccepted := make(map[int]int)
		totalApps := len(genes)
		totalAccepted := 0

		for i, g := range genes {
			pr := apps[i].Priority
			priTotal[pr]++
			if g {
				priAccepted[pr]++
				totalAccepted++
			}
		}

		var overallAcc float64
		if totalApps > 0 {
			overallAcc = float64(totalAccepted) / float64(totalApps)
		}

		// 3. Coverage từng priority
		coverage := make(map[int]float64)
		for pr := minPri; pr <= maxPri; pr++ {
			if priTotal[pr] > 0 {
				coverage[pr] = float64(priAccepted[pr]) / float64(priTotal[pr])
			}
		}

		// 4. CoverageScore: priority cao có weight lớn hơn
		priorityCoverageScore := 0.0
		for pr := minPri; pr <= maxPri; pr++ {
			cov, ok := coverage[pr]
			if !ok {
				continue
			}
			if maxPri == minPri {
				priorityCoverageScore += cov
			} else {
				// pr gần maxPri → t ~1 → weightCoverage ~1.5
				t := float64(pr-minPri) / float64(maxPri-minPri) // 0..1
				weightCoverage := 1.0 + 0.5*t                    // 1..1.5
				priorityCoverageScore += cov * weightCoverage
			}
		}

		// 5. Penalty nếu coverage(priority thấp) > coverage(priority cao)
		priorityViolationPenalty := 0.0
		for high := minPri; high <= maxPri; high++ {
			covHigh, okHigh := coverage[high]
			if !okHigh {
				continue
			}
			for low := minPri; low < high; low++ {
				covLow, okLow := coverage[low]
				if !okLow {
					continue
				}
				if covLow > covHigh {
					priorityViolationPenalty += covLow - covHigh
				}
			}
		}

		// 6. Kết hợp
		// Các hệ số scale này chủ yếu để:
		// - reward coverage & overallAcc đủ mạnh
		// - penalty violation đủ đau để tránh priority cao bị bỏ đói
		fitness := 0.0
		fitness += basePriority
		fitness += 100.0 * priorityCoverageScore
		fitness += 50.0 * overallAcc
		fitness -= 200.0 * priorityViolationPenalty

		return fitness

	default:
		return sumPri
	}
}

// -----------------------------------------------------------------------------
// Resources helper – lấy AppResources từ model.Application
// -----------------------------------------------------------------------------

func buildResourcesAndCaps(apps []models.K8sApp, capRatio float64) ([]appRes, resourceCap, error) {
	appMap, err := asmodel.GenerateApplications(apps)
	if err != nil {
		return nil, resourceCap{}, err
	}

	resList := make([]appRes, len(apps))
	var totalCPU, totalMem, totalStor float64

	for i, inApp := range apps {
		app, ok := appMap[inApp.Name]
		if !ok {
			// nếu không tìm thấy (hiếm), cho tài nguyên rất nhỏ
			resList[i] = appRes{CPU: 0.1, MemMi: 10, StorGi: 0.1}
			totalCPU += 0.1
			totalMem += 10
			totalStor += 0.1
			continue
		}
		cpu := app.Resources.CpuCore
		mem := app.Resources.Memory
		stor := app.Resources.Storage

		resList[i] = appRes{CPU: cpu, MemMi: mem, StorGi: stor}
		totalCPU += cpu
		totalMem += mem
		totalStor += stor
	}

	if capRatio <= 0 || capRatio > 1 {
		capRatio = 0.6
	}

	caps := resourceCap{
		CPU:    totalCPU * capRatio,
		MemMi:  totalMem * capRatio,
		StorGi: totalStor * capRatio,
	}
	return resList, caps, nil
}

// -----------------------------------------------------------------------------
// Mapping solution → acceptedApps (AppInfo) – cái executor.go đọc
// -----------------------------------------------------------------------------

func solutionToAcceptedApps(apps []models.K8sApp, sol gaSolution) ([]models.AppInfo, bool, error) {
	if len(sol.Genes) == 0 || len(apps) == 0 {
		return nil, false, nil
	}
	indices := make([]int, 0)
	for i, g := range sol.Genes {
		if g {
			indices = append(indices, i)
		}
	}
	if len(indices) == 0 {
		// không chọn được app nào → coi như unusable
		return nil, false, nil
	}
	accepted := buildAcceptedAppInfos(apps, indices)
	return accepted, true, nil
}

// chuyển từ apps + indices → []AppInfo
func buildAcceptedAppInfos(apps []models.K8sApp, indices []int) []models.AppInfo {
	accepted := make([]models.AppInfo, 0, len(indices))

	for _, idx := range indices {
		if idx < 0 || idx >= len(apps) {
			continue
		}
		app := apps[idx]

		var ai models.AppInfo
		ai.AppName = app.Name
		ai.Priority = app.Priority
		ai.AutoScheduled = true

		accepted = append(accepted, ai)
	}

	return accepted
}
