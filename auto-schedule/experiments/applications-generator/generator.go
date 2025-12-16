package applicationsgenerator

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/KeepTheBeats/routing-algorithms/random"

	asmodel "emcontroller/auto-schedule/model"
	"emcontroller/models"
)

type depGenHelper struct {
	appName  string
	priority int
	oriIdx   int // original index
}

type depGenHelperSlice []depGenHelper

func (dghs depGenHelperSlice) Len() int {
	return len(dghs)
}

func (dghs depGenHelperSlice) Swap(i, j int) {
	dghs[i], dghs[j] = dghs[j], dghs[i]
}

func (dghs depGenHelperSlice) Less(i, j int) bool {
	return dghs[i].priority < dghs[j].priority
}

func newDepGenHelper(app models.K8sApp, originalIndex int) depGenHelper {
	return depGenHelper{
		appName:  app.Name,
		priority: app.Priority,
		oriIdx:   originalIndex,
	}
}

// the variables of applications.
type appVars struct {
	image    string
	commands []string
	ports    []models.PortInfo
}

// make applications for some simple tests
func MakeAppsForTest(namePrefix string, count int, possibleVars []appVars) []models.K8sApp {
	outApps := make([]models.K8sApp, count)

	for i := 0; i < len(outApps); i++ {
		randomVar := possibleVars[random.RandomInt(0, len(possibleVars)-1)]

		outApps[i].Name = fmt.Sprintf("%s-%d", namePrefix, i)
		outApps[i].AutoScheduled = true
		outApps[i].Replicas = 1
		outApps[i].Priority = random.RandomInt(asmodel.MinPriority, asmodel.MaxPriority)
		//outApps[i].HostNetwork = random.RandomInt(0, 1) == 0
		outApps[i].HostNetwork = false
		outApps[i].Containers = []models.K8sContainer{
			models.K8sContainer{
				Name:     "container",
				Image:    randomVar.image,
				Commands: randomVar.commands,
				Ports:    randomVar.ports,
				WorkDir:  "",
				Resources: models.K8sResReq{
					Limits: models.K8sResList{
						CPU:    fmt.Sprintf("%.1f", random.NormalRandomBM(1.0, 32.0, 6.0, 6.0)),
						Memory: fmt.Sprintf("%.0fMi", random.NormalRandomBM(200.0, 16384.0, 2048.0, 8192.0)),
					},
				},
			},
		}

		resStorage := random.RandomInt(0, 200)
		if resStorage > 0 {
			outApps[i].Containers[0].Resources.Limits.Storage = fmt.Sprintf("%dGi", resStorage)
		}
		outApps[i].Containers[0].Resources.Requests = outApps[i].Containers[0].Resources.Limits

	}

	// generate dependencies
	var depHelpers []depGenHelper = make([]depGenHelper, count)
	for i := 0; i < len(outApps); i++ {
		depHelpers[i] = newDepGenHelper(outApps[i], i)
	}
	sort.Sort(depGenHelperSlice(depHelpers)) // sort apps, and an app can only depend on those after it.

	for i := 0; i < len(depHelpers); i++ {
		for j := i + 1; j < len(depHelpers); j++ {
			if random.RandomInt(0, 3) == 0 {
				outApps[depHelpers[i].oriIdx].Dependencies = append(outApps[depHelpers[i].oriIdx].Dependencies, models.Dependency{AppName: depHelpers[j].appName})
			}
		}
	}

	fmt.Println("Generated Applications Json:")
	fmt.Println(models.JsonString(outApps))

	return outApps
}

// the variables of applications.
type appRes struct {
	name    string
	cpu     int // unit core
	memory  int // unit MiB
	storage int // unit GiB
}

