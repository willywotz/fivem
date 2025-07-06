package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func copyFile(srcPath, targetDir string) error {
	// Define the target executable path
	targetPath := filepath.Join(targetDir, filepath.Base(srcPath))

	// Copy the executable to the target path
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

	return nil
}
