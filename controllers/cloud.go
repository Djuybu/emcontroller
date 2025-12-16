package controllers

import (
	"fmt"

	"github.com/astaxie/beego"

	"emcontroller/models"
	"emcontroller/weather"
)

type CloudController struct {
	beego.Controller
}

// ================== LIST TẤT CẢ CLOUDS (/cloud) ==================

func (c *CloudController) Get() {
	cloudList, errs := models.ListClouds()
	if len(errs) != 0 {
		sumErr := models.HandleErrSlice(errs)
		beego.Error(fmt.Sprintf("Get Clouds, Error: %s", sumErr.Error()))
		c.Ctx.ResponseWriter.Header().Set("Content-Type", "text/plain")
		c.Data["errorMessage"] = sumErr.Error()
		c.TplName = "error.tpl"
		return
	}

	// 💡 Bơm nhiệt độ cho từng cloud (nếu có lat/lon)
	for i := range cloudList {
		lat := cloudList[i].Latitude
		lon := cloudList[i].Longitude

		if lat == "" || lon == "" {
			continue
		}

		temp, err := weather.GetCurrentTemperature(lat, lon)
		if err != nil {
			beego.Warn("GetCurrentTemperature error for cloud",
				cloudList[i].Name, ":", err)
			continue
		}

		cloudList[i].TemperatureC = temp
		cloudList[i].HasTemperature = true
	}

	// (Nếu bạn có Stats tổng quan thì set ở đây)
	c.Data["TotalClouds"] = len(cloudList)
	// c.Data["AvgCpuUsage"]   = ...
	// c.Data["TotalMemory"]   = ...
	// c.Data["TotalStorage"]  = ...

	c.Data["cloudList"] = cloudList
	c.TplName = "cloud.tpl"
}

// =============== SINGLE CLOUD (/cloud/:cloudName) =================

func (c *CloudController) GetSingleCloud() {
	// Lấy tên cloud từ URL: /cloud/:cloudName
	cloudName := c.Ctx.Input.Param(":cloudName")

	// Lấy thông tin cloud + danh sách VM
	cloudInfo, vmList, _, err := models.GetCloud(cloudName)
	if err != nil {
		beego.Error(fmt.Sprintf("GetSingleCloud, GetCloud Error: %s", err.Error()))
		c.Ctx.ResponseWriter.Header().Set("Content-Type", "text/plain")
		c.Data["errorMessage"] = err.Error()
		c.TplName = "error.tpl"
		return
	}

	// Vì cloudInfo là value (models.CloudInfo), không thể so sánh với nil.
	// Nếu bạn cần detect "không tìm thấy", có thể dựa vào Name rỗng (tuỳ implementation GetCloud).
	if cloudInfo.Name == "" {
		c.Ctx.ResponseWriter.WriteHeader(404)
		c.Data["errorMessage"] = fmt.Sprintf("Cloud %s not found", cloudName)
		c.TplName = "error.tpl"
		return
	}

	// 💡 Bơm nhiệt độ nếu có lat/lon
	if cloudInfo.Latitude != "" && cloudInfo.Longitude != "" {
		if temp, werr := weather.GetCurrentTemperature(
			cloudInfo.Latitude,
			cloudInfo.Longitude,
		); werr != nil {
			beego.Warn("GetCurrentTemperature error for single cloud",
				cloudInfo.Name, ":", werr)
		} else {
			cloudInfo.TemperatureC = temp
			cloudInfo.HasTemperature = true
		}
	}

	c.Data["cloudInfo"] = cloudInfo
	c.Data["vmList"] = vmList
	c.TplName = "singleCloud.tpl"
}