// from real applications or applications in the experiments of existing papers
var appsToChoose []appRes = []appRes{
	appRes{name: "existingPaperApp1", cpu: 2, memory: 1024, storage: 8},
	appRes{name: "existingPaperApp2", cpu: 2, memory: 1024, storage: 4},
	appRes{name: "existingPaperApp3", cpu: 4, memory: 2048, storage: 3},
	appRes{name: "existingPaperApp4", cpu: 2, memory: 1024, storage: 2},
	appRes{name: "existingPaperMySQL", cpu: 1, memory: 500, storage: 0},
	appRes{name: "actualNginxController", cpu: 8, memory: 8192, storage: 155},
	appRes{name: "actualRedis", cpu: 4, memory: 15360, storage: 30},
	appRes{name: "actualPostgres", cpu: 2, memory: 2048, storage: 1},
	appRes{name: "actualRabbitmq", cpu: 1, memory: 256, storage: 6},
	appRes{name: "actualConsul", cpu: 4, memory: 16384, storage: 100},
	appRes{name: "actualRedmine", cpu: 4, memory: 4096, storage: 20},
	appRes{name: "actualMiRFleet", cpu: 2, memory: 8192, storage: 128},
	appRes{name: "actualApacheStorm", cpu: 12, memory: 24576, storage: 0},
	appRes{name: "actualApacheKafka", cpu: 4, memory: 8192, storage: 500},
	appRes{name: "actualApacheZookeeper", cpu: 2, memory: 2048, storage: 80},
}

const (
	minNodePort int = 30000
	maxNodePort int = 32768

	// mcmEndpoint string = "172.27.15.31:20000"
	mcmEndpoint string = "localhost:20000"

	defaultWorkload int    = 5000000 // input value of cumulative sum
	exptImage       string = "172.27.15.31:5000/mcexp:20230905"
	baseCmd         string = "./experiment-app"
	svcPort         int    = 81
)

// call multi-cloud manager API to get all occupied nodePorts. The input parameter is the endpoint (IP:port) of multi-cloud manager
func getOccupiedNodePorts(mcmEp string) (map[string]string, error) {
	apps, err := getAllApps(mcmEp)
	if err != nil {
		return nil, fmt.Errorf("get all applications, error: %w", err)
	}

	var nodePorts map[string]string = make(map[string]string)
	for _, app := range apps {
		for _, nodePort := range app.NodePort {
			nodePorts[nodePort] = app.AppName
		}
	}

	return nodePorts, nil
}

func getAllApps(mcmEp string) ([]models.AppInfo, error) {
	url := "http://" + mcmEp + "/application"
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("url: %s, make request error: %w", url, err)
	}
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("url: %s, do request error: %w", url, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("url: %s, res.statusCode is %d, read res.Body error: %w", url, res.StatusCode, err)
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("url: %s, res.statusCode is %d, res.Body is %s", url, res.StatusCode, string(body))
	}

	var apps []models.AppInfo
	if err := json.Unmarshal(body, &apps); err != nil {
		return nil, fmt.Errorf("url: %s, res.statusCode is %d, res.Body is %s, Unmarshal body error: %s", url, res.StatusCode, string(body), err.Error())
	}

	return apps, nil
}

