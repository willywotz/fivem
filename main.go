package main

import (
	"fmt"
	"time"

	"github.com/go-ole/go-ole"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	_ = becomeAdmin()

	go func() {
		ticker := time.NewTicker(1 * time.Hour)

		for {
			<-ticker.C

			fmt.Println("Checking for updates...")
			_ = handleAutoUpdate()
		}
	}()

	// if inService, _ := svc.IsWindowsService(); inService {
	// 	runService(svcName, false)
	// 	return
	// }

	// _ = removeService(svcName)

	// go func() { _ = installService(svcName, "FiveM Service") }()

	fmt.Printf("fivem started. version: %s\n", buildVersion)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()
}
