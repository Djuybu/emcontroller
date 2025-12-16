package algorithms

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/KeepTheBeats/routing-algorithms/random"
	"github.com/astaxie/beego"
	chart "github.com/wcharczuk/go-chart"

	asmodel "emcontroller/auto-schedule/model"
)

/*
 * MTDP – Minimizing Total Data Center Power (Fairness-Aware Edition)
 *
 * PROBLEM FIX:
 * - Trước đây: Priority thấp bị bỏ qua do Reward < Energy Cost hoặc bị Priority cao chèn ép.
 * - Bây giờ:
 * 1. Base Reward lớn: Đảm bảo Accept Priority 1 vẫn tốt hơn là Reject.
 * 2. Diversity Penalty: Phạt nặng nếu trong quần thể thiếu vắng bất kỳ mức Priority nào.
 * 3. Repair Logic: Cố gắng dàn trải để priority nào cũng có phần.
 */

type Mtdp struct {
	ChromosomesCount      int
	IterationCount        int
	CrossoverProbability  float64
	MutationProbability   float64
	StopNoUpdateIteration int
	CurNoUpdateIteration  int
	AvgTemperature        float64
	OptimalTemperature    float64
	BestFitnessRecords    []float64
	BestSolnRecords       []asmodel.Solution
	BestFitnessEachIter   []float64
}

func NewMtdp(chromosomesCount, iterationCount int, crossoverProbability, mutationProbability float64, stopNoUpdateIteration int) *Mtdp {
	return &Mtdp{
		ChromosomesCount:      chromosomesCount,
		IterationCount:        iterationCount,
		CrossoverProbability:  crossoverProbability,
		MutationProbability:   mutationProbability,
		StopNoUpdateIteration: stopNoUpdateIteration,
		AvgTemperature:        25.0,
		OptimalTemperature:    25.0,
	}
}

func (m *Mtdp) Schedule(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string) (asmodel.Solution, error) {
	beego.Info("Using scheduling algorithm:", MTDPName, "(FAIRNESS MODE)")
	m.calculateAvgTemperature(clouds)

	initPopulation := m.initialize(clouds, apps, appsOrder)
	currentPopulation := m.selectionOperator(clouds, apps, initPopulation)

	for iter := 1; iter <= m.IterationCount; iter++ {
		currentPopulation = m.crossoverOperator(clouds, apps, appsOrder, currentPopulation)
		currentPopulation = m.mutationOperator(clouds, apps, appsOrder, currentPopulation)
		currentPopulation = m.selectionOperator(clouds, apps, currentPopulation)
		if m.CurNoUpdateIteration > m.StopNoUpdateIteration {
			beego.Info("MTDP converged early at iteration:", iter)
			break
		}
	}

	if len(m.BestSolnRecords) == 0 {
		return asmodel.GenEmptySoln(), fmt.Errorf("MTDP: no solution recorded")
	}

	best := m.BestSolnRecords[len(m.BestSolnRecords)-1]

	// Final Polish: Cố gắng nhét nốt những priority còn thiếu
	best = m.ensureFairness(best, clouds, apps, appsOrder)

	return best, nil
}

func (m *Mtdp) calculateAvgTemperature(clouds map[string]asmodel.Cloud) {
	if len(clouds) == 0 {
		m.AvgTemperature = 25.0
		return
	}
	var sum float64
	var cnt int
	for _, c := range clouds {
		if c.TemperatureC > 0 {
			sum += c.TemperatureC
			cnt++
		}
	}
	if cnt > 0 {
		m.AvgTemperature = sum / float64(cnt)
	} else {
		m.AvgTemperature = 25.0
	}
}

func (m *Mtdp) initialize(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string) []asmodel.Solution {
	pop := make([]asmodel.Solution, 0, m.ChromosomesCount)
	for i := 0; i < m.ChromosomesCount; i++ {
		pop = append(pop, CmpRandomAcceptMostSolution(clouds, apps, appsOrder))
	}
	return pop
}

// ================= FITNESS: CHÌA KHÓA ĐỂ KHẮC PHỤC PRIORITY 0 =================