// make applications for experiments
func MakeExperimentApps(namePrefix string, count int, fastMode bool) ([]models.K8sApp, error) {
	depDivisor := float64(count) / 10.0
	outApps := make([]models.K8sApp, count)

	occNodePorts, err := getOccupiedNodePorts(mcmEndpoint)
	if err != nil {
		return nil, fmt.Errorf("getOccupiedNodePorts from multi-cloud manager endpoint %s, error: %s", mcmEndpoint, err.Error())
	}

	nextNodePortToTry := minNodePort

	for i := 0; i < len(outApps); i++ {
		// find a nodePort to use
		var nodePortToUse string
		for {
			nodePortToUse = fmt.Sprintf("%d", nextNodePortToTry)
			nextNodePortToTry++
			if nextNodePortToTry > maxNodePort {
				return nil, fmt.Errorf("all available NodePorts in [%d, %d] are occupied", minNodePort, maxNodePort)
			}

			// If this nodePort is not occupied, we use it.
			if _, exist := occNodePorts[nodePortToUse]; !exist {
				break
			}
		}

		// choose one application randomly
		chosenApp := appsToChoose[random.RandomInt(0, len(appsToChoose)-1)]

		outApps[i].Name = fmt.Sprintf("%s-%d", namePrefix, i)
		outApps[i].AutoScheduled = true
		outApps[i].Replicas = 1
		outApps[i].HostNetwork = false

		// randomly generate a priority between the min and max values
		outApps[i].Priority = random.RandomInt(asmodel.MinPriority, asmodel.MaxPriority)

		// measured from real applications.
		// We do not need to use the same amount of workload, because in the comparison of response time, I only compare the applications accepted by both my algorithm and another algorithm.
		workload := int(random.NormalRandomBM(55000, 1415000, 466267, 370555))
		// workload := 381475 // To avoid the interference, all applications should have a same amount of workload.

		args := []string{fmt.Sprintf("%d", workload), fmt.Sprintf("%d", chosenApp.cpu), fmt.Sprintf("%d", chosenApp.memory), fmt.Sprintf("%d", chosenApp.storage)}
		if fastMode { // fast mode is to test the functions. In this mode, the applications will not use time to occupy memory and storage.
			args = []string{fmt.Sprintf("%d", workload), fmt.Sprintf("%d", chosenApp.cpu), fmt.Sprintf("%d", 0), fmt.Sprintf("%d", 0)}
		}

		outApps[i].Containers = []models.K8sContainer{
			models.K8sContainer{
				Name:     "container",
				Image:    exptImage,
				Commands: []string{baseCmd},
				Args:     args,
				Ports: []models.PortInfo{
					models.PortInfo{
						ContainerPort: 3333,
						Name:          "tcp",
						Protocol:      "tcp",
						ServicePort:   fmt.Sprintf("%d", svcPort),
						NodePort:      nodePortToUse,
					},
				},
				WorkDir: "",
				Resources: models.K8sResReq{
					Limits: models.K8sResList{
						CPU:    fmt.Sprintf("%d", chosenApp.cpu),
						Memory: fmt.Sprintf("%dMi", chosenApp.memory),
					},
				},
			},
		}
		if chosenApp.storage > 0 {
			outApps[i].Containers[0].Resources.Limits.Storage = fmt.Sprintf("%dGi", chosenApp.storage)
		}
		outApps[i].Containers[0].Resources.Requests = outApps[i].Containers[0].Resources.Limits

	}

	// generate dependencies
	var depHelpers []depGenHelper = make([]depGenHelper, count)
	for i := 0; i < len(outApps); i++ {
		depHelpers[i] = newDepGenHelper(outApps[i], i)
	}
	// sort apps, and an app can only depend on those after it, which means the apps with equal or higher priorities than its.
	sort.Sort(depGenHelperSlice(depHelpers))

	for i := 0; i < len(depHelpers); i++ {
		// according to an existing paper, I use 16/196=0.0816 as the possibility that one application depends on another.
		depPoss := float64(16) / float64(196)
		for j := i + 1; j < len(depHelpers); j++ {
			//if depHelpers[j].priority <= depHelpers[i].priority { // if we need to disable apps to depend on those with the equal priority with it, we can uncomment this if.
			//	continue
			//}
			if random.RandomFloat64(0, 1) < depPoss {
				depPoss /= depDivisor // the more dependencies an app has, the lower possibility it can have more deps.
				// this will be read by the scheduling algorithm.
				outApps[depHelpers[i].oriIdx].Dependencies = append(outApps[depHelpers[i].oriIdx].Dependencies, models.Dependency{AppName: depHelpers[j].appName})
				// this is to let the app access its dependent ones
				outApps[depHelpers[i].oriIdx].Containers[0].Args = append(outApps[depHelpers[i].oriIdx].Containers[0].Args, fmt.Sprintf("http://%s%s:%d/experiment", depHelpers[j].appName, models.ServiceSuffix, svcPort))
			}
		}
	}

	fmt.Println("Generated Applications Json:")
	fmt.Println(models.JsonString(outApps))

	return outApps, nil
}
