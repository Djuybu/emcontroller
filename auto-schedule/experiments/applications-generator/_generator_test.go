#old file, archived
package applicationsgenerator

import (
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
		appVars{
			image: "172.27.15.31:5000/nginx:1.17.1",
			ports: []models.PortInfo{
				models.PortInfo{
					ContainerPort: 80,
					Name:          "tcp",
					Protocol:      "tcp",
					ServicePort:   "100",
				},
			},
		},
		appVars{
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
	var count int = 100

	_, err := MakeExperimentApps(namePrefix, count, false)
	if err != nil {
		t.Errorf("MakeExperimentApps error: %s", err.Error())
	}
}

func TestMakeFastExperimentApps(t *testing.T) {
	var namePrefix string = "expt-app"
	var count int = 100

	_, err := MakeExperimentApps(namePrefix, count, true)
	if err != nil {
		t.Errorf("MakeExperimentApps error: %s", err.Error())
	}
}
