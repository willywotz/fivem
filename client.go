package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func UpdateClientStatus() {
	txts, err := net.LookupTXT("_fivem_tools.willywotz.com")
	if err != nil {
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
		return
	}

	machineID, _ := machineID()
	hostname, _ := os.Hostname()
	username, _ := os.LookupEnv("USERNAME")

	ip := "unknown"
	ipResp, _ := http.Get("https://api.ipify.org")
	defer func() { _ = ipResp.Body.Close() }()
	if ipResp.StatusCode == http.StatusOK {
		ipBytes, _ := io.ReadAll(ipResp.Body)
		ip = strings.TrimSpace(string(ipBytes))
	}

	data := map[string]any{
		"machine_id": machineID,
		"hostname":   hostname,
		"username":   username,
		"ip":         ip,

		"time": time.Now().Unix(),
	}

	body := bytes.NewBuffer(nil)
	if err := json.NewEncoder(body).Encode(data); err != nil {
		return
	}
	_, _ = http.Post(baseURL+"/status", "application/json", body)
}
