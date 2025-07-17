package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Status struct {
	MachineID string    `json:"machine_id"`
	Hostname  string    `json:"hostname"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	From      string    `json:"from"`
	Time      time.Time `json:"time"`
}

func UpdateClientStatus(from string) {
	fmt.Println("Updating client status...")

	txts, err := net.LookupTXT("_fivem_tools.willywotz.com")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to lookup TXT records: %v\n", err)
		return
	}
	mapTxts := make(map[string]string)
	for _, txt := range txts {
		for _, part := range strings.Split(txt, ";") {
			kv := strings.SplitN(part, "=", 2)
			if len(kv) == 2 {
				mapTxts[kv[0]] = kv[1]
			}
		}
	}
	var baseURL string
	if val, ok := mapTxts["url"]; ok {
		baseURL = val
	} else {
		fmt.Fprintf(os.Stderr, "No URL found in TXT records\n")
		return
	}

	machineID, _ := machineID()
	hostname, _ := os.Hostname()
	username, _ := os.LookupEnv("USERNAME")

	ip := "unknown"
	ipResp, err := http.Get("https://api.ipify.org")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get public IP: %v\n", err)
	}
	defer func() { _ = ipResp.Body.Close() }()
	if ipResp.StatusCode == http.StatusOK {
		ipBytes, _ := io.ReadAll(ipResp.Body)
		ip = strings.TrimSpace(string(ipBytes))
	}

	data := Status{
		MachineID: machineID,
		Hostname:  hostname,
		Username:  username,
		IP:        ip,
		From:      from,
		Time:      time.Now(),
	}

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(data); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode status data: %v\n", err)
		return
	}

	_, err = http.Post(baseURL+"/status", "application/json", body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to post status: %v\n", err)
		return
	}
}

func handleUpdateClientStatus(from string) {
	UpdateClientStatus(from)

	for range time.Tick(1 * time.Second) {
		UpdateClientStatus(from)
	}
}
