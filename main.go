package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/svc"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	_ = becomeAdmin()
	_ = handleAutoUpdate()

	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	_ = installService(svcName, svcDisplayName)
	// _ = startService(svcName)

	fmt.Printf("fivem started. version: %s\n", buildVersion)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()

	exePath, _ := os.Executable()
	if strings.Contains(exePath, "go-build") {
		_ = removeService(svcName)
	}
}