func (m *Mtdp) Fitness(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, chromosome asmodel.Solution) float64 {
	const (
		// Thưởng SÀN: Bất kỳ app nào được chạy cũng được +1000 điểm.
		// Điều này làm cho Priority 1 (1001đ) và Priority 10 (1010đ) không quá chênh lệch.
		// Priority 1 sẽ không bị coi là "rác" nữa.
		BaseReward = 1000.0

		// Hệ số Priority: Vẫn giữ để Priority cao có chút lợi thế
		PriorityWeight = 10.0

		// Phạt khi Reject: Rất nặng
		RejectionPenalty = 5000.0

		// Phạt năng lượng: Nhỏ hơn BaseReward để không bao giờ làm Priority 1 bị lỗ
		EnergyPenalty = 1.0 
	)

	var fitness float64
	
	// Theo dõi xem có đủ bộ Priority từ 1-10 không
	acceptedPriorities := make(map[int]int) 

	for appName, app := range apps {
		gene, ok := chromosome.AppsSolution[appName]
		if !ok { return -1e9 }

		if !gene.Accepted {
			// Reject -> Phạt nặng
			fitness -= RejectionPenalty
		} else {
			// Accept -> Tính điểm
			
			// 1. Base Reward (Quan trọng nhất để cứu Priority thấp)
			score := BaseReward

			// 2. Priority Bonus
			score += float64(app.Priority) * PriorityWeight

			// 3. Energy Cost
			cpu := gene.AllocatedCpuCore
			if cpu <= 0 { cpu = 1 }
			
			temp := m.AvgTemperature
			if c, ok2 := clouds[gene.TargetCloudName]; ok2 && c.TemperatureC > 0 {
				temp = c.TemperatureC
			}
			
			delta := temp - m.OptimalTemperature
			if delta < 0 { delta = 0 }
			
			appEnergy := float64(cpu) * (1.0 + (delta*delta)/100.0)
			
			score -= EnergyPenalty * appEnergy

			fitness += score
			
			// Ghi nhận Priority này đã có mặt
			acceptedPriorities[app.Priority]++
		}
	}

	// --- DIVERSITY BONUS (Thưởng Đa Dạng) ---
	// Nếu giải pháp này chứa đủ các Priority từ 1 đến 10, thưởng thêm điểm.
	// Nếu thiếu Priority nào (đặc biệt là Priority thấp), trừ điểm.
	
	missingCount := 0
	for p := asmodel.MinPriority; p <= asmodel.MaxPriority; p++ {
		if acceptedPriorities[p] == 0 {
			missingCount++
		}
	}
	
	// Phạt cực nặng cho mỗi Priority bị vắng mặt hoàn toàn (0%)
	// Điều này ép GA phải tìm cách giữ lại ít nhất 1 app cho mỗi Priority
	fitness -= float64(missingCount) * 10000.0 

	return fitness
}

// ================= FAIRNESS REPAIR =================

func (m *Mtdp) ensureFairness(sol asmodel.Solution, clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string) asmodel.Solution {
	fixed := asmodel.SolutionCopy(sol)
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Lặp qua từng Priority để đảm bảo "ai cũng có quà"
	for pri := asmodel.MinPriority; pri <= asmodel.MaxPriority; pri++ {
		
		// Đếm số lượng hiện tại
		count := 0
		var missingApps []string
		for name, app := range apps {
			if app.Priority == pri {
				if fixed.AppsSolution[name].Accepted {
					count++
				} else {
					missingApps = append(missingApps, name)
				}
			}
		}

		// Nếu Priority này đã có app chạy -> Tốt, bỏ qua
		if count > 0 || len(missingApps) == 0 {
			continue
		}

		// NẾU CHƯA CÓ (Count == 0): Phải cứu bằng mọi giá
		// Chiến thuật: Hy sinh Priority cao (đã có nhiều) để cứu Priority thấp (đang là 0)
		
		rng.Shuffle(len(missingApps), func(i, j int) { missingApps[i], missingApps[j] = missingApps[j], missingApps[i] })
		
		appToRescue := missingApps[0] // Chọn 1 app để cứu
		
		// Tìm Cloud nào đó
		var allClouds []string
		for c := range clouds { allClouds = append(allClouds, c) }
		
		rescued := false
		for _, cName := range allClouds {
			// 1. Thử nhét vào
			trySol := asmodel.SolutionCopy(fixed)
			gene := trySol.AppsSolution[appToRescue]
			gene.Accepted = true
			gene.TargetCloudName = cName
			trySol.AppsSolution[appToRescue] = gene
			
			// Tạo danh sách ưu tiên giả: Đưa appToRescue lên đầu
			fakeOrder := []string{appToRescue}
			for _, n := range appsOrder { if n != appToRescue { fakeOrder = append(fakeOrder, n) } }

			if ref, ok := CmpRefineSoln(clouds, apps, fakeOrder, trySol); ok {
				if ref.AppsSolution[appToRescue].Accepted {
					fixed = ref
					rescued = true
					break
				}
			}
			
			// 2. Nếu không nhét được -> Tắt bớt 1-2 app Priority CAO (>=7) trên cloud đó
			// Logic: "Lấy của người giàu chia cho người nghèo"
			if !rescued {
				var richVictims []string
				for name, g := range trySol.AppsSolution {
					if g.Accepted && g.TargetCloudName == cName && apps[name].Priority >= 7 {
						richVictims = append(richVictims, name)
					}
				}
				
				if len(richVictims) > 0 {
					// Tắt 1 ông lớn
					victim := richVictims[rng.Intn(len(richVictims))]
					vGene := trySol.AppsSolution[victim]
					vGene.Accepted = false
					vGene.TargetCloudName = ""
					vGene.AllocatedCpuCore = 0
					trySol.AppsSolution[victim] = vGene
					
					// Thử lại
					if ref, ok := CmpRefineSoln(clouds, apps, fakeOrder, trySol); ok {
						if ref.AppsSolution[appToRescue].Accepted {
							fixed = ref
							rescued = true
							break
						}
					}
				}
			}
			if rescued { break }
		}
	}
	return fixed
}

