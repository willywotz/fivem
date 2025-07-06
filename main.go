package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-ole/go-ole"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	_ = becomeAdmin()

	updateCtx := context.Background()
	updateCtx, onceUpdateDone := context.WithCancel(updateCtx)
	go autoUpdate(onceUpdateDone)
	<-updateCtx.Done()

	// if inService, _ := svc.IsWindowsService(); inService {
	// 	runService(svcName, false)
	// 	return
	// }

	// _ = removeService(svcName)

	// go func() { _ = installService(svcName, "FiveM Service") }()

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()
}

func autoUpdate(onceUpdateDone context.CancelFunc) {
	ticker := time.NewTicker(1 * time.Hour)

	for {
		fmt.Println("Checking for updates...")
		_ = handleAutoUpdate()
		onceUpdateDone()
		<-ticker.C
	}
}
