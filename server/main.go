package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for simplicity in example.
		// In production, tighten this for security!
		// Example: return r.Host == "yourdomain.com" || r.Host == "www.yourdomain.com"
		return true
	},
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(os.Stdout).Encode(r.Header) // Log request headers for debugging

	// Upgrade the HTTP connection to a WebSocket connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	defer conn.Close() // Close the connection when the function returns

	log.Printf("Client connected from %s", r.RemoteAddr)

	// Loop to read and write messages
	for {
		// Read message from client
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break // Exit loop on error (e.g., client disconnected)
		}
		log.Printf("Received message from client: %s (Type: %d)", p, messageType)

		// Echo the message back to the client
		echoMessage := fmt.Sprintf("Server received: %s at %s", p, time.Now().Format(time.RFC3339))
		if err := conn.WriteMessage(messageType, []byte(echoMessage)); err != nil {
			log.Printf("Error writing message: %v", err)
			break // Exit loop on error
		}

		// Optional: Send a periodic message from server to client
		time.Sleep(1 * time.Second)
		if err := conn.WriteMessage(websocket.TextMessage, []byte("Hello from server!")); err != nil {
			log.Printf("Error sending periodic message: %v", err)
			break
		}
	}
	log.Printf("Client disconnected from %s", r.RemoteAddr)
}

type Status struct {
	MachineID string `json:"machine_id"`
	Hostname  string `json:"hostname"`
	Username  string `json:"username"`
	IP        string `json:"ip"`
	Country   string `json:"country"`
	From      string `json:"from"`
	Status    string `json:"status"`

	Time time.Time `json:"time"`
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

	http.HandleFunc("/ws", wsHandler)

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.WriteHeader(http.StatusOK)

		htmlContent, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(htmlContent)
	})

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

	log.Println("Starting server on :8080")
	log.Println(http.ListenAndServe(":8080", nil))
}
