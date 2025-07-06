package main

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

const (
	SW_HIDE = iota
	SW_SHOWNORMAL
	SW_SHOWMINIMIZED
	SW_SHOWMAXIMIZED
	SW_SHOWNOACTIVATE
	SW_SHOW
	SW_MINIMIZE
	SW_SHOWMINNOACTIVE
	SW_SHOWNA
	SW_RESTORE
	SW_SHOWDEFAULT
	SW_FORCEMINIMIZE
)

var (
	shell32       = syscall.NewLazyDLL("shell32.dll")
	shellExecuteW = shell32.NewProc("ShellExecuteW")
)

func becomeAdmin() error {
	if _, err := os.Open("\\\\.\\PHYSICALDRIVE0"); err == nil {
		return nil
	}

	verb := "runas"
	exe, _ := os.Executable()
	cwd, _ := os.Getwd()
	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	_, _, err := shellExecuteW.Call(
		uintptr(0),                       // hwnd
		uintptr(unsafe.Pointer(verbPtr)), // lpOperation
		uintptr(unsafe.Pointer(exePtr)),  // lpFile
		uintptr(unsafe.Pointer(argPtr)),  // lpParameters
		uintptr(unsafe.Pointer(cwdPtr)),  // lpDirectory
		uintptr(SW_SHOWNORMAL),           // nShowCmd SW_NORMAL
	)
	if err != nil && err != syscall.Errno(0) /* ERROR_SUCCESS */ {
		return fmt.Errorf("failed to execute ShellExecute: %w", err)
	}

	os.Exit(0)

	return nil
}
