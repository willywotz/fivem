package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/svc"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	_ = becomeAdmin()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_ = autoUpdate(ctx)
	_ = installService(svcName, svcDisplayName)
	_ = startService(svcName)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()

	if srcPath, _ := os.Executable(); strings.Contains(srcPath, "go-build") {
		_ = removeService(svcName)
	}
}
