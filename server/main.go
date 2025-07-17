package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
			for i := len(status) - 1; i >= 0; i-- {
				if time.Since(status[i].Time) > 4*time.Hour {
					statusMu.Lock()
					status = append(status[:i], status[i+1:]...)
					statusMu.Unlock()
				}
			}
		}
	}()

	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		statusMu.Lock()
		defer statusMu.Unlock()

		if r.Method == http.MethodPost {
			var newStatus Status
			defer func() { _ = r.Body.Close() }()
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(r.Body)
			if err := json.NewDecoder(buf).Decode(&newStatus); err != nil {
				hostname := r.Header.Get("Client-Hostname")
				fmt.Fprintf(os.Stderr, "[%v]: Failed to decode request body: %v\n", hostname, err)
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			status = append(status, newStatus)
			w.WriteHeader(http.StatusCreated)
			return
		}

		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/get-status", func(w http.ResponseWriter, r *http.Request) {
		statusMu.Lock()
		defer statusMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
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
