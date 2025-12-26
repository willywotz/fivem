package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	browserClients = make(map[*websocket.Conn]*BrowserClient)
	audioClients   = make(map[string]*AudioClientInfo)
	clientMutex    sync.RWMutex
	port           = flag.String("port", "8080", "Server port")
)

type BrowserClient struct {
	ID              string
	Conn            *websocket.Conn
	SelectedSources map[string]bool // audio client IDs to listen to
	Volume          float32
}

type AudioClientInfo struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	LastSeen      time.Time `json:"lastSeen"`
	SampleRate    uint32    `json:"sampleRate"`
	Channels      uint16    `json:"channels"`
	BitsPerSample uint16    `json:"bitsPerSample"`
	Format        string    `json:"format"`
	PacketCount   int64     `json:"packetCount"`
	CurrentAudio  []byte    `json:"-"`
}

type AudioMessage struct {
	Type          string `json:"type"`
	AudioData     string `json:"audioData,omitempty"`
	SampleRate    uint32 `json:"sampleRate,omitempty"`
	Channels      uint16 `json:"channels,omitempty"`
	Format        string `json:"format,omitempty"`
	BitsPerSample uint16 `json:"bitsPerSample,omitempty"`
	ClientID      string `json:"clientId,omitempty"`
	ClientName    string `json:"clientName,omitempty"`
	MachineID     string `json:"machineId,omitempty"`
}

type AudioMessageRaw struct {
	Type          string `json:"type"`
	AudioData     []byte `json:"audioData"`
	SampleRate    uint32 `json:"sampleRate"`
	Channels      uint16 `json:"channels"`
	Format        string `json:"format"`
	BitsPerSample uint16 `json:"bitsPerSample"`
	ClientName    string `json:"clientName,omitempty"`
	MachineID     string `json:"machineId,omitempty"`
}

type ControlMessage struct {
	Type            string            `json:"type"`
	SelectedSources map[string]bool   `json:"selectedSources,omitempty"`
	Volume          float32           `json:"volume,omitempty"`
	AudioClients    []AudioClientInfo `json:"audioClients,omitempty"`
	WatchClients    bool              `json:"watchClients,omitempty"`
}

func generateClientID() string {
	return fmt.Sprintf("client_%d", time.Now().UnixNano())
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	browserClientID := generateClientID()
	browserClient := &BrowserClient{
		ID:              browserClientID,
		Conn:            conn,
		SelectedSources: make(map[string]bool),
		Volume:          1.0,
	}

	// Register browser client
	clientMutex.Lock()
	browserClients[conn] = browserClient
	clientMutex.Unlock()

	// Unregister client on disconnect
	defer func() {
		clientMutex.Lock()
		delete(browserClients, conn)
		clientMutex.Unlock()
		log.Printf("Browser client disconnected: %s, remaining: %d", browserClientID, len(browserClients))
	}()

	log.Printf("Browser client connected: %s, total browser clients: %d", browserClientID, len(browserClients))

	// Send current audio client list immediately
	sendClientList(conn)

	// Handle messages from browser client
messageLoop:
	for {
		var msg ControlMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		// Handle control messages
		switch msg.Type {
		case "selectSources":
			clientMutex.Lock()
			browserClient.SelectedSources = msg.SelectedSources
			clientMutex.Unlock()
			log.Printf("Browser client %s selected sources: %v", browserClientID, msg.SelectedSources)
		case "setVolume":
			clientMutex.Lock()
			browserClient.Volume = msg.Volume
			clientMutex.Unlock()
		case "requestClientList":
			sendClientList(conn)
		case "watchClients":
			// Client is requesting to watch for changes (handled automatically)
			log.Printf("Browser client %s started watching client list", browserClientID)
		case "ping":
			// Respond to ping to keep connection alive
			if err := conn.WriteJSON(ControlMessage{Type: "pong"}); err != nil {
				log.Printf("Failed to send pong: %v", err)
				break messageLoop
			}
		}
	}
}