func (m *Mtdp) selectionOperator(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, population []asmodel.Solution) []asmodel.Solution {
	fitnesses := make([]float64, len(population))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < len(population); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			f := m.Fitness(clouds, apps, population[idx])
			mu.Lock()
			fitnesses[idx] = f
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	newPopulation := make([]asmodel.Solution, 0, m.ChromosomesCount)
	pickHelper := make([]int, len(fitnesses))
	bestFitThisIter := -math.MaxFloat64
	bestFitThisIterIdx := 0

	for i := 0; i < m.ChromosomesCount; i++ {
		picked := random.RandomPickN(pickHelper, 2)
		selIdx := picked[0]
		if fitnesses[picked[1]] > fitnesses[picked[0]] {
			selIdx = picked[1]
		}
		newPopulation = append(newPopulation, asmodel.SolutionCopy(population[selIdx]))

		if fitnesses[selIdx] > bestFitThisIter {
			bestFitThisIter = fitnesses[selIdx]
			bestFitThisIterIdx = selIdx
		}
	}

	var bestFitAll float64
	var bestSolnAll asmodel.Solution
	if len(m.BestFitnessRecords) == 0 {
		bestFitAll = bestFitThisIter
		bestSolnAll = population[bestFitThisIterIdx]
		m.CurNoUpdateIteration = 0
	} else {
		bestFitAll = m.BestFitnessRecords[len(m.BestFitnessRecords)-1]
		bestSolnAll = m.BestSolnRecords[len(m.BestSolnRecords)-1]
		if bestFitThisIter > bestFitAll {
			bestFitAll = bestFitThisIter
			bestSolnAll = population[bestFitThisIterIdx]
			m.CurNoUpdateIteration = 0
		} else {
			m.CurNoUpdateIteration++
		}
	}
	m.BestFitnessEachIter = append(m.BestFitnessEachIter, bestFitThisIter)
	m.BestFitnessRecords = append(m.BestFitnessRecords, bestFitAll)
	m.BestSolnRecords = append(m.BestSolnRecords, asmodel.SolutionCopy(bestSolnAll))

	return newPopulation
}

