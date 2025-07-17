package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sync"
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

		fmt.Println("Received status update request")

		if r.Method == http.MethodPost {
			var newStatus Status
			if err := json.NewDecoder(r.Body).Decode(&newStatus); err != nil {
				fmt.Printf("Failed to decode request body: %v\n", err)
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			status = append(status, newStatus)
			w.WriteHeader(http.StatusCreated)
			return
		}
	})

	http.HandleFunc("/get-status", func(w http.ResponseWriter, r *http.Request) {
		statusMu.Lock()
		defer statusMu.Unlock()

		tmpStatus := make([]Status, len(status))
		copy(tmpStatus, status)
		slices.Reverse(tmpStatus)

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