func handleAudioData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var audioMsg AudioMessageRaw
	if err := json.Unmarshal(body, &audioMsg); err != nil {
		http.Error(w, "Failed to parse JSON", http.StatusBadRequest)
		return
	}

	// Use machine ID as the primary identifier, fallback to header-based ID
	var clientID string
	if audioMsg.MachineID != "" {
		clientID = audioMsg.MachineID
	} else {
		// Fallback to header-based ID for backward compatibility
		clientID = r.Header.Get("X-Client-ID")
		if clientID == "" {
			clientID = generateClientID()
			w.Header().Set("X-Client-ID", clientID)
		}
		log.Printf("Warning: Audio packet received without machine ID, using fallback ID: %s", clientID)
	}

	clientMutex.Lock()
	isNewClient := false
	// Update or create audio client info
	if audioClient, exists := audioClients[clientID]; exists {
		audioClient.LastSeen = time.Now()
		audioClient.PacketCount++
		audioClient.CurrentAudio = audioMsg.AudioData

		// Update audio format info in case it changed
		audioClient.SampleRate = audioMsg.SampleRate
		audioClient.Channels = audioMsg.Channels
		audioClient.BitsPerSample = audioMsg.BitsPerSample
		audioClient.Format = audioMsg.Format
	} else {
		clientName := audioMsg.ClientName
		if clientName == "" {
			if audioMsg.MachineID != "" {
				clientName = fmt.Sprintf("Machine %s", audioMsg.MachineID[:8])
			} else {
				clientName = fmt.Sprintf("Audio Client %s", clientID[len(clientID)-8:])
			}
		}
		audioClients[clientID] = &AudioClientInfo{
			ID:            clientID,
			Name:          clientName,
			LastSeen:      time.Now(),
			SampleRate:    audioMsg.SampleRate,
			Channels:      audioMsg.Channels,
			BitsPerSample: audioMsg.BitsPerSample,
			Format:        audioMsg.Format,
			PacketCount:   1,
			CurrentAudio:  audioMsg.AudioData,
		}
		isNewClient = true
		log.Printf("New audio client registered: %s (Machine ID: %s)", clientName, clientID)
	}
	clientMutex.Unlock()

	// Notify all browser clients about new audio source if it's a new client
	if isNewClient {
		broadcastClientList()
	}

	// Send audio to selected browser clients
	broadcastSelectiveAudio(clientID, audioMsg.AudioData, audioMsg.SampleRate, audioMsg.Channels, audioMsg.BitsPerSample)

	w.WriteHeader(http.StatusOK)
}

func broadcastSelectiveAudio(sourceClientID string, audioData []byte, sampleRate uint32, channels uint16, bitsPerSample uint16) {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	sourceInfo := audioClients[sourceClientID]
	if sourceInfo == nil {
		return
	}

	for conn, browserClient := range browserClients {
		// Check if this browser client wants to hear this audio source
		if !browserClient.SelectedSources[sourceClientID] {
			continue
		}

		msg := AudioMessage{
			Type:          "audio",
			AudioData:     base64.StdEncoding.EncodeToString(audioData),
			SampleRate:    sampleRate,
			Channels:      channels,
			Format:        "pcm",
			BitsPerSample: bitsPerSample,
			ClientID:      sourceClientID,
			ClientName:    sourceInfo.Name,
		}

		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Failed to send audio data: %v", err)
			conn.Close()
			delete(browserClients, conn)
		}
	}
}

func broadcastClientList() {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	var clientList []AudioClientInfo
	for _, client := range audioClients {
		clientList = append(clientList, *client)
	}

	msg := ControlMessage{
		Type:         "clientList",
		AudioClients: clientList,
	}

	for conn := range browserClients {
		if err := conn.WriteJSON(msg); err != nil {
			log.Printf("Failed to send client list: %v", err)
		}
	}
}

func sendClientList(conn *websocket.Conn) {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	var clientList []AudioClientInfo
	for _, client := range audioClients {
		clientList = append(clientList, *client)
	}

	msg := ControlMessage{
		Type:         "clientList",
		AudioClients: clientList,
	}

	if err := conn.WriteJSON(msg); err != nil {
		log.Printf("Failed to send client list: %v", err)
	}
}