func (m *Mtdp) crossoverOperator(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string, population []asmodel.Solution) []asmodel.Solution {
	if len(apps) <= 1 {
		return population
	}
	var idxNeed []int
	for i := 0; i < len(population); i++ {
		if random.RandomFloat64(0, 1) < m.CrossoverProbability {
			idxNeed = append(idxNeed, i)
		}
	}

	var crossovered []asmodel.Solution
	whetherCrossover := make([]bool, len(population))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for len(idxNeed) > 1 {
		p1, p2 := random.RandomInt(0, len(idxNeed)-1), random.RandomInt(0, len(idxNeed)-1)
		if p1 == p2 {
			p2 = (p1 + 1) % len(idxNeed)
		}

		idx1, idx2 := idxNeed[p1], idxNeed[p2]
		whetherCrossover[idx1], whetherCrossover[idx2] = true, true

		wg.Add(1)
		go func(a, b asmodel.Solution) {
			defer wg.Done()
			n1, n2 := CmpAllPossTwoPointCrossover(a, b, clouds, apps, appsOrder)
			mu.Lock()
			crossovered = append(crossovered, n1, n2)
			mu.Unlock()
		}(asmodel.SolutionCopy(population[idx1]), asmodel.SolutionCopy(population[idx2]))

		high, low := p1, p2
		if p2 > p1 {
			high, low = p2, p1
		}
		idxNeed = append(idxNeed[:high], idxNeed[high+1:]...)
		idxNeed = append(idxNeed[:low], idxNeed[low+1:]...)
	}
	wg.Wait()
	for i := 0; i < len(population); i++ {
		if !whetherCrossover[i] {
			crossovered = append(crossovered, asmodel.SolutionCopy(population[i]))
		}
	}
	return crossovered
}

func (m *Mtdp) mutationOperator(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string, population []asmodel.Solution) []asmodel.Solution {
	mutated := make([]asmodel.Solution, len(population))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < len(population); i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				newChrom := asmodel.GenEmptySoln()
				for appName, oriGene := range population[idx].AppsSolution {
					if random.RandomFloat64(0, 1) < m.MutationProbability {
						newChrom.AppsSolution[appName] = m.geneMutate(clouds, oriGene)
					} else {
						newChrom.AppsSolution[appName] = asmodel.SasCopy(oriGene)
					}
				}
				if chrom, ok := CmpRefineSoln(clouds, apps, appsOrder, newChrom); ok {
					mu.Lock()
					mutated[idx] = chrom
					mu.Unlock()
					break
				}
			}
		}(i)
	}
	wg.Wait()
	return mutated
}

func (m *Mtdp) geneMutate(clouds map[string]asmodel.Cloud, ori asmodel.SingleAppSolution) asmodel.SingleAppSolution {
	mut := asmodel.SasCopy(asmodel.RejSoln)
	cloudsToPick := asmodel.CloudMapCopy(clouds)
	if ori.Accepted {
		delete(cloudsToPick, ori.TargetCloudName)
	}
	mut.Accepted = random.RandomInt(0, 1) == 0
	if mut.Accepted {
		mut.TargetCloudName, _ = randomCloudMapPick(cloudsToPick)
	}
	return mut
}

func (m *Mtdp) DrawEvoChart() {
	drawChartFunc := func(res http.ResponseWriter, r *http.Request) {
		var xValuesAllBest []float64
		for i := range m.BestFitnessRecords {
			xValuesAllBest = append(xValuesAllBest, float64(i))
		}

		graph := chart.Chart{
			Title: "MTDP Evolution",
			XAxis: chart.XAxis{
				Name:           "Iteration Number",
				NameStyle:      chart.StyleShow(),
				Style:          chart.StyleShow(),
				ValueFormatter: func(v interface{}) string { return strconv.FormatInt(int64(v.(float64)), 10) },
			},
			YAxis: chart.YAxis{
				AxisType:  chart.YAxisSecondary,
				Name:      "Fitness",
				NameStyle: chart.StyleShow(),
				Style:     chart.StyleShow(),
			},
			Background: chart.Style{Padding: chart.Box{Top: 50, Left: 20}},
			Series: []chart.Series{
				chart.ContinuousSeries{
					Name:    "Best Fitness (Cumulative)",
					XValues: xValuesAllBest,
					YValues: m.BestFitnessRecords,
				},
				chart.ContinuousSeries{
					Name:    "Best Fitness (Current Iter)",
					XValues: xValuesAllBest,
					YValues: m.BestFitnessEachIter,
					Style: chart.Style{
						Show:            true,
						StrokeDashArray: []float64{5.0, 3.0, 2.0, 3.0},
						StrokeWidth:     1,
					},
				},
			},
		}
		graph.Elements = []chart.Renderable{chart.LegendThin(&graph)}
		res.Header().Set("Content-Type", "image/png")
		if err := graph.Render(chart.PNG, res); err != nil {
			log.Println("DrawEvoChart error:", err)
		}
	}

	http.HandleFunc("/", drawChartFunc)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Println("ListenAndServe error:", err)
	}
}