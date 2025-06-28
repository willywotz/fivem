package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

func checkAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func becomeAdmin() error {
	verb := "runas"

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	var showCmd int32 = 0 // SW_HIDE shows the window normally

	// err = windows.ShellExecute(0, verbPtr, exePtr, argPtr, cwdPtr, showCmd)
	// if err != nil {
	// 	return fmt.Errorf("failed to execute ShellExecute: %w", err)
	// }

	_, _, err = procShellExecuteW.Call(
		uintptr(0),                       // hwnd
		uintptr(unsafe.Pointer(verbPtr)), // lpOperation
		uintptr(unsafe.Pointer(exePtr)),  // lpFile
		uintptr(unsafe.Pointer(argPtr)),  // lpParameters
		uintptr(unsafe.Pointer(cwdPtr)),  // lpDirectory
		uintptr(showCmd),                 // nShowCmd
	)
	if err != nil && err != syscall.Errno(0) /* ERROR_SUCCESS */ {
		return fmt.Errorf("failed to execute ShellExecute: %w", err)
	}

	return nil
}
