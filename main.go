package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows/svc"
)

var version string = "v0"

var BaseURL string = "http://localhost:8080"

func main() {
	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	fmt.Println(becomeAdmin())
	fmt.Println(update())
	fmt.Println(installService(svcName, svcDisplayName))
	fmt.Println(verifyExecuteServicePath(svcName))
	fmt.Println(startService(svcName))

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
