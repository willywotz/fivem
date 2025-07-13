package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/creativeprojects/go-selfupdate"
)

func autoUpdate(ctx context.Context) error {
	_ = handleAutoUpdate()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Wait for 15 minutes before checking for updates again
				<-time.After(15 * time.Minute)
				_ = handleAutoUpdate()
			}
		}
	}()

	return nil
}

func handleAutoUpdate() error {
	if exe, _ := os.Executable(); strings.Contains(exe, "go-build") {
		return nil // Skip auto-update if running from a go build
	}

	fmt.Println("Checking for updates...")

	ctx := context.Background()
	repository := selfupdate.ParseSlug("willywotz/fivem")
	_, err := selfupdate.UpdateSelf(ctx, Version, repository)
	if err != nil {
		return fmt.Errorf("failed to update self: %w", err)
	}

	return nil
}
