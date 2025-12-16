package algorithms

import (
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"sync"

	"github.com/KeepTheBeats/routing-algorithms/random"
	"github.com/astaxie/beego"
	chart "github.com/wcharczuk/go-chart"

	asmodel "emcontroller/auto-schedule/model"
	"emcontroller/models"
)

/*
PriorityAwareGA – Hybrid Priority-Aware Genetic Algorithm

Mục tiêu:
- Giữ cấu trúc GA giống MCSSGA (init → crossover → mutation → selection).
- Tối ưu các metric:
    + AppAcceptanceRate (tổng)
    + AppPriorityWeightedAcceptanceRate
    + Per-priority App Acceptance Rate (hạn chế = 0)
- Priority “nhẹ” hơn MCSSGA:
    + Không nhân trực tiếp non-priority fitness với Priority (1..10).
    + Thay vào đó dùng weight normalized trong [1, 1+PriorityBonusScale] (ví dụ [1, 2]).
- Thêm fairness term theo priority:
    + Coverage[p] = accepted_p / total_p.
    + Reward phủ đều các priority.
    + Penalty nếu priority thấp có coverage > priority cao (vi phạm thứ tự).
*/

// Các hệ số điều chỉnh fitness (có thể tune lại nếu cần).
const (
	// Độ quan trọng của core service fitness (computation + network).
	coreFitnessWeight float64 = 1.0

	// Độ quan trọng của coverage theo priority (khuyến khích mỗi priority đều được phục vụ).
	priorityCoverageWeight float64 = 0.7

	// Độ quan trọng của overall acceptance rate.
	overallAcceptanceWeight float64 = 0.5

	// Mức phạt nếu coverage của priority thấp > priority cao.
	priorityViolationPenaltyWeight float64 = 0.8

	// Khoảng bonus cho priority: weight ∈ [1, 1+PriorityBonusScale],
	// vd PriorityBonusScale=1 → high priority ≈ 2x low priority, không quá “gắt” như nhân thẳng 10.
	PriorityBonusScale float64 = 1.0
)

// PriorityAwareGA – Genetic Algorithm ưu tiên công bằng theo priority
type PriorityAwareGA struct {
	ChromosomesCount     int
	IterationCount       int
	CrossoverProbability float64
	MutationProbability  float64

	StopNoUpdateIteration int
	CurNoUpdateIteration  int

	MaxReachableRtt       float64
	AvgDepNum             float64
	ExpAppCompuTimeOneCpu float64

	// record the best solution in all iterations
	BestFitnessRecords []float64
	BestSolnRecords    []asmodel.Solution

	// record the best fitness in each iteration
	BestFitnessEachIter []float64
}

// Constructor
func NewPriorityAwareGA(
	chromosomesCount int,
	iterationCount int,
	crossoverProbability float64,
	mutationProbability float64,
	stopNoUpdateIteration int,
	exTimeOneCpu float64,
) *PriorityAwareGA {
	return &PriorityAwareGA{
		ChromosomesCount:      chromosomesCount,
		IterationCount:        iterationCount,
		CrossoverProbability:  crossoverProbability,
		MutationProbability:   mutationProbability,
		StopNoUpdateIteration: stopNoUpdateIteration,
		CurNoUpdateIteration:  0,
		MaxReachableRtt:       0,
		ExpAppCompuTimeOneCpu: exTimeOneCpu,
		BestFitnessRecords:    nil,
		BestSolnRecords:       nil,
		BestFitnessEachIter:   nil,
	}
}

// SetMaxReaRtt – giống MCSSGA
func (p *PriorityAwareGA) SetMaxReaRtt(clouds map[string]asmodel.Cloud) {
	var maxReaRtt float64 = 0
	for _, srcCloud := range clouds {
		for _, ns := range srcCloud.NetState {
			if ns.Rtt > maxReaRtt && ns.Rtt < maxAccRttMs {
				maxReaRtt = ns.Rtt
			}
		}
	}
	p.MaxReachableRtt = maxReaRtt
}

// SetAvgDepNum – giống MCSSGA
func (p *PriorityAwareGA) SetAvgDepNum(apps map[string]asmodel.Application) {
	var sumDepNum int = 0
	for _, app := range apps {
		sumDepNum += len(app.Dependencies)
	}
	p.AvgDepNum = float64(sumDepNum) / float64(len(apps))
}

