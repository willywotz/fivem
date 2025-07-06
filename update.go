package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/hashicorp/go-version"
	"golang.org/x/sys/windows/svc"
)

type Release struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

func getLatestRelease() (*Release, error) {
	url := "https://api.github.com/repos/willywotz/fivem/releases/latest"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch latest release: received status code %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`

		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode latest release response: %w", err)
	}
	if data.TagName == "" {
		return nil, fmt.Errorf("latest release does not have a tag name")
	}
	if len(data.Assets) == 0 {
		return nil, fmt.Errorf("latest release does not have any assets")
	}

	release := Release{
		Version:     data.TagName,
		DownloadURL: data.Assets[0].URL,
	}

	return &release, nil
}

func handleAutoUpdate() error {
	currentExePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get current exe path: %w", err)
	}

	if strings.Contains(currentExePath, "go-build") {
		return fmt.Errorf("Current executable path contains 'go-build', skipping update check")
	}

	oldFilePath := filepath.Base(currentExePath) + binaryFileName + ".old"
	if _, err := os.Stat(oldFilePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Failed to check old file existence: %v\n", err)
	} else if err := os.Remove(oldFilePath); err != nil {
		fmt.Printf("Failed to remove old file: %v\n", err)
	} else if err := hideFile(oldFilePath); err != nil {
		fmt.Printf("Failed to hide old file: %v\n", err)
	}

	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("Failed to get latest release: %w", err)
	}

	v1, _ := version.NewVersion(buildVersion)
	v2, _ := version.NewVersion(release.Version)

	if buildVersion == "" || !v1.LessThan(v2) {
		return fmt.Errorf("Current version %s is not less than latest version %s", buildVersion, release.Version)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		_ = controlService(svcName, svc.Stop, svc.Stopped)
	}

	log.Printf("Current version: %s, Latest version: %s", buildVersion, release.Version)
	log.Printf("Downloading latest release from %s", release.DownloadURL)

	resp, err := http.Get(release.DownloadURL)
	if err != nil {
		return fmt.Errorf("Failed to download latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to download latest release: received status code %d", resp.StatusCode)
	}

	err = os.Rename(currentExePath, currentExePath+".old")
	if err != nil {
		return fmt.Errorf("Failed to rename current exe: %w", err)
	}

	f, err := os.Create(currentExePath)
	if err != nil {
		return fmt.Errorf("Failed to create new binary file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("Failed to write new binary file: %w", err)
	}

	f.Close()

	cmd := exec.Command(currentExePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to start new binary: %w", err)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		if err := startService(svcName); err != nil {
			return fmt.Errorf("Failed to start service after update: %w", err)
		}
	}

	os.Exit(0)

	return nil
}

func hideFile(path string) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setFileAttributes := kernel32.NewProc("SetFileAttributesW")

	r1, _, err := setFileAttributes.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))), 2)

	if r1 == 0 {
		return err
	} else {
		return nil
	}
}
