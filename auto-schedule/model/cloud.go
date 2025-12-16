package model

import (
	"fmt"
	"math"
	"strings"

	"github.com/astaxie/beego"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"emcontroller/models"
	"emcontroller/weather"
)

type Cloud struct {
	Name         string                         `json:"name"`
	Type         string                         `json:"type"`
	Resources    models.ResourceStatus          `json:"resources"` // used and all resources of this cloud. Here we start with struct defined in "models" package, and in the future if we find that this cannot meet the needs here, we can define new structs.
	NetState     map[string]models.NetworkState `json:"netState"`  // the network state from this cloud to every cloud
	K8sNodes     []K8sNode                      `json:"k8sNodes"`  // all existing Kubernetes nodes whose VMs are on this cloud
	TemperatureC float64                        `json:"temp_c"`
}

// the set of all cloud types that support creating new VMs when auto-scheduling
var typesCanCreateNewVM map[string]struct{} = map[string]struct{}{
	models.ProxmoxIaas: struct{}{},
}

// Not all cloud types support creating new VMs.
// For example, CLAAUDIA does not allow users to create flavors in Openstack, so if we want to create new VMs in auto-scheduling, this will be more complicated, so we do not support creating new VMs in auto-scheduling.
func (c Cloud) SupportCreateNewVM() bool {
	if _, exist := typesCanCreateNewVM[c.Type]; exist {
		return true
	}
	return false
}

// According to the input resource percentage, this function can generate the information of the shared VM to create.
func (c Cloud) GetSharedVmToCreate(resPct float64, allRest bool) models.IaasVm {
	var totalResources GenericResources
	if !allRest {
		totalResources = c.GetResVmToCreate(resPct)
	} else { // when allRest is true, resPct will not be used
		totalResources = c.GetAllRestRes()
	}
	return models.IaasVm{
		Name:    c.GetNameVmToCreate(),
		Cloud:   c.Name,
		VCpu:    totalResources.CpuCore,
		Ram:     totalResources.Memory,
		Storage: totalResources.Storage,
	}
}

// auto-schedule vms should have special prefixes
func (c Cloud) GetNameVmToCreate() string {
	var vmName string
OUTLOOP:
	for i := 0; i < math.MaxInt; i++ {
		vmName = fmt.Sprintf("%s%s-%d", ASVmNamePrefix, strings.ToLower(c.Name), i)
		for _, existingVm := range c.K8sNodes {
			if existingVm.Name == vmName {
				continue OUTLOOP // the intended vmName is already used, we continue to try the next possible name.
			}
		}
		return vmName // the intended vmName is not used, so we can use it.
	}
	panic(fmt.Sprintf("Cloud [%s], all available auto-schedule vm names are used up. There are [%d] existing auto-schedule vms on this cloud.", c.Name, len(c.K8sNodes)))
}

// According to the input resource percentage, this function can generate the total resources of the shared VM to create.
func (c Cloud) GetResVmToCreate(resPct float64) GenericResources {
	return GenericResources{
		CpuCore: math.Floor(resPct * c.Resources.Limit.VCpu),
		Memory:  math.Floor(resPct * c.Resources.Limit.Ram),
		Storage: math.Floor(resPct * c.Resources.Limit.Storage),
	}
}

// get all rest resources of this cloud
func (c Cloud) GetAllRestRes() GenericResources {
	restCpu := math.Floor(c.Resources.Limit.VCpu - c.Resources.InUse.VCpu)
	if restCpu < 0 {
		restCpu = 0
	}

	restRam := math.Floor(c.Resources.Limit.Ram - c.Resources.InUse.Ram)
	if restRam < 0 {
		restRam = 0
	}

	restStorage := math.Floor(c.Resources.Limit.Storage - c.Resources.InUse.Storage)
	if restStorage < 0 {
		restStorage = 0
	}

	return GenericResources{
		CpuCore: restCpu,
		Memory:  restRam,
		Storage: restStorage,
	}
}