// Clean up stale audio clients
func cleanupStaleClients() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		clientMutex.Lock()
		staleThreshold := time.Now().Add(-60 * time.Second)
		hasChanges := false
		removedClients := []string{}

		for id, client := range audioClients {
			if client.LastSeen.Before(staleThreshold) {
				log.Printf("Removing stale audio client: %s (%s)", client.Name, id)
				removedClients = append(removedClients, client.Name)
				delete(audioClients, id)
				hasChanges = true
			}
		}
		clientMutex.Unlock()

		if hasChanges {
			log.Printf("Cleaned up %d stale clients: %v", len(removedClients), removedClients)
			broadcastClientList()
		}
	}
}

// Send periodic client list updates to maintain freshness
func periodicClientListUpdate() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		clientMutex.RLock()
		hasClients := len(audioClients) > 0
		browserCount := len(browserClients)
		clientMutex.RUnlock()

		if hasClients && browserCount > 0 {
			broadcastClientList()
		}
	}
}

func main() {
	flag.Parse()

	// Check for environment variable override
	if envPort := os.Getenv("PORT"); envPort != "" {
		*port = envPort
	}

	// Start background routines
	go cleanupStaleClients()
	go periodicClientListUpdate()

	// Handle audio data from capture client
	http.HandleFunc("/audio", handleAudioData)

	// Handle WebSocket connections from browsers
	http.HandleFunc("/ws", handleWebSocket)

	// WebSocket endpoint specifically for watching client list changes
	http.HandleFunc("/ws/clients", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		// Send initial client list
		sendClientList(conn)

		// Keep connection alive and send updates
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			sendClientList(conn)
		}
	})

	// API endpoint to get audio clients
	http.HandleFunc("/api/clients", func(w http.ResponseWriter, r *http.Request) {
		clientMutex.RLock()
		var clientList []AudioClientInfo
		for _, client := range audioClients {
			clientList = append(clientList, *client)
		}
		clientMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(clientList)
	})

	// API endpoint for detailed client statistics
	http.HandleFunc("/api/stats", handleClientStats)

	// Health check endpoint with more details
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		clientMutex.RLock()
		audioCount := len(audioClients)
		browserCount := len(browserClients)
		clientMutex.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		healthInfo := map[string]interface{}{
			"status":         "running",
			"port":           *port,
			"audioClients":   audioCount,
			"browserClients": browserCount,
			"timestamp":      time.Now().Format(time.RFC3339),
		}
		json.NewEncoder(w).Encode(healthInfo)
	})

	// Serve HTML page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlPage)
	})

	addr := ":" + *port
	log.Printf("Audio server starting on %s", addr)
	log.Printf("Browser interface: http://localhost%s", addr)
	log.Printf("WebSocket endpoint: ws://localhost%s/ws", addr)
	log.Printf("Client watch endpoint: ws://localhost%s/ws/clients", addr)
	log.Printf("API endpoints: /api/clients, /api/stats")
	log.Printf("Health check: http://localhost%s/health", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

const htmlPage = `
<!DOCTYPE html>
<html>
<head>
	<title>Multi-Client Audio Stream Server</title>
	<style>
		body { font-family: Arial, sans-serif; margin: 20px; }
		.client-list { margin: 20px 0; padding: 15px; border: 1px solid #ccc; border-radius: 5px; }
		.client-item { margin: 10px 0; padding: 10px; background: #f5f5f5; border-radius: 3px; }
		.client-checkbox { margin-right: 10px; }
		.controls { margin: 10px 0; }
		button { padding: 10px 20px; margin: 5px; }
		input[type="range"] { width: 200px; }
		.status { padding: 10px; margin: 10px 0; border-radius: 3px; }
		.status.connected { background: #d4edda; color: #155724; }
		.status.disconnected { background: #f8d7da; color: #721c24; }
		.auto-refresh { margin: 10px 0; }
		.connection-info { font-size: 0.9em; color: #666; margin: 10px 0; }
		.machine-id { font-family: monospace; color: #666; }
	</style>
</head>
<body>
	<h1>Multi-Client Audio Stream Server</h1>

	<div class="controls">
		<button id="start">Start Audio</button>
		<button id="stop">Stop Audio</button>
		<button id="refreshClients">Refresh Client List</button>
		<button id="showStats">Show Statistics</button>
	</div>

	<div id="status" class="status">Ready</div>

	<div class="connection-info">
		<div>WebSocket Status: <span id="wsStatus">Disconnected</span></div>
		<div>Last Update: <span id="lastUpdate">Never</span></div>
		<div>Total Clients: <span id="clientCount">0</span></div>
	</div>

	<div class="auto-refresh">
		<label>
			<input type="checkbox" id="autoRefresh" checked>
			Auto-refresh client list (real-time updates)
		</label>
	</div>

	<div>
		<label>Master Volume: <input type="range" id="volume" min="0" max="100" value="50"></label>
		<span id="volumeValue">50%</span>
	</div>

	<div class="client-list">
		<h3>Available Audio Sources:</h3>
		<div id="clientList">No audio sources available</div>
	</div>

	<div id="audioInfo">Audio format info will appear here</div>

	<script>
		const startBtn = document.getElementById('start');
		const stopBtn = document.getElementById('stop');
		const refreshBtn = document.getElementById('refreshClients');
		const status = document.getElementById('status');
		const audioInfo = document.getElementById('audioInfo');
		const volumeSlider = document.getElementById('volume');
		const volumeValue = document.getElementById('volumeValue');
		const clientList = document.getElementById('clientList');
		const wsStatus = document.getElementById('wsStatus');
		const lastUpdate = document.getElementById('lastUpdate');
		const clientCount = document.getElementById('clientCount');
		const autoRefresh = document.getElementById('autoRefresh');

		let ws = null;
		let audioContext = null;
		let gainNode = null;
		let audioBuffers = {};
		let nextPlayTimes = {};
		let selectedSources = {};
		let audioClients = {};
		let clientListUpdateTimer = null;

		function setStatus(msg, isConnected = false) {
			status.textContent = msg;
			status.className = 'status ' + (isConnected ? 'connected' : 'disconnected');
			console.log(msg);
		}

		function updateWSStatus(connected) {
			wsStatus.textContent = connected ? 'Connected' : 'Disconnected';
			wsStatus.style.color = connected ? '#155724' : '#721c24';
		}

		function updateLastUpdate() {
			lastUpdate.textContent = new Date().toLocaleTimeString();
		}

		function updateVolumeDisplay() {
			volumeValue.textContent = volumeSlider.value + '%';
		}

		function sendControlMessage(type, data = {}) {
			if (ws && ws.readyState === WebSocket.OPEN) {
				ws.send(JSON.stringify({ type, ...data }));
			}
		}

		function updateClientList(clients) {
			audioClients = {};
			clients.forEach(client => {
				audioClients[client.id] = client;
			});

			clientCount.textContent = clients.length;
			updateLastUpdate();

			if (clients.length === 0) {
				clientList.innerHTML = 'No audio sources available';
				return;
			}

			const html = clients.map(client =>
				'<div class="client-item">' +
					'<label>' +
						'<input type="checkbox" class="client-checkbox" data-client-id="' + client.id + '"' +
							   (selectedSources[client.id] ? ' checked' : '') + '>' +
						'<strong>' + client.name + '</strong>' +
						'<br>' +
						'<div class="machine-id">Machine ID: ' + client.id + '</div>' +
						'<small>Format: ' + client.sampleRate + 'Hz, ' + client.channels + 'ch, ' + client.bitsPerSample + 'bit |' +
							   ' Packets: ' + client.packetCount + ' |' +
							   ' Last seen: ' + new Date(client.lastSeen).toLocaleTimeString() + '</small>' +
					'</label>' +
				'</div>'
			).join('');

			clientList.innerHTML = html;

			// Add event listeners to checkboxes
			document.querySelectorAll('.client-checkbox').forEach(checkbox => {
				checkbox.addEventListener('change', (e) => {
					const clientId = e.target.dataset.clientId;
					selectedSources[clientId] = e.target.checked;
					sendControlMessage('selectSources', { selectedSources });
				});
			});
		}

		// Add statistics view
		document.getElementById('showStats').addEventListener('click', async () => {
			try {
				const response = await fetch('/api/stats');
				const stats = await response.json();
				const statsWindow = window.open('', '_blank', 'width=600,height=400');
				statsWindow.document.write(
					'<html>' +
					'<head><title>Server Statistics</title></head>' +
					'<body>' +
						'<h2>Audio Server Statistics</h2>' +
						'<p><strong>Total Audio Clients:</strong> ' + stats.totalAudioClients + '</p>' +
						'<p><strong>Total Browser Clients:</strong> ' + stats.totalBrowserClients + '</p>' +
						'<h3>Audio Clients Details:</h3>' +
						'<pre>' + JSON.stringify(stats.audioClients, null, 2) + '</pre>' +
						'<h3>Browser Clients:</h3>' +
						'<pre>' + JSON.stringify(stats.browserClients, null, 2) + '</pre>' +
					'</body>' +
					'</html>'
				);
				statsWindow.document.close();
			} catch (error) {
				alert('Failed to fetch statistics: ' + error.message);
			}
		});

		// Keep connection alive with periodic pings
		function startHeartbeat() {
			return setInterval(() => {
				if (ws && ws.readyState === WebSocket.OPEN) {
					sendControlMessage('ping');
				}
			}, 30000); // Ping every 30 seconds
		}

		function base64ToArrayBuffer(base64) {
			const binaryString = window.atob(base64);
			const len = binaryString.length;
			const bytes = new Uint8Array(len);
			for (let i = 0; i < len; i++) {
				bytes[i] = binaryString.charCodeAt(i);
			}
			return bytes.buffer;
		}

		function scheduleAudioPlayback(clientId) {
			if (!audioBuffers[clientId]) audioBuffers[clientId] = [];
			if (!nextPlayTimes[clientId]) nextPlayTimes[clientId] = 0;

			while (audioBuffers[clientId].length > 0) {
				const audioData = audioBuffers[clientId].shift();
				const source = audioContext.createBufferSource();
				source.buffer = audioData.buffer;
				source.connect(gainNode);
				gainNode.connect(audioContext.destination);

				if (nextPlayTimes[clientId] < audioContext.currentTime) {
					nextPlayTimes[clientId] = audioContext.currentTime;
				}

				source.start(nextPlayTimes[clientId]);
				nextPlayTimes[clientId] += audioData.buffer.duration;
			}
		}

		function playAudioData(audioData, sampleRate, channels, bitsPerSample, clientId, clientName) {
			if (!audioContext) return;

			try {
				const arrayBuffer = base64ToArrayBuffer(audioData);
				const bytesPerSample = bitsPerSample / 8;
				const numFrames = arrayBuffer.byteLength / (channels * bytesPerSample);

				const webAudioBuffer = audioContext.createBuffer(channels, numFrames, sampleRate);

				if (bitsPerSample === 32) {
					const float32Array = new Float32Array(arrayBuffer);
					for (let channel = 0; channel < channels; channel++) {
						const channelData = webAudioBuffer.getChannelData(channel);
						for (let i = 0; i < numFrames; i++) {
							channelData[i] = float32Array[i * channels + channel];
						}
					}
				} else if (bitsPerSample === 16) {
					const int16Array = new Int16Array(arrayBuffer);
					for (let channel = 0; channel < channels; channel++) {
						const channelData = webAudioBuffer.getChannelData(channel);
						for (let i = 0; i < numFrames; i++) {
							channelData[i] = int16Array[i * channels + channel] / 32768.0;
						}
					}
				} else if (bitsPerSample === 8) {
					const uint8Array = new Uint8Array(arrayBuffer);
					for (let channel = 0; channel < channels; channel++) {
						const channelData = webAudioBuffer.getChannelData(channel);
						for (let i = 0; i < numFrames; i++) {
							channelData[i] = (uint8Array[i * channels + channel] - 128) / 128.0;
						}
					}
				}

				if (!audioBuffers[clientId]) audioBuffers[clientId] = [];
				audioBuffers[clientId].push({ buffer: webAudioBuffer });

				if (audioBuffers[clientId].length > 10) {
					audioBuffers[clientId].shift();
				}

				scheduleAudioPlayback(clientId);
			} catch (error) {
				console.error('Error playing audio:', error);
			}
		}

		volumeSlider.addEventListener('input', (e) => {
			updateVolumeDisplay();
			if (gainNode) {
				gainNode.gain.value = e.target.value / 100;
			}
			sendControlMessage('setVolume', { volume: e.target.value / 100 });
		});

		refreshBtn.addEventListener('click', () => {
			sendControlMessage('requestClientList');
		});

		autoRefresh.addEventListener('change', (e) => {
			if (e.target.checked && ws && ws.readyState === WebSocket.OPEN) {
				sendControlMessage('watchClients', { watchClients: true });
			}
		});

		startBtn.addEventListener('click', async () => {
			try {
				setStatus('Connecting...');

				audioContext = new (window.AudioContext || window.webkitAudioContext)();
				gainNode = audioContext.createGain();
				gainNode.gain.value = volumeSlider.value / 100;

				ws = new WebSocket('ws://' + window.location.hostname + ':' + window.location.port + '/ws');

				ws.onopen = () => {
					setStatus('Connected - waiting for audio sources', true);
					updateWSStatus(true);
					sendControlMessage('requestClientList');
					if (autoRefresh.checked) {
						sendControlMessage('watchClients', { watchClients: true });
					}
					clientListUpdateTimer = startHeartbeat();
				};

				ws.onmessage = (event) => {
					try {
						const message = JSON.parse(event.data);

						if (message.type === 'audio') {
							audioInfo.textContent = "Playing: " + message.clientName + " | Format: " + message.sampleRate + "Hz, " + message.channels + "ch, " + message.bitsPerSample + "bit";
							playAudioData(message.audioData, message.sampleRate, message.channels, message.bitsPerSample, message.clientId, message.clientName);
						} else if (message.type === 'clientList') {
							updateClientList(message.audioClients || []);
						} else if (message.type === 'pong') {
							// Heartbeat response received
						}
					} catch (err) {
						console.error('Error handling message:', err);
					}
				};

				ws.onerror = (err) => {
					setStatus('WebSocket error');
					updateWSStatus(false);
					console.error('WebSocket error:', err);
				};

				ws.onclose = () => {
					setStatus('Disconnected');
					updateWSStatus(false);
					if (clientListUpdateTimer) {
						clearInterval(clientListUpdateTimer);
						clientListUpdateTimer = null;
					}
				};

				startBtn.disabled = true;
				stopBtn.disabled = false;
				refreshBtn.disabled = false;
			} catch (err) {
				setStatus('Error: ' + err.message);
			}
		});

		stopBtn.addEventListener('click', () => {
			if (ws) {
				ws.close();
				ws = null;
			}
			if (audioContext) {
				audioContext.close();
				audioContext = null;
				gainNode = null;
			}
			if (clientListUpdateTimer) {
				clearInterval(clientListUpdateTimer);
				clientListUpdateTimer = null;
			}
			audioBuffers = {};
			nextPlayTimes = {};
			startBtn.disabled = false;
			stopBtn.disabled = true;
			refreshBtn.disabled = true;
			updateWSStatus(false);
			setStatus('Stopped');
		});

		// Initialize
		updateVolumeDisplay();
		stopBtn.disabled = true;
		refreshBtn.disabled = true;
	</script>
</body>
</html>
`

func handleClientStats(w http.ResponseWriter, r *http.Request) {
	clientMutex.RLock()
	defer clientMutex.RUnlock()

	var audioClientsList []AudioClientInfo
	for _, client := range audioClients {
		audioClientsList = append(audioClientsList, *client)
	}

	var browserClientsList []map[string]interface{}
	for _, client := range browserClients {
		browserClientsList = append(browserClientsList, map[string]interface{}{
			"id":              client.ID,
			"selectedSources": client.SelectedSources,
			"volume":          client.Volume,
		})
	}

	stats := map[string]interface{}{
		"totalAudioClients":   len(audioClients),
		"totalBrowserClients": len(browserClients),
		"audioClients":        audioClientsList,
		"browserClients":      browserClientsList,
		"timestamp":           time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