// Schedule – luồng chính giống MCSSGA, chỉ đổi tên & Fitness
func (p *PriorityAwareGA) Schedule(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	appsOrder []string,
) (asmodel.Solution, error) {
	beego.Info("Using scheduling algorithm: PriorityAwareGA")
	p.SetMaxReaRtt(clouds)
	beego.Info("MaxReachableRtt:", p.MaxReachableRtt)
	p.SetAvgDepNum(apps)
	beego.Info("AvgDepNum:", p.AvgDepNum)

	beego.Info("Clouds:")
	for _, cloud := range clouds {
		beego.Info(models.JsonString(cloud))
	}
	beego.Info("Applications:")
	for _, app := range apps {
		beego.Info(models.JsonString(app))
	}
	beego.Info("Applications:", models.JsonString(apps))
	beego.Info("Clouds:", models.JsonString(clouds))
	beego.Info("appsOrder:", models.JsonString(appsOrder))

	// 1. init population
	initPopulation := p.initialize(clouds, apps, appsOrder)

	// 2. iteration 0 – selection
	currentPopulation := p.selectionOperator(clouds, apps, initPopulation)

	// 3. GA loop
	for iteration := 1; iteration <= p.IterationCount; iteration++ {
		currentPopulation = p.crossoverOperator(clouds, apps, appsOrder, currentPopulation)
		currentPopulation = p.mutationOperator(clouds, apps, appsOrder, currentPopulation)
		currentPopulation = p.selectionOperator(clouds, apps, currentPopulation)

		if p.CurNoUpdateIteration > p.StopNoUpdateIteration {
			break
		}
	}

	beego.Info("Best fitness in each iteration:", p.BestFitnessEachIter)
	beego.Info("Final BestFitnessRecords:", p.BestFitnessRecords)
	beego.Info("Total iteration number (the following 2 should be equal): ",
		len(p.BestFitnessRecords), len(p.BestSolnRecords))

	if len(p.BestSolnRecords) == 0 {
		return asmodel.GenEmptySoln(), fmt.Errorf("PriorityAwareGA: no solution generated")
	}
	return p.BestSolnRecords[len(p.BestSolnRecords)-1], nil
}

// initialize – giống MCSSGA (dùng RandomAcceptMostSolution)
func (p *PriorityAwareGA) initialize(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	appsOrder []string,
) []asmodel.Solution {
	var initPopulation []asmodel.Solution
	for i := 0; i < p.ChromosomesCount; i++ {
		sol := RandomAcceptMostSolution(clouds, apps, appsOrder)
		initPopulation = append(initPopulation, sol)
	}
	return initPopulation
}

// crossoverOperator – giống MCSSGA, dùng AllPossTwoPointCrossover
func (p *PriorityAwareGA) crossoverOperator(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	appsOrder []string,
	population []asmodel.Solution,
) []asmodel.Solution {
	if len(apps) <= 1 {
		return population
	}

	var idxNeedCrossover []int
	for i := 0; i < len(population); i++ {
		if random.RandomFloat64(0, 1) < p.CrossoverProbability {
			idxNeedCrossover = append(idxNeedCrossover, i)
		}
	}

	var crossoveredPopulation []asmodel.Solution
	whetherCrossover := make([]bool, len(population))

	var crpoMu sync.Mutex
	var wg sync.WaitGroup

	for len(idxNeedCrossover) > 1 {
		// pick first
		first := random.RandomInt(0, len(idxNeedCrossover)-1)
		firstIndex := idxNeedCrossover[first]
		whetherCrossover[firstIndex] = true
		idxNeedCrossover = append(idxNeedCrossover[:first], idxNeedCrossover[first+1:]...)

		// pick second
		second := random.RandomInt(0, len(idxNeedCrossover)-1)
		secondIndex := idxNeedCrossover[second]
		whetherCrossover[secondIndex] = true
		idxNeedCrossover = append(idxNeedCrossover[:second], idxNeedCrossover[second+1:]...)

		firstChromosome := asmodel.SolutionCopy(population[firstIndex])
		secondChromosome := asmodel.SolutionCopy(population[secondIndex])

		wg.Add(1)
		go func() {
			defer wg.Done()
			newFirst, newSecond :=
				AllPossTwoPointCrossover(firstChromosome, secondChromosome, clouds, apps, appsOrder)
			crpoMu.Lock()
			crossoveredPopulation = append(crossoveredPopulation, newFirst, newSecond)
			crpoMu.Unlock()
		}()
	}
	wg.Wait()

	for i := 0; i < len(population); i++ {
		if !whetherCrossover[i] {
			crossoveredPopulation = append(crossoveredPopulation, asmodel.SolutionCopy(population[i]))
		}
	}

	return crossoveredPopulation
}

