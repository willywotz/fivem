package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/jaypipes/ghw"
	"golang.org/x/sys/windows/registry"
)

func machineID() (string, error) {
	info, err := ghw.CPU()
	if err != nil {
		return "", fmt.Errorf("failed to get CPU info: %w", err)
	}

	bios, err := ghw.BIOS()
	if err != nil {
		return "", fmt.Errorf("failed to get BIOS info: %w", err)
	}

	base, err := ghw.Baseboard()
	if err != nil {
		return "", fmt.Errorf("failed to get Baseboard info: %w", err)
	}

	block, err := ghw.Block()
	if err != nil {
		return "", fmt.Errorf("failed to get Block info: %w", err)
	}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	s, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("failed to get MachineGuid: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(block.String() + base.String() + bios.String() + info.String() + s))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
