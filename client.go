package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moutend/go-hook/pkg/keyboard"
	"github.com/moutend/go-hook/pkg/mouse"
	"github.com/moutend/go-hook/pkg/types"
)

var (
	lastActivityTime time.Time
	lastActivityMu   sync.Mutex
)

type Status struct {
	MachineID string `json:"machine_id"`
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Country   string `json:"country"`
	From      string `json:"from"`
	Status    string `json:"status"`
}

func UpdateClientStatus(from string) {
	fmt.Println("Updating client status...")

	machineID, _ := machineID()
	hostname, _ := os.Hostname()
	username, _ := os.LookupEnv("USERNAME")

	ip := "unknown"
	country := "unknown"
	// ipResp, err := http.Get("https://api.country.is/")
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "Failed to get public IP: %v\n", err)
	// }
	// defer func() { _ = ipResp.Body.Close() }()
	// if ipResp.StatusCode == http.StatusOK {
	// 	var ipData struct {
	// 		IP      string `json:"ip"`
	// 		Country string `json:"country"`
	// 	}
	// 	if err := json.NewDecoder(ipResp.Body).Decode(&ipData); err != nil {
	// 		fmt.Fprintf(os.Stderr, "Failed to decode IP response: %v\n", err)
	// 	} else {
	// 		ip = ipData.IP
	// 		country = ipData.Country
	// 	}
	// }

	status := "active"
	lastActivityMu.Lock()
	if time.Since(lastActivityTime) > 5*time.Minute {
		status = "away"
	}
	lastActivityMu.Unlock()

	data := Status{
		MachineID: machineID,
		Hostname:  hostname,
		Username:  username,
		IP:        ip,
		Country:   country,
		From:      from,
		Status:    status,
	}

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode status data: %v\n", err)
		return
	}

	baseURL := GetTxt("base_url", "http://localhost:8080")
	r, err := http.NewRequest(http.MethodPost, baseURL+"/status", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
		return
	}

	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "fivem-tools-client")
	r.Header.Set("Client-Hostname", hostname)

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to post status: %v\n", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		fmt.Fprintf(os.Stderr, "Failed to post status, got status code: %d\n", resp.StatusCode)
		return
	}
}

func handleUpdateClientStatus(from string) {
	lastActivityMu.Lock()
	lastActivityTime = time.Now()
	lastActivityMu.Unlock()

	keyboardChan := make(chan types.KeyboardEvent, 100)
	_ = keyboard.Install(nil, keyboardChan)
	defer func() { _ = keyboard.Uninstall() }()

	mouseChan := make(chan types.MouseEvent, 100)
	_ = mouse.Install(nil, mouseChan)
	defer func() { _ = mouse.Uninstall() }()

	timeChan := make(chan time.Time, 1)

	go func() {
		for {
			select {
			case <-keyboardChan:
				select {
				case timeChan <- time.Now():
				default:
				}
			case <-mouseChan:
				select {
				case timeChan <- time.Now():
				default:
				}
			}
		}
	}()

	go func() {
		for t := range timeChan {
			lastActivityMu.Lock()
			lastActivityTime = t
			lastActivityMu.Unlock()
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		UpdateClientStatus(from)

		statusTickStr := GetTxt("status_tick", "300")
		statusTickInt, _ := strconv.Atoi(statusTickStr)
		statusTick := time.Duration(statusTickInt) * time.Second

		if statusTick <= 0 {
			statusTick = 300 * time.Second // Default to 5 minutes if invalid
		}

		time.Sleep(statusTick)
	}
}

var (
	mapTxts     map[string]string
	mapTxtsMu   sync.Mutex
	mapTxtsTime time.Time
)

func GetTxt(name string, defaultValue ...string) string {
	mapTxtsMu.Lock()
	defer mapTxtsMu.Unlock()

	ttl := time.Since(mapTxtsTime) < 5*time.Minute
	if v, ok := mapTxts[name]; ok && ttl {
		return v
	}

	localMapTxts := make(map[string]string)
	txts, err := net.LookupTXT("_fivem_tools.willywotz.com")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to lookup TXT records: %v\n", err)
		return getOrDefaultMap(localMapTxts, name, defaultValue...)
	}

	for _, txt := range txts {
		for _, part := range strings.Split(txt, ";") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			localMapTxts[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	if len(localMapTxts) == 0 {
		fmt.Fprintf(os.Stderr, "No valid TXT records found\n")
		return getOrDefaultMap(localMapTxts, name, defaultValue...)
	}

	mapTxts = localMapTxts
	mapTxtsTime = time.Now()

	return getOrDefaultMap(localMapTxts, name, defaultValue...)
}

func getOrDefaultMap[T any](m map[string]T, key string, defaultValue ...T) T {
	if value, ok := m[key]; ok {
		return value
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	var zeroValue T
	return zeroValue
}
