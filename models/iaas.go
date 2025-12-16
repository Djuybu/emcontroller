package models

import (
	"fmt"
	"strings"
	"sync"

	"github.com/astaxie/beego"
	"github.com/gophercloud/gophercloud"
	"github.com/spf13/viper"
)

// ===== Cloud Type Constants =====
const LocalIaas = "local"

// ===== Interfaces & Structs =====
type Iaas interface {
	ShowName() string
	ShowType() string
	ShowWebUrl() string

	GetVM(vmID string) (*IaasVm, error)
	ListAllVMs() ([]IaasVm, error)

	// Tạo VM theo cơ chế mặc định của cloud driver (ví dụ virt-install với --location/--cdrom, hoặc API cloud)
	CreateVM(name string, vcpu, ram, storage int) (*IaasVm, error)

	DeleteVM(vmID string) error
	CheckResources() (ResourceStatus, error)
	IsCreatedByMcm(vmID string) (bool, error)
}

// importer là interface mở rộng (không bắt buộc) dành cho driver hỗ trợ import từ image có sẵn
// (vd: Local driver sử dụng virt-install --import/--boot hd)
type importer interface {
	CreateVMFromImage(name, imagePath string, vcpu, ram, storage int) (*IaasVm, error)
}

type ResourceStatus struct {
	Limit ResSet `json:"limit"`
	InUse ResSet `json:"inUse"`
}

func (rs ResourceStatus) LeastRemainPct() float64 {
	leastPct := 1.0

	// Helper an toàn chia 0
	safePct := func(limit, inuse float64) float64 {
		if limit <= 0 {
			// Nếu không có hạn mức (0 hoặc âm), coi như không hạn chế
			return 1.0
		}
		p := (limit - inuse) / limit
		if p < 0 {
			return 0
		}
		if p > 1 {
			return 1
		}
		return p
	}

	pctVCpu := safePct(rs.Limit.VCpu, rs.InUse.VCpu)
	if pctVCpu < leastPct {
		leastPct = pctVCpu
	}
	pctRam := safePct(rs.Limit.Ram, rs.InUse.Ram)
	if pctRam < leastPct {
		leastPct = pctRam
	}
	pctStorage := safePct(rs.Limit.Storage, rs.InUse.Storage)
	if pctStorage < leastPct {
		leastPct = pctStorage
	}
	return leastPct
}

func (rs ResourceStatus) Overflow() bool {
	return rs.InUse.VCpu > rs.Limit.VCpu ||
		rs.InUse.Ram > rs.Limit.Ram ||
		rs.InUse.Storage > rs.Limit.Storage
}

type IaasVm struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	IPs           []string `json:"ips"`
	VCpu          float64  `json:"vcpu"`
	Ram           float64  `json:"ram"`
	Storage       float64  `json:"storage"`
	Status        string   `json:"status"`
	Cloud         string   `json:"cloud"`
	CloudType     string   `json:"cloudType"`
	McmCreate     bool     `json:"mcmCreate"`
	OsVariant     string   `json:"osVariant,omitempty"`
	InstallMethod string   `json:"installMethod,omitempty"`
}

type ResSet struct {
	VCpu    float64 `json:"vcpu"`
	Ram     float64 `json:"ram"`
	Vm      float64 `json:"vm"`
	Volume  float64 `json:"volume"`
	Storage float64 `json:"storage"`
	Port    float64 `json:"port"`
}

func (r1 ResSet) AllMoreThan(r2 ResSet) bool {
	// Trả về true nếu mọi tài nguyên của r1 đều > r2 (bỏ qua r2<0)
	if r2.VCpu >= 0 && !(r1.VCpu > r2.VCpu) {
		return false
	}
	if r2.Ram >= 0 && !(r1.Ram > r2.Ram) {
		return false
	}
	if r2.Vm >= 0 && !(r1.Vm > r2.Vm) {
		return false
	}
	if r2.Volume >= 0 && !(r1.Volume > r2.Volume) {
		return false
	}
	if r2.Storage >= 0 && !(r1.Storage > r2.Storage) {
		return false
	}
	if r2.Port >= 0 && !(r1.Port > r2.Port) {
		return false
	}
	return true
}