// mutationOperator – giống MCSSGA, nhưng receiver là PriorityAwareGA
func (p *PriorityAwareGA) mutationOperator(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	appsOrder []string,
	population []asmodel.Solution,
) []asmodel.Solution {
	mutatedPopulation := make([]asmodel.Solution, len(population))

	var popuMu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < len(population); i++ {
		wg.Add(1)
		go func(chromIdx int) {
			defer wg.Done()

			for {
				mutatedChromosome := asmodel.GenEmptySoln()

				for appName, oriGene := range population[chromIdx].AppsSolution {
					if random.RandomFloat64(0, 1) < p.MutationProbability {
						mutatedChromosome.AppsSolution[appName] = p.geneMutate(clouds, oriGene)
					} else {
						mutatedChromosome.AppsSolution[appName] = asmodel.SasCopy(oriGene)
					}
				}

				mutatedChromosome, acceptable :=
					RefineSoln(clouds, apps, appsOrder, mutatedChromosome)
				if acceptable {
					popuMu.Lock()
					mutatedPopulation[chromIdx] = mutatedChromosome
					popuMu.Unlock()
					break
				}
			}
		}(i)
	}
	wg.Wait()

	return mutatedPopulation
}

// geneMutate – giống MCSSGA
func (p *PriorityAwareGA) geneMutate(
	clouds map[string]asmodel.Cloud,
	ori asmodel.SingleAppSolution,
) asmodel.SingleAppSolution {
	mutated := asmodel.SasCopy(asmodel.RejSoln)

	cloudsToPick := asmodel.CloudMapCopy(clouds)
	if ori.Accepted {
		delete(cloudsToPick, ori.TargetCloudName)
	}

	mutated.Accepted = random.RandomInt(0, 1) == 0
	if mutated.Accepted {
		mutated.TargetCloudName, _ = randomCloudMapPick(cloudsToPick)
	}

	return mutated
}

