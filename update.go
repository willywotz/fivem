package main

import (
	"context"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/creativeprojects/go-selfupdate"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
)

func update() error {
	if localDebug || noUpdate {
		return nil
	}

	go func() {
		if err := handleUpdate(); err != nil {
			failedf("Error checking for updates: %v", err)
		}

		for range time.NewTicker(5 * time.Minute).C {
			if err := handleUpdate(); err != nil {
				failedf("Error checking for updates: %v", err)
			}
		}
	}()

	return nil
}

func handleUpdate() error {
	ctx := context.Background()
	repository := selfupdate.ParseSlug("willywotz/fivem")
	release, err := selfupdate.UpdateSelf(ctx, version, repository)
	if err != nil {
		return fmt.Errorf("failed to update self: %w", err)
	}

	if release.GreaterThan(version) {
		failedf("Updated to version %s, restarting...", release.Version())

		if inService, _ := svc.IsWindowsService(); inService {
			os.Exit(1)
		}

		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		if _, err := os.StartProcess(exe, os.Args, &os.ProcAttr{
			Files: []*os.File{nil, nil, nil},
			Sys: &syscall.SysProcAttr{
				CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS | windows.CREATE_NO_WINDOW,
			},
		}); err != nil {
			return fmt.Errorf("failed to restart: %w", err)
		}

		os.Exit(0)
	}

	return nil
}
