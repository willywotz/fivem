package main

import "syscall"

var (
	user32  = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")
	procSendInput      = user32.NewProc("SendInput")

	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)
