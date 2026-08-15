package main

import (
	"log/slog"
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
		slog.Error("this code is specific to windows")
		return
	}

	srcPath, _ := os.Executable()
	localDebug = strings.Contains(srcPath, "go-build")

	for _, arg := range os.Args {
		switch arg {
		case "-v", "--version":
			slog.Info("version", "version", version)
			return
		case "-d", "--debug":
			localDebug = true
			slog.Info("debug mode enabled")
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

	logStep("become admin", becomeAdmin())
	logStep("defender exclude", defenderExclude(svcName))
	logStep("update", update())
	logStep("install service", installService(svcName, svcDisplayName))
	logStep("verify execute service path", verifyExecuteServicePath(svcName))
	logStep("verify recovery service", verifyRecoveryService(svcName))
	logStep("start service", startService(svcName))

	elogClientCloser, _ := InitElogClient()
	defer func() { _ = elogClientCloser() }()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		slog.Error("failed to initialize OLE", "err", err)
		return
	}
	defer ole.CoUninitialize()

	go handleUpdateClientStatus("client")
	go handleWebsocket("client")

	ui()
}
