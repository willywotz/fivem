package main

import (
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/go-ole/go-ole"
	_ "github.com/josephspurrier/goversioninfo"
	"golang.org/x/sys/windows/svc"
)

var version string = "v0"

var BaseURL string = "http://localhost:8080"

var localDebug bool = false

func main() {
	if runtime.GOOS != "windows" {
		log.Fatalln("This code is specific to Windows.")
		return
	}

	srcPath, _ := os.Executable()
	localDebug = strings.Contains(srcPath, "go-build")

	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	log.Println(becomeAdmin())
	log.Println(defenderExclude(svcName))
	log.Println(update())
	log.Println(installService(svcName, svcDisplayName))
	log.Println(verifyExecuteServicePath(svcName))
	log.Println(verifyRecoveryService(svcName))
	log.Println(startService(svcName))

	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		log.Fatalln("Failed to initialize OLE: ", err)
		return
	}
	defer ole.CoUninitialize()

	// go handleUpdateClientStatus("client")

	ui()
}
