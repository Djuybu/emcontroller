package applicationsgenerator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"emcontroller/models"
)

func TestInnerGetOccupiedNodePorts(t *testing.T) {
	nodePorts, err := getOccupiedNodePorts("localhost:20000")
	if err != nil {
		t.Errorf("get nodePorts error: %s", err.Error())
	} else {
		t.Logf("Apps: %+v", nodePorts)
	}
}

func TestInnerGetAllApps(t *testing.T) {
	apps, err := getAllApps("localhost:20000")
	if err != nil {
		t.Errorf("get apps error: %s", err.Error())
	} else {
		t.Logf("Apps: %+v", apps)
	}
}

func TestMakeAppsForTest(t *testing.T) {
	var namePrefix string = "test-app"
	var count int = 40

	var possibleVars []appVars = []appVars{
		{
			image: "172.27.15.31:5000/nginx:1.17.1",
			ports: []models.PortInfo{
				{
					ContainerPort: 80,
					Name:          "tcp",
					Protocol:      "tcp",
					ServicePort:   "100",
				},
			},
		},
		{
			image: "172.27.15.31:5000/ubuntu:latest",
			commands: []string{
				"bash",
				"-c",
				"while true;do sleep 10;done",
			},
		},
	}

	_ = MakeAppsForTest(namePrefix, count, possibleVars)
}

func TestMakeExperimentApps(t *testing.T) {
	var namePrefix string = "expt-app"
	var count int = 60

	apps, err := MakeExperimentApps(namePrefix, count, false)
	if err != nil {
		t.Fatalf("MakeExperimentApps error: %s", err.Error())
	}

	// Ghi ra file JSON
	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent error: %s", err.Error())
	}

	// Đặt tên file: group_n100.json (hoặc sửa thành n60 tuỳ bạn)
	filename := "group_n100.json"

	// Đảm bảo ghi trong thư mục package hiện tại
	outPath := filepath.Join(".", filename)

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %s", err.Error())
	}

	t.Logf("Generated experiment apps JSON written to %s", outPath)
}

func TestMakeFastExperimentApps(t *testing.T) {
	var namePrefix string = "expt-app"
	var count int = 20

	apps, err := MakeExperimentApps(namePrefix, count, true)
	if err != nil {
		t.Fatalf("MakeExperimentApps (fast) error: %s", err.Error())
	}

	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent error: %s", err.Error())
	}

	filename := "group_n20_fast.json"
	outPath := filepath.Join(".", filename)

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		t.Fatalf("WriteFile error: %s", err.Error())
	}

	t.Logf("Generated FAST experiment apps JSON written to %s", outPath)
}
