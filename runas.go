package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/svc"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

func becomeAdmin() error {
	if exe, _ := os.Executable(); strings.Contains(exe, "go-build") {
		return nil
	}

	if inService, _ := svc.IsWindowsService(); inService {
		return nil
	}

	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err == nil {
		return nil // Already running as admin
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	verb := "runas"

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	_, _, err = procShellExecuteW.Call(
		uintptr(0),                       // hwnd
		uintptr(unsafe.Pointer(verbPtr)), // lpOperation
		uintptr(unsafe.Pointer(exePtr)),  // lpFile
		uintptr(unsafe.Pointer(argPtr)),  // lpParameters
		uintptr(unsafe.Pointer(cwdPtr)),  // lpDirectory
		uintptr(1),                       // nShowCmd SW_NORMAL
	)
	if err != nil && err != syscall.Errno(0) /* ERROR_SUCCESS */ {
		fmt.Fprintf(os.Stderr, "Failed to elevate privileges: %v\n", err)
	}

	os.Exit(0) // Exit the current process after starting the new one with admin privileges

	return nil
}
