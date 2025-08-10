package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	machineIDCache   string
	machineIDCacheMu sync.Mutex
)

func machineID() (string, error) {
	machineIDCacheMu.Lock()
	defer machineIDCacheMu.Unlock()

	if machineIDCache != "" {
		return machineIDCache, nil
	}

	ss := make([]string, 0)

	// block, err := ghw.Block()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get Block info: %w", err)
	// }
	// ss = append(ss, block.String())

	// base, err := ghw.Baseboard()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get Baseboard info: %w", err)
	// }
	// ss = append(ss, base.String())

	// bios, err := ghw.BIOS()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get BIOS info: %w", err)
	// }
	// ss = append(ss, bios.String())

	// info, err := ghw.CPU()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get CPU info: %w", err)
	// }
	// ss = append(ss, info.String())

	// k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	// if err != nil {
	// 	return "", fmt.Errorf("failed to open registry key: %w", err)
	// }
	// defer func() { _ = k.Close() }()

	// machineGuid, _, err := k.GetStringValue("MachineGuid")
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get MachineGuid: %w", err)
	// }
	// ss = append(ss, machineGuid)

	localHostname, _ := os.Hostname()
	ss = append(ss, localHostname)

	localUsername, _ := os.LookupEnv("USERNAME")
	ss = append(ss, localUsername)

	h := sha256.New()
	h.Write([]byte(strings.Join(ss, "")))
	machineIDCache = fmt.Sprintf("%x", h.Sum(nil))

	return machineIDCache, nil
}
