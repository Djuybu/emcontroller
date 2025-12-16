package algorithms

import (
	"sync"

	asmodel "emcontroller/auto-schedule/model"
)

const (
	// previously cpuCoreStep was set as 0.1, consistent with the stride in Kubernetes, but we need to use static CPU Manager policy, which requires all CPU requests and limits are integers.
	cpuCoreStep float64 = 1 // also named as stride. This can be seen as the unit that we allocate CPU cores in our algorithm.

	// On a cloud, when existing VMs do not have enough resources for the applications scheduled here, we have 3 possible choices to create a new VM:
	// 1. a VM with 50% resources; if 50% is not enough, do 3.
	// 2. a VM with 30% resources; if 30% is not enough, do 3.
	// 3. a VM with all rest resources; if all rest is not enough, it means this solution is not acceptable.
	biggerVmResPct  float64 = 0.5
	smallerVmResPct float64 = 0.3

	floatDelta float64 = 0.0001 // binary-floating-point data is not accurate, so we need to allow a delta when checking whether 2 float values are equal

	maxAccRttMs float64 = 20000 // unit: millisecond (ms). Maximum acceptable Round-Trip Time (RTT) between to applications with a dependency. The value should be smaller than models.UnreachableRttMs.

	McssgaName   string = "Mcssga"
	CompRandName string = "CompRand"
	BERandName   string = "BERand"
	AmagaName    string = "Amaga"
	AmpgaName    string = "Ampga"
	DiktyogaName string = "Diktyoga"
	MTDPName     string = "MTDP" // Minimizing Total Data Center Power - based on ISLPED 2009 paper
)

var (
	// scheduling, migration, and cleanup cannot be done at the same time, so we set a mutex for them.
	ScheMu sync.Mutex
)

// SchedulingAlgorithm is the interface that all algorithms should implement
type SchedulingAlgorithm interface {
	Schedule(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string) (asmodel.Solution, error)
}

// After scheduling applications to clouds, we get a coarse solution. Then, we use this function to refine the solution, do 3 things:
// 1. schedule applications to VMs inside clouds;
// 2. allocate CPUs to applications inside VMs;
// 3. Check whether this solution is acceptable.
// If this solution passes the above 3 things, we return the refined solution.
func RefineSoln(clouds map[string]asmodel.Cloud, apps map[string]asmodel.Application, appsOrder []string, soln asmodel.Solution) (asmodel.Solution, bool) {
	// 1. give the solution node names
	solnWithVm, vmAcceptable := allocateVms(clouds, apps, appsOrder, soln)
	if !vmAcceptable {
		return asmodel.Solution{}, false
	}
	// 2. Allocate CPU cores
	solnWithCpu, cpuAcceptable := allocateCpus(clouds, apps, appsOrder, solnWithVm)
	if !cpuAcceptable {
		return asmodel.Solution{}, false
	}
	// 3. Check whether this solution is acceptable.
	if !Acceptable(clouds, apps, appsOrder, solnWithCpu) {
		return asmodel.Solution{}, false
	}
	return solnWithCpu, true
}