func CloudCopy(src Cloud) Cloud {
	var dst Cloud = src
	dst.K8sNodes = make([]K8sNode, len(src.K8sNodes))
	copy(dst.K8sNodes, src.K8sNodes)
	return dst
}

func CloudMapCopy(src map[string]Cloud) map[string]Cloud {
	var dst map[string]Cloud = make(map[string]Cloud)
	for name, cloud := range src {
		dst[name] = CloudCopy(cloud)
	}
	return dst
}

// generate a group of Cloud from a group of models.Iaas
func GenerateClouds(inputClouds map[string]models.Iaas) (map[string]Cloud, error) {
	var outputClouds map[string]Cloud = make(map[string]Cloud)

	netStates, err := models.GetNetState()
	if err != nil {
		outErr := fmt.Errorf("Check network state from MySQL Error: %w", err)
		beego.Error(outErr)
		return nil, outErr
	}

	allK8sNodes, err := models.ListNodes(metav1.ListOptions{})
	if err != nil {
		outErr := fmt.Errorf("List Kubernetes Nodes Error: %w", err)
		beego.Error(outErr)
		return nil, outErr
	}

	for name, inCloud := range inputClouds {
		outputClouds[name], err = GenerateOneCloud(inCloud, netStates[name], allK8sNodes)
		if err != nil {
			outErr := fmt.Errorf("generate the Cloud  [%s] from models.Iaas, Error: %w", name, err)
			beego.Error(outErr)
			return nil, outErr
		}
	}

	return outputClouds, nil
}

// getCloudLocation returns latitude and longitude for a cloud name
// Default locations for common cloud names, can be extended or moved to config
func getCloudLocation(cloudName string) (string, string) {
	// Default location mapping - can be extended or moved to config file
	locationMap := map[string]struct {
		lat string
		lon string
	}{
		"myvm":           {"21.0285", "105.8542"}, // Hà Nội
		"CLAAUDIAweifan": {"55.6762", "12.5683"},  // Copenhagen, Denmark
		"HPE1":           {"21.0285", "105.8542"}, // Default to Hà Nội
		"HPE2":           {"21.0285", "105.8542"},
		"NOKIA1":         {"21.0285", "105.8542"},
		"NOKIA2":         {"21.0285", "105.8542"},
		"NOKIA3":         {"21.0285", "105.8542"},
		"NOKIA4":         {"21.0285", "105.8542"},
		"NOKIA5":         {"21.0285", "105.8542"},
		"NOKIA6":         {"21.0285", "105.8542"},
		"NOKIA7":         {"21.0285", "105.8542"},
		"NOKIA8":         {"21.0285", "105.8542"},
		"NOKIA9":         {"21.0285", "105.8542"},
		"NOKIA10":        {"21.0285", "105.8542"},
	}

	if loc, ok := locationMap[cloudName]; ok {
		return loc.lat, loc.lon
	}
	// Default to Hà Nội if cloud name not found
	return "21.0285", "105.8542"
}

// generate the Cloud from models.Iaas, the network states of this cloud, and all Kubernetes Nodes
func GenerateOneCloud(inCloud models.Iaas, cloudNetStates map[string]models.NetworkState, allK8sNodes []apiv1.Node) (Cloud, error) {

	resources, err := inCloud.CheckResources()
	if err != nil {
		outErr := fmt.Errorf("Check resources for cloud Name [%s] Type [%s], error: %w", inCloud.ShowType(), inCloud.ShowType(), err)
		beego.Error(outErr)
		return Cloud{}, outErr
	}

	k8sNodesOnCloud, err := getK8sNodesOnCloud(inCloud, allK8sNodes)
	if err != nil {
		outErr := fmt.Errorf("Get Kubernetes Nodes on Cloud [%s] Type [%s], error: %w", inCloud.ShowType(), inCloud.ShowType(), err)
		beego.Error(outErr)
		return Cloud{}, outErr
	}

	// Fetch temperature for this cloud
	var temperature float64 = 20.0 // default temperature
	lat, lon := getCloudLocation(inCloud.ShowName())
	temp, err := weather.GetCurrentTemperature(lat, lon)
	if err != nil {
		beego.Warn(fmt.Sprintf("Failed to fetch temperature for cloud [%s] at [%s,%s], using default 20°C: %v", inCloud.ShowName(), lat, lon, err))
	} else {
		temperature = temp
		beego.Info(fmt.Sprintf("Cloud [%s] temperature: %.1f°C (location: %s,%s)", inCloud.ShowName(), temperature, lat, lon))
	}

	var outCloud Cloud = Cloud{
		Name:         inCloud.ShowName(),
		Type:         inCloud.ShowType(),
		NetState:     cloudNetStates,
		Resources:    resources,
		K8sNodes:     k8sNodesOnCloud,
		TemperatureC: temperature,
	}

	return outCloud, nil
}

