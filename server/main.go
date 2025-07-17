package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"
)

type Status struct {
	MachineID string    `json:"machine_id"`
	Hostname  string    `json:"hostname"`
	Username  string    `json:"username"`
	IP        string    `json:"ip"`
	Country   string    `json:"country"`
	From      string    `json:"from"`
	Time      time.Time `json:"time"`
}

func main() {
	status := make([]Status, 0)
	statusMu := &sync.Mutex{}

	go func() {
		for range time.Tick(4 * time.Hour) {
			statusMu.Lock()
			newStatus := make([]Status, 0)
			for _, s := range status {
				if time.Since(s.Time) < 24*time.Hour {
					newStatus = append(newStatus, s)
				}
			}
			status = newStatus
			statusMu.Unlock()
		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		statusMu.Lock()
		defer statusMu.Unlock()

		if r.Method == http.MethodPost {
			var newStatus Status
			defer func() { _ = r.Body.Close() }()
			if err := json.NewDecoder(r.Body).Decode(&newStatus); err != nil {
				hostname := r.Header.Get("Client-Hostname")
				fmt.Fprintf(os.Stderr, "[%v]: Failed to decode request body: %v\n", hostname, err)
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			newStatus.Time = time.Now()
			status = append(status, newStatus)
			w.WriteHeader(http.StatusCreated)
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/get-status", func(w http.ResponseWriter, r *http.Request) {
		statusMu.Lock()
		defer statusMu.Unlock()

		tmpStatus := make([]Status, len(status))
		copy(tmpStatus, status)
		slices.Reverse(tmpStatus)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=0")
		if err := json.NewEncoder(w).Encode(tmpStatus); err != nil {
			fmt.Printf("Failed to encode status: %v\n", err)
			http.Error(w, "Failed to encode status", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	_ = http.ListenAndServe(":8080", nil)
}