// ===== Global Variables =====
var Clouds map[string]Iaas = make(map[string]Iaas)
var iaasConfig *viper.Viper

// ===== Configuration =====
func readIaasConfig() {
	iaasConfig = viper.New()
	iaasConfig.SetConfigFile("conf/iaas.json")
	iaasConfig.SetConfigType("json")
	if err := iaasConfig.ReadInConfig(); err != nil {
		panic(fmt.Errorf("fatal error parse iaas.json: %w", err))
	}
}

// ===== Init Clouds =====
func InitClouds() {
	readIaasConfig()
	var iaasParas []map[string]interface{}
	if err := iaasConfig.UnmarshalKey("iaas", &iaasParas); err != nil {
		panic(fmt.Errorf("UnmarshalKey \"iaas\" of iaas.json error: %w", err))
	}

	for i := 0; i < len(iaasParas); i++ {
		switch iaasParas[i]["type"].(string) {
		case OpenstackIaas:
			osCloud := InitOpenstack(iaasParas[i])
			Clouds[osCloud.Name] = osCloud
		case ProxmoxIaas:
			pCloud := InitProxmox(iaasParas[i])
			Clouds[pCloud.Name] = pCloud
		case LocalIaas: // ✅ local VM
			lc := InitLocal(iaasParas[i])
			Clouds[lc.Name] = lc
		default:
			beego.Info(fmt.Sprintf("Multi-cloud manager does not support cloud type [%s] of cloud [%s]",
				iaasParas[i]["type"].(string),
				iaasParas[i]["name"].(string)))
		}
	}

	beego.Info(fmt.Sprintf("All %d clouds are initialized.", len(Clouds)))
}

// ===== SSH Waiters =====

func WaitForSshPem(user string, pemFilePath string, sshIP string, sshPort int, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		sshClient, err := SshClientWithPem(pemFilePath, user, sshIP, sshPort)
		if err != nil {
			beego.Info(fmt.Sprintf("Waiting for SSH ip %s, SshClientWithPem error: %s", sshIP, err.Error()))
			return false, nil
		}
		defer sshClient.Close()
		output, err := SshOneCommand(sshClient, DiskInitCmd)
		if err != nil {
			beego.Info(fmt.Sprintf("Waiting for SSH ip %s, SshOneCommand error: %s", sshIP, err.Error()))
			return false, nil
		}
		beego.Info(fmt.Sprintf("SSH %s enabled, output: %s", sshIP, output))
		return true, nil
	})
}

func WaitForSshPasswdAndInit(user string, passwd string, sshIP string, sshPort int, secs int) error {
	return gophercloud.WaitFor(secs, func() (bool, error) {
		sshClient, err := SshClientWithPasswd(user, passwd, sshIP, sshPort)
		if err != nil {
			beego.Info(fmt.Sprintf("Waiting for SSH ip %s, SshClientWithPasswd error: %s", sshIP, err.Error()))
			return false, nil
		}
		defer sshClient.Close()
		output, err := SshOneCommand(sshClient, DiskInitCmd)
		if err != nil {
			beego.Info(fmt.Sprintf("Waiting for SSH ip %s, SshOneCommand error: %s", sshIP, err.Error()))
			return false, nil
		}
		beego.Info(fmt.Sprintf("SSH %s enabled, output: %s", sshIP, output))
		return true, nil
	})
}

// ===== Helpers =====

