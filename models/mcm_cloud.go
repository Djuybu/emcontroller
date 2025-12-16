package models

import (
	"fmt"
	"sync"

	"github.com/astaxie/beego"

	"emcontroller/weather" // ⭐ NEW: dùng để lấy nhiệt độ
)

// ===== Types cho UI =====

type Limits struct {
	VCpu    float64
	Ram     float64 // MB
	Storage float64 // GB
	Vm      float64
	Volume  float64
	Port    float64
}

type Usage = Limits

type CloudResources struct {
	Limit Limits
	InUse Usage
}

type CloudInfo struct {
	Name   string
	Type   string
	WebUrl string

	Resources CloudResources

	// Các field % để template sử dụng (progress bars / badges)
	CpuPercent     float64
	RamPercent     float64
	StoragePercent float64
	VmPercent      float64
	VolumePercent  float64
	PortPercent    float64

	Latitude  string
	Longitude string

	TemperatureC   float64
	HasTemperature bool
}

// ===== Helpers =====

func safeDiv(a, b float64) float64 {
	if b <= 0 {
		return 0
	}
	return (a / b) * 100.0
}

func toCloudInfo(cloud Iaas, rs ResourceStatus) CloudInfo {
	ci := CloudInfo{
		Name:   cloud.ShowName(),
		Type:   cloud.ShowType(),
		WebUrl: cloud.ShowWebUrl(),
		Resources: CloudResources{
			Limit: Limits{
				VCpu:    rs.Limit.VCpu,
				Ram:     rs.Limit.Ram,
				Storage: rs.Limit.Storage,
				Vm:      rs.Limit.Vm,
				Volume:  rs.Limit.Volume,
				Port:    rs.Limit.Port,
			},
			InUse: Usage{
				VCpu:    rs.InUse.VCpu,
				Ram:     rs.InUse.Ram,
				Storage: rs.InUse.Storage,
				Vm:      rs.InUse.Vm,
				Volume:  rs.InUse.Volume,
				Port:    rs.InUse.Port,
			},
		},
		// % phục vụ UI
		CpuPercent:     safeDiv(rs.InUse.VCpu, rs.Limit.VCpu),
		RamPercent:     safeDiv(rs.InUse.Ram, rs.Limit.Ram),
		StoragePercent: safeDiv(rs.InUse.Storage, rs.Limit.Storage),
		VmPercent:      safeDiv(rs.InUse.Vm, rs.Limit.Vm),
		VolumePercent:  safeDiv(rs.InUse.Volume, rs.Limit.Volume),
		PortPercent:    safeDiv(rs.InUse.Port, rs.Limit.Port),
	}

	// ⭐ Lấy toạ độ từ cloud nếu có hỗ trợ
	// Đoạn này dùng type assertion cho mềm dẻo, không phá struct Iaas hiện tại.
	type locator interface {
		GetLatitude() string
		GetLongitude() string
	}
	if loc, ok := cloud.(locator); ok {
		ci.Latitude = loc.GetLatitude()
		ci.Longitude = loc.GetLongitude()
	}

	return ci
}

// ===== Public APIs =====

// ListClouds đọc từ biến toàn cục Clouds (map[string]Iaas) và trả danh sách CloudInfo
func ListClouds() ([]CloudInfo, []error) {
	var cloudList []CloudInfo
	var errs []error

	// slice không an toàn khi truy cập đồng thời → dùng mutex
	var cloudListMu sync.Mutex
	var errsMu sync.Mutex

	var wg sync.WaitGroup
	for _, cloud := range Clouds {
		wg.Add(1)
		go func(cloud Iaas) {
			defer wg.Done()

			// Lấy tài nguyên
			rs, err := cloud.CheckResources()
			if err != nil {
				outErr := fmt.Errorf("check resources failed for cloud Name [%s] Type [%s]: %w",
					cloud.ShowName(), cloud.ShowType(), err)
				beego.Error(outErr)
				errsMu.Lock()
				errs = append(errs, outErr)
				errsMu.Unlock()
			}

			ci := toCloudInfo(cloud, rs)

			// ⭐ BƠM NHIỆT ĐỘ VÀO CloudInfo (cho UI list)
			if ci.Type == LocalIaas && ci.Latitude != "" && ci.Longitude != "" {
				temp, terr := weather.GetCurrentTemperature(ci.Latitude, ci.Longitude)
				if terr != nil {
					beego.Warn(fmt.Sprintf("GetCurrentTemperature error for cloud [%s]: %s", ci.Name, terr.Error()))
				} else {
					ci.TemperatureC = temp
					ci.HasTemperature = true
				}
			}

			beego.Info(fmt.Sprintf("Cloud [%s], type [%s], resources [%+v]", ci.Name, ci.Type, ci.Resources))

			cloudListMu.Lock()
			cloudList = append(cloudList, ci)
			cloudListMu.Unlock()

			beego.Info(fmt.Sprintf("Cloud [%s], type [%s] added to cloudList", ci.Name, ci.Type))
		}(cloud)
	}
	wg.Wait()

	return cloudList, errs
}

// GetCloud trả thông tin 1 cloud + danh sách VM của nó
func GetCloud(cloudName string) (CloudInfo, []IaasVm, error, error) {
	cloud, ok := Clouds[cloudName]
	if !ok || cloud == nil {
		err := fmt.Errorf("cloud %q not found", cloudName)
		return CloudInfo{}, nil, err, nil
	}

	// Resources
	rs, errRes := cloud.CheckResources()
	if errRes != nil {
		beego.Error(fmt.Sprintf("Check resources for cloud Name [%s] Type [%s], error: %s",
			cloud.ShowName(), cloud.ShowType(), errRes.Error()))
	}
	cloudInfo := toCloudInfo(cloud, rs)

	// ⭐ Thêm nhiệt độ cho trang chi tiết 1 cloud
	if cloudInfo.Type == LocalIaas && cloudInfo.Latitude != "" && cloudInfo.Longitude != "" {
		temp, terr := weather.GetCurrentTemperature(cloudInfo.Latitude, cloudInfo.Longitude)
		if terr != nil {
			beego.Warn(fmt.Sprintf("GetCurrentTemperature error for cloud [%s]: %s", cloudInfo.Name, terr.Error()))
		} else {
			cloudInfo.TemperatureC = temp
			cloudInfo.HasTemperature = true
		}
	}

	// VMs
	vmList, errVMs := cloud.ListAllVMs()
	if errVMs != nil {
		beego.Error(fmt.Sprintf("List VMs in cloud Name [%s] Type [%s], error: %s",
			cloud.ShowName(), cloud.ShowType(), errVMs.Error()))
	}

	return cloudInfo, vmList, errRes, errVMs
}
