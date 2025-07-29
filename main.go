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

var (
	noBecomeAdmin              = false
	noDefenderExclude          = false
	noUpdate                   = false
	noInstallService           = false
	noVerifyExecuteServicePath = false
	noVerifyRecoveryService    = false
	noStartService             = false
	noElogClient               = false
)

func main() {
	if runtime.GOOS != "windows" {
		failedf("This code is specific to Windows.")
		return
	}

	srcPath, _ := os.Executable()
	localDebug = strings.Contains(srcPath, "go-build")

	for _, arg := range os.Args {
		switch arg {
		case "-v", "--version":
			log.Println("Version:", version)
			return
		case "-d", "--debug":
			localDebug = true
			log.Println("Debug mode enabled")
		case "-no-become-admin":
			noBecomeAdmin = true
		case "-no-defender-exclude":
			noDefenderExclude = true
		case "-no-update":
			noUpdate = true
		case "-no-install-service":
			noInstallService = true
		case "-no-verify-execute-service-path":
			noVerifyExecuteServicePath = true
		case "-no-verify-recovery-service":
			noVerifyRecoveryService = true
		case "-no-start-service":
			noStartService = true
		case "-no-elog-client":
			noElogClient = true
		}
	}

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

	elogClientCloser, _ := InitElogClient()
	defer func() { _ = elogClientCloser() }()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		failedf("failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	go handleUpdateClientStatus("client")

	ui()
}