func getK8sNodesOnCloud(cloud models.Iaas, allK8sNodes []apiv1.Node) ([]K8sNode, error) {
	var k8sNodes []K8sNode

	// We should find the VMs that meet the 2 conditions:
	// 1. They are on this cloud;
	// 2. They are Kubernetes Nodes;

	vms, err := cloud.ListAllVMs()
	if err != nil {
		outErr := fmt.Errorf("List vms in Cloud [%s] Type [%s], error: %w", cloud.ShowType(), cloud.ShowType(), err)
		beego.Error(outErr)
		return []K8sNode{}, outErr
	}

	for _, vm := range vms {
		// If we cannot get the IP of a VM, we do not consider scheduling applications to it.
		if len(vm.IPs) == 0 {
			continue
		}
		// We do not schedule applications to the VMs that are not created by multi-cloud manager
		if createdByMcm, err := cloud.IsCreatedByMcm(vm.ID); err != nil {
			printErr := fmt.Errorf("Cloud [%s] Type [%s] check IsCreatedByMcm [%s/%s], error: %w. So we jump it.", cloud.ShowType(), cloud.ShowType(), vm.Name, vm.ID, err)
			beego.Error(printErr)
			continue
		} else if !createdByMcm {
			continue
		}

		// NOTE: Maybe I should add a Kubernetes annotation to every VM created by auto-scheduling and only get the VMs with that annotation here, to avoid using other VMs for auto-scheduling, but maybe I should not do it, because if users hope their VMs not to be used by others, they should add Kubernetes taints to them, or otherwise when other uses create Kubernetes applications, Kubernetes may also schedule them to their VMs automatically.
		// NOTE 2: Although I need to make a mechanism to delete unused VMs periodically, which should only delete the VMs created by auto-scheduling, I still do not need to add the annotation, because I can use the name prefix ASVmNamePrefix to filter the auto-scheduling VMs.

		for _, node := range allK8sNodes {
			if len(node.Spec.Taints) > 0 {
				// If len(node.Spec.Taints) > 0, do not auto schedule applications to this node. This can ensure the auto-schedule will not put applications on network state test VMs, and also provide a way to users to avoid auto-schedule from putting applications on their VMs.
				continue
			}

			// If the IPs and names match, we find a Kubernetes node on this cloud
			// We use a VM for auto-scheduling only if both its name and IP are the same as those of its Kubernetes node.
			if vm.IPs[0] == models.GetNodeInternalIp(node) && vm.Name == node.Name {
				// get all pods on this VM.
				podsOnNode, err := models.ListPodsOnNode(models.KubernetesNamespace, node.Name)
				if err != nil {
					outErr := fmt.Errorf("List pods on Kubernetes node [%s], error: %w", node.Name, err)
					beego.Error(outErr)
					return []K8sNode{}, outErr
				}

				thisNode := GenK8sNodeFromPods(vm, podsOnNode)
				k8sNodes = append(k8sNodes, thisNode)

				break // When we find a match, we can break to search for the next VM.
			}
		}
	}

	return k8sNodes, nil
}