// selectionOperator – giống MCSSGA nhưng dùng PriorityAwareGA.Fitness
func (p *PriorityAwareGA) selectionOperator(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	population []asmodel.Solution,
) []asmodel.Solution {

	fitnesses := make([]float64, len(population))

	var fitMu sync.Mutex
	var wg sync.WaitGroup
	for i := 0; i < len(population); i++ {
		wg.Add(1)
		go func(chromIdx int) {
			defer wg.Done()
			thisFitness := p.Fitness(clouds, apps, population[chromIdx])
			fitMu.Lock()
			fitnesses[chromIdx] = thisFitness
			fitMu.Unlock()
		}(i)
	}
	wg.Wait()

	beego.Info("PriorityAwareGA – fitness values this iteration:", fitnesses)

	var newPopulation []asmodel.Solution
	pickHelper := make([]int, len(fitnesses))

	bestFitThisIter := -math.MaxFloat64
	bestFitThisIterIdx := 0

	for i := 0; i < p.ChromosomesCount; i++ {
		picked := random.RandomPickN(pickHelper, 2)
		var selChrIdx int
		if fitnesses[picked[0]] > fitnesses[picked[1]] {
			selChrIdx = picked[0]
		} else {
			selChrIdx = picked[1]
		}

		newChromosome := asmodel.SolutionCopy(population[selChrIdx])
		newPopulation = append(newPopulation, newChromosome)

		selFitness := fitnesses[selChrIdx]
		if selFitness > bestFitThisIter {
			bestFitThisIter = selFitness
			bestFitThisIterIdx = selChrIdx
		}
	}

	// update best records
	if len(p.BestFitnessRecords) != len(p.BestSolnRecords) {
		panic(fmt.Sprintf("PriorityAwareGA: len(BestFitnessRecords)=%d, len(BestSolnRecords)=%d",
			len(p.BestFitnessRecords), len(p.BestSolnRecords)))
	}

	var bestFitAllIter float64
	var bestSolnAllIter asmodel.Solution

	if len(p.BestFitnessRecords) == 0 {
		bestFitAllIter = bestFitThisIter
		bestSolnAllIter = population[bestFitThisIterIdx]
		p.CurNoUpdateIteration = 0
	} else {
		bestFitAllIter = p.BestFitnessRecords[len(p.BestFitnessRecords)-1]
		bestSolnAllIter = p.BestSolnRecords[len(p.BestSolnRecords)-1]
		if bestFitThisIter > bestFitAllIter {
			bestFitAllIter = bestFitThisIter
			bestSolnAllIter = population[bestFitThisIterIdx]
			p.CurNoUpdateIteration = 0
		} else {
			p.CurNoUpdateIteration++
		}
	}

	p.BestFitnessEachIter = append(p.BestFitnessEachIter, bestFitThisIter)
	p.BestFitnessRecords = append(p.BestFitnessRecords, bestFitAllIter)
	p.BestSolnRecords = append(p.BestSolnRecords, asmodel.SolutionCopy(bestSolnAllIter))

	return newPopulation
}
// Fitness mới cho PriorityAwareGA:
// - Vẫn tối ưu base service fitness (computation + network)
// - Cực kỳ ưu tiên fairness theo priority:
//   + Thưởng coverage cao (accepted/total) cho từng priority (priority cao thưởng nhiều hơn)
//   + Phạt nếu coverage(priority thấp) > coverage(priority cao)
//   + Phạt rất nặng nếu priority nào có total>0 mà accepted=0
func (p *PriorityAwareGA) Fitness(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	chromosome asmodel.Solution,
) float64 {

	if len(apps) == 0 {
		return -math.MaxFloat64
	}

	// Tìm min/max priority thực tế trong bộ apps
	minPri := math.MaxInt
	maxPri := 0
	for _, app := range apps {
		if app.Priority < minPri {
			minPri = app.Priority
		}
		if app.Priority > maxPri {
			maxPri = app.Priority
		}
	}
	if minPri > maxPri { // không có app hợp lệ
		return -math.MaxFloat64
	}
	if minPri == maxPri {
		// tất cả cùng priority, vẫn xử lý được nhưng coverage theo priority sẽ không quan trọng lắm
	}

	type priStats struct {
		total    int
		accepted int
	}
	priStatMap := make(map[int]*priStats)

	totalApps := 0
	totalAccepted := 0

	baseServiceFitness := 0.0

	// 1. Tính base service fitness + thống kê priority
	for appName, app := range apps {
		totalApps++
		pr := app.Priority
		if _, ok := priStatMap[pr]; !ok {
			priStatMap[pr] = &priStats{}
		}
		priStatMap[pr].total++

		gene := chromosome.AppsSolution[appName]
		if gene.Accepted {
			priStatMap[pr].accepted++
			totalAccepted++
		}

		// non-priority fitness (computation + network)
		nonPriFit := p.fitnessOneAppNonPri(clouds, apps, chromosome, appName)

		// normalized priority weight ∈ [1, 1+PriorityBonusScale]
		// priority càng cao (số càng nhỏ) → weight càng lớn
		var priWeight float64
		if maxPri == minPri {
			priWeight = 1.0
		} else {
			// t ∈ [0..1], với priority cao (pr gần minPri) → t gần 1
			t := float64(maxPri-pr) / float64(maxPri-minPri)
			priWeight = 1.0 + PriorityBonusScale*t
		}

		baseServiceFitness += nonPriFit * priWeight
	}

	if totalApps == 0 {
		return -math.MaxFloat64
	}

	// 2. Coverage theo priority + đếm số priority bị "bỏ đói"
	coverage := make(map[int]float64)
	zeroPriorityCount := 0

	for pr, st := range priStatMap {
		if st.total > 0 {
			cov := float64(st.accepted) / float64(st.total)
			coverage[pr] = cov
			if st.accepted == 0 {
				zeroPriorityCount++
			}
		}
	}

	// 3. CoverageScore – thưởng coverage, ưu tiên priority cao
	coverageScore := 0.0
	for pr := minPri; pr <= maxPri; pr++ {
		cov, ok := coverage[pr]
		if !ok {
			continue
		}
		var wCov float64
		if maxPri == minPri {
			wCov = 1.0
		} else {
			// priority cao (gần minPri) → weightCoverage lớn hơn
			t := float64(maxPri-pr) / float64(maxPri-minPri) // 0..1
			wCov = 1.0 + 0.5*t                               // 1..1.5
		}
		coverageScore += cov * wCov
	}

	// 4. Penalty: nếu coverage(priority thấp) > coverage(priority cao)
	// giả sử priority thấp = số lớn hơn
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

	// 5. Overall acceptance
	overallAcceptance := float64(totalAccepted) / float64(totalApps)

	// 6. Kết hợp tất cả:
	//    - coreFitnessWeight: giữ nguyên tầm quan trọng của base service fitness
	//    - scale coverageScore & overallAcceptance lên đủ lớn
	//    - phạt cực nặng nếu có priority nào accepted = 0
	// Các hệ số này bạn có thể tune thêm nếu muốn "ưu tiên fairness" mạnh hơn nữa.
	const (
		coreFitnessWeight          = 1.0   // giống trước
		coverageScoreWeight        = 200.0 // thưởng coverage
		overallAcceptanceWeight    = 100.0 // thưởng acceptance chung
		violationPenaltyWeight     = 400.0 // phạt khi priority thấp phủ nhiều hơn priority cao
		zeroPriorityPenaltyWeight  = 800.0 // phạt cực mạnh nếu có priority bị bỏ đói
	)

	fitness := 0.0
	fitness += coreFitnessWeight * baseServiceFitness
	fitness += coverageScoreWeight * coverageScore
	fitness += overallAcceptanceWeight * overallAcceptance
	fitness -= violationPenaltyWeight * priorityViolationPenalty
	fitness -= zeroPriorityPenaltyWeight * float64(zeroPriorityCount)

	return fitness
}


