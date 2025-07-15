package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"golang.org/x/sys/windows/svc"
)

func update() error {
	if exe, _ := os.Executable(); strings.Contains(exe, "go-build") {
		return nil
	}

	if err := handleUpdate(); err != nil {
		fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
	}

	ticker := time.NewTicker(5 * time.Minute)

	go func() {
		for range ticker.C {
			if err := handleUpdate(); err != nil {
				fmt.Fprintf(os.Stderr, "Error checking for updates: %v\n", err)
			}
		}
	}()

	return nil
}

func handleUpdate() error {
	fmt.Println("Checking for updates...")

	ctx := context.Background()
	repository := selfupdate.ParseSlug("willywotz/fivem")
	release, err := selfupdate.UpdateSelf(ctx, version, repository)
	if err != nil {
		return fmt.Errorf("failed to update self: %w", err)
	}

	if release.GreaterThan(version) {
		fmt.Printf("Updated to version %s, restarting...\n", release.Version())

		if inService, _ := svc.IsWindowsService(); inService {
			return startService(svcName)
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		if _, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
		}); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}

		os.Exit(0)
	}

	return nil
}
