package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

func main() {
	http.HandleFunc("/", IndexHandler)

	_ = http.ListenAndServe(":8080", nil)
}

type Logger struct {
	MachineID string `json:"machine_id"`
	Time      string `json:"time"`
	Action    string `json:"action"`
	Message   string `json:"message"`
}

var logs = make([]Logger, 0)

var muLogs sync.Mutex

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	muLogs.Lock()
	defer muLogs.Unlock()

	if r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(logs)
		return
	}

	if r.Method == http.MethodPost {
		var logEntry Logger
		if err := json.NewDecoder(r.Body).Decode(&logEntry); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logEntry.Time = time.Now().Format(time.RFC3339)
		logs = append(logs, logEntry)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