// getImagePathForCloud: ưu tiên lấy từ vm.OsVariant (nếu là đường dẫn file),
// sau đó tới iaas.json: imagePathMap.<cloudName>
func getImagePathForCloud(v IaasVm) string {
	// Nếu OsVariant có vẻ là đường dẫn image thì dùng luôn
	if v.OsVariant != "" &&
		(strings.HasPrefix(v.OsVariant, "/") ||
			strings.HasSuffix(v.OsVariant, ".qcow2") ||
			strings.HasSuffix(v.OsVariant, ".img")) {
		return v.OsVariant
	}
	// Nếu không, thử lấy từ cấu hình
	if iaasConfig != nil {
		key := fmt.Sprintf("imagePathMap.%s", v.Cloud)
		if iaasConfig.IsSet(key) {
			return iaasConfig.GetString(key)
		}
	}
	return ""
}

// needInstallMethodFallback: phát hiện lỗi virt-install yêu cầu chỉ định install method
func needInstallMethodFallback(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "An install method must be specified") ||
		strings.Contains(msg, "--location") ||
		strings.Contains(msg, "--cdrom") ||
		strings.Contains(msg, "--pxe") ||
		strings.Contains(msg, "--import")
}

// ===== VM Management =====

func CreateVms(vms []IaasVm) ([]IaasVm, error) {
	vmGroups := GroupVmsByCloud(vms)

	var errs []error
	var createdVms []IaasVm
	var errsMu sync.Mutex
	var createdVmsMu sync.Mutex
	var wg sync.WaitGroup

	for _, vmGroup := range vmGroups {
		wg.Add(1)
		go func(vg []IaasVm) {
			defer wg.Done()
			for _, v := range vg {
				beego.Info(fmt.Sprintf("Start create VM [%s] Cloud [%s]", v.Name, v.Cloud))
				cloud, exist := Clouds[v.Cloud]
				if !exist {
					outErr := fmt.Errorf("Create vm %s error: cloud [%s] not found.", v.Name, v.Cloud)
					beego.Error(outErr)
					errsMu.Lock()
					errs = append(errs, outErr)
					errsMu.Unlock()
					return
				}

				// --- NEW: honor InstallMethod when explicitly set ---
				switch strings.ToLower(strings.TrimSpace(v.InstallMethod)) {
				case "import":
					imagePath := getImagePathForCloud(v)
					if imagePath == "" {
						beego.Error(fmt.Sprintf(
							"InstallMethod=import but missing imagePath for VM [%s] on cloud [%s]. "+
								"Set vm.OsVariant to absolute image path or configure iaas.json:imagePathMap.%s",
							v.Name, v.Cloud, v.Cloud))
						// fallthrough to default behavior (try CreateVM then fallback)
					} else {
						if imp, ok := cloud.(importer); ok {
							beego.Info(fmt.Sprintf("Create VM [%s] via explicit import with image [%s]", v.Name, imagePath))
							if createdVM2, err2 := imp.CreateVMFromImage(v.Name, imagePath, int(v.VCpu), int(v.Ram), int(v.Storage)); err2 == nil {
								beego.Info(fmt.Sprintf("Created vm by explicit import:\n%+v\n", createdVM2))
								createdVmsMu.Lock()
								createdVms = append(createdVms, *createdVM2)
								createdVmsMu.Unlock()
								continue // done
							} else {
								beego.Error(fmt.Sprintf("Explicit import for %s failed: %s", v.Name, err2))
								errsMu.Lock()
								errs = append(errs, err2)
								errsMu.Unlock()
								continue // don't try default path to avoid double-creating
							}
						} else {
							beego.Error(fmt.Sprintf("Cloud [%s] does not support CreateVMFromImage (import).", v.Cloud))
							// fallthrough to default behavior
						}
					}
				case "":
					// no preference → use default path below
				case "location", "cdrom", "pxe", "boot":
					// Not wired into Iaas interface yet; proceed with default CreateVM and rely on driver or fallback.
					beego.Info(fmt.Sprintf("InstallMethod=%s requested for [%s], proceeding with driver default and fallback.",
						v.InstallMethod, v.Name))
				default:
					beego.Info(fmt.Sprintf("Unknown installMethod=%q for [%s], using driver default.", v.InstallMethod, v.Name))
				}
				// --- end NEW ---

				// 1) Thử tạo theo đường chuẩn của cloud driver
				createdVM, err := cloud.CreateVM(v.Name, int(v.VCpu), int(v.Ram), int(v.Storage))
				if err == nil {
					beego.Info(fmt.Sprintf("Created vm:\n%+v\n", createdVM))
					createdVmsMu.Lock()
					createdVms = append(createdVms, *createdVM)
					createdVmsMu.Unlock()
					continue
				}

				// 2) Nếu lỗi do thiếu install method (virt-install), thử fallback import từ image có sẵn
				if needInstallMethodFallback(err) {
					imagePath := getImagePathForCloud(v)
					if imagePath == "" {
						beego.Error(fmt.Sprintf(
							"Missing imagePath for fallback import of VM [%s] on cloud [%s]. "+
								"Set vm.OsVariant to absolute image path or configure iaas.json:imagePathMap.%s",
							v.Name, v.Cloud, v.Cloud))
					} else {
						if imp, ok := cloud.(importer); ok {
							beego.Info(fmt.Sprintf("Retry create VM [%s] on cloud [%s] with existing image [%s] ...",
								v.Name, v.Cloud, imagePath))
							createdVM2, err2 := imp.CreateVMFromImage(v.Name, imagePath, int(v.VCpu), int(v.Ram), int(v.Storage))
							if err2 == nil {
								beego.Info(fmt.Sprintf("Created vm by import:\n%+v\n", createdVM2))
								createdVmsMu.Lock()
								createdVms = append(createdVms, *createdVM2)
								createdVmsMu.Unlock()
								continue
							}
							beego.Error(fmt.Sprintf("Create vm %s fallback by image [%s] error: %s", v.Name, imagePath, err2))
							errsMu.Lock()
							errs = append(errs, err2)
							errsMu.Unlock()
							// tiếp tục xuống ghi nhận lỗi gốc
						} else {
							beego.Error(fmt.Sprintf("Cloud [%s] does not support CreateVMFromImage fallback.", v.Cloud))
						}
					}
				}

				// 3) Ghi nhận lỗi nguyên bản nếu không fallback được hoặc fallback fail
				beego.Error(fmt.Sprintf("Create vm %s error: %s", v.Name, err))
				errsMu.Lock()
				errs = append(errs, err)
				errsMu.Unlock()
			}
		}(vmGroup)
	}
	wg.Wait()

	if len(errs) > 0 {
		outErr := fmt.Errorf("CreateVms failed: %v", errs)
		beego.Error(outErr)
		return createdVms, outErr
	}
	return createdVms, nil
}

func FindVm(vmName string, vms []IaasVm) (*IaasVm, bool) {
	for _, vm := range vms {
		if vmName == vm.Name {
			return &vm, true
		}
	}
	return nil, false
}

func GroupVmsByCloud(vms []IaasVm) map[string][]IaasVm {
	out := make(map[string][]IaasVm)
	for _, vm := range vms {
		out[vm.Cloud] = append(out[vm.Cloud], vm)
	}
	return out
}

func DeleteBatchVms(vms []IaasVm) []error {
	var errs []error
	var errsMu sync.Mutex
	var wg sync.WaitGroup
	for _, vm := range vms {
		wg.Add(1)
		go func(v IaasVm) {
			defer wg.Done()
			if err := Clouds[v.Cloud].DeleteVM(v.ID); err != nil {
				outErr := fmt.Errorf("Delete vm [%s:%s] on [%s] failed: %v", v.Name, v.ID, v.Cloud, err)
				beego.Error(outErr)
				errsMu.Lock()
				errs = append(errs, outErr)
				errsMu.Unlock()
			}
		}(vm)
	}
	wg.Wait()
	return errs
}
