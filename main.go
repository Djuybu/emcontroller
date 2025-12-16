package main

import (
	"flag"
	"fmt"
	"runtime"
	"strconv"
	"time"

	"github.com/astaxie/beego"

	"emcontroller/auto-schedule/algorithms"
	"emcontroller/models"
	_ "emcontroller/routers"
)

var gitCommit, buildDate string

func printVersion() {
	fmt.Printf("Build time: [%s]. Git commit: [%s]\n", models.BuildDate, models.GitCommit)
}

func main() {
	models.BuildDate = buildDate
	models.GitCommit = gitCommit

	versionFlag := flag.Bool("v", false, "Print the current version and exit.")
	flag.Parse()
	if *versionFlag {
		printVersion()
		return
	}

	// ===============================
	// BỎ QUA INIT CLOUD / OPENSTACK
	// ===============================
	models.InitSomeThing()

	numCpuToUse := runtime.NumCPU()
	beego.Info(fmt.Sprintf("Using %d CPU cores for goroutines.", numCpuToUse))
	runtime.GOMAXPROCS(numCpuToUse)

	if netTestOn, err := beego.AppConfig.Bool("TurnOnNetTest"); err == nil && netTestOn {
		beego.Info("Network performance test function is on.")
		if err := models.InitNetPerfDB(); err != nil {
			outErr := fmt.Errorf("Initialize the database [%s] in MySQL failed, error: [%w]", models.NetPerfDbName, err)
			beego.Error(outErr)
			panic(outErr)
		}
		models.MeasNetPerf()
		netTestPeriodSec, err := strconv.Atoi(beego.AppConfig.String("NetTestPeriodSec"))
		if err != nil {
			beego.Error(fmt.Sprintf("Read config \"NetTestPeriodSec\" error: %s, set the period as the DefaultNetTestPeriodSec", err.Error()))
			netTestPeriodSec = models.DefaultNetTestPeriodSec
		}

		beego.Info(fmt.Sprintf("The period of measuring network performance is %d seconds, which is also the periods of auto-scheduling VMs cleanup.", netTestPeriodSec))
		go models.CronTaskTimer(models.MeasNetPerf, time.Duration(netTestPeriodSec)*time.Second)
		go models.CronTaskTimer(algorithms.GcASVms, time.Duration(netTestPeriodSec)*time.Second)

		models.NetTestFuncOn = true
		models.NetTestPeriodSec = netTestPeriodSec
	} else if err != nil {
		beego.Error(fmt.Sprintf("Read \"TurnOnNetTest\" in app.conf, error: [%s]. We turn off the network performance test function.", err.Error()))
	} else {
		beego.Info("Network performance test function is off.")
	}

	// ===============================
	// CHỈ CHẠY BEGOO UI
	// ===============================
	beego.Run()
}
