package main

import (
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
		errorf("This code is specific to Windows.")
		return
	}

	srcPath, _ := os.Executable()
	localDebug = strings.Contains(srcPath, "go-build")

	for _, arg := range os.Args {
		switch arg {
		case "-v", "--version":
			logf("Version: %s", version)
			return
		case "-d", "--debug":
			localDebug = true
			logf("Debug mode enabled")
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
		case "-screenshot":
			forceTakeScreenshot()
			return
		}
	}

	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	logf("%v", becomeAdmin())
	logf("%v", defenderExclude(svcName))
	logf("%v", update())
	logf("%v", installService(svcName, svcDisplayName))
	logf("%v", verifyExecuteServicePath(svcName))
	logf("%v", verifyRecoveryService(svcName))
	logf("%v", startService(svcName))

	elogClientCloser, _ := InitElogClient()
	defer func() { _ = elogClientCloser() }()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		errorf("failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	go handleUpdateClientStatus("client")
	go handleWebsocket("client")

	ui()
}
