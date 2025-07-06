package main

import (
	"context"
	"fmt"

	"github.com/go-ole/go-ole"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	// if inService, _ := svc.IsWindowsService(); inService {
	// 	runService(svcName, false)
	// 	return
	// }

	_ = becomeAdmin()

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_ = autoUpdate(ctx)

	// _ = installService(svcName, svcDisplayName)
	// _ = startService(svcName)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()

	// if exe, _ := exePath(); strings.Contains(exe, "go-build") {
	// 	removeService(svcName)
	// }
}