// fitnessOneAppNonPri – giống MCSSGA phiên bản hiện tại
// (KHÔNG nhân với priority)
func (p *PriorityAwareGA) fitnessOneAppNonPri(
	clouds map[string]asmodel.Cloud,
	apps map[string]asmodel.Application,
	chromosome asmodel.Solution,
	thisAppName string,
) float64 {

	var thisAppFitnessNonPri float64

	if !chromosome.AppsSolution[thisAppName].Accepted {
		// nếu reject: phạt một giá trị âm vừa phải
		thisAppFitnessNonPri = -(p.ExpAppCompuTimeOneCpu + p.MaxReachableRtt*p.AvgDepNum) / 2
	} else {
		// tính computation part
		thisAlloCpu := chromosome.AppsSolution[thisAppName].AllocatedCpuCore
		if thisAlloCpu <= 0 {
			// trường hợp bất thường, tránh chia cho 0
			thisAlloCpu = 1
		}
		thisAppPart := p.ExpAppCompuTimeOneCpu - p.ExpAppCompuTimeOneCpu/thisAlloCpu

		// network part
		netPart := p.MaxReachableRtt * p.AvgDepNum
		for _, dep := range apps[thisAppName].Dependencies {
			depAppName := dep.AppName

			thisCloudName := chromosome.AppsSolution[thisAppName].TargetCloudName
			depCloudName := chromosome.AppsSolution[depAppName].TargetCloudName
			thisNodeName := chromosome.AppsSolution[thisAppName].K8sNodeName
			depNodeName := chromosome.AppsSolution[depAppName].K8sNodeName

			var thisRtt float64
			if thisNodeName == depNodeName {
				thisRtt = 0
			} else {
				thisRtt = clouds[thisCloudName].NetState[depCloudName].Rtt
			}

			netPart -= thisRtt
		}
		if netPart < 0 {
			netPart = 0
		}

		thisAppFitnessNonPri = thisAppPart + netPart
	}

	return thisAppFitnessNonPri
}

// DrawEvoChart – giống MCSSGA nhưng cho PriorityAwareGA
func (p *PriorityAwareGA) DrawEvoChart() {
	var drawChartFunc func(http.ResponseWriter, *http.Request) = func(res http.ResponseWriter, r *http.Request) {
		var xValuesAllBest []float64
		for i := range p.BestFitnessRecords {
			xValuesAllBest = append(xValuesAllBest, float64(i))
		}

		graph := chart.Chart{
			Title: "PriorityAwareGA Evolution",
			XAxis: chart.XAxis{
				Name:      "Iteration Number",
				NameStyle: chart.StyleShow(),
				Style:     chart.StyleShow(),
				ValueFormatter: func(v interface{}) string {
					return strconv.FormatInt(int64(v.(float64)), 10)
				},
			},
			YAxis: chart.YAxis{
				AxisType:  chart.YAxisSecondary,
				Name:      "Fitness",
				NameStyle: chart.StyleShow(),
				Style:     chart.StyleShow(),
			},
			Background: chart.Style{
				Padding: chart.Box{
					Top:  50,
					Left: 20,
				},
			},
			Series: []chart.Series{
				chart.ContinuousSeries{
					Name:    "Best Fitness in all iterations",
					XValues: xValuesAllBest,
					YValues: p.BestFitnessRecords,
				},
				chart.ContinuousSeries{
					Name:    "Best Fitness in each iteration",
					XValues: xValuesAllBest,
					YValues: p.BestFitnessEachIter,
					Style: chart.Style{
						Show:            true,
						StrokeDashArray: []float64{5.0, 3.0, 2.0, 3.0},
						StrokeWidth:     1,
					},
				},
			},
		}

		graph.Elements = []chart.Renderable{
			chart.LegendThin(&graph),
		}

		res.Header().Set("Content-Type", "image/png")
		err := graph.Render(chart.PNG, res)
		if err != nil {
			log.Println("PriorityAwareGA – graph.Render(chart.PNG, res) error:", err)
		}
	}

	http.HandleFunc("/", drawChartFunc)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("PriorityAwareGA – http.ListenAndServe(\":8080\", nil) error:", err)
	}
}
