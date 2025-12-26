package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	clients = make(map[*websocket.Conn]bool)
	mutex   sync.RWMutex
)

type AudioMessage struct {
	Type          string `json:"type"`
	AudioData     string `json:"audioData,omitempty"`
	SampleRate    uint32 `json:"sampleRate,omitempty"`
	Channels      uint16 `json:"channels,omitempty"`
	Format        string `json:"format,omitempty"`
	BitsPerSample uint16 `json:"bitsPerSample,omitempty"`
}

type AudioMessageRaw struct {
	Type          string `json:"type"`
	AudioData     []byte `json:"audioData"`
	SampleRate    uint32 `json:"sampleRate"`
	Channels      uint16 `json:"channels"`
	Format        string `json:"format"`
	BitsPerSample uint16 `json:"bitsPerSample"`
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

	// Broadcast to WebSocket clients
	broadcastAudio(audioMsg.AudioData, audioMsg.SampleRate, audioMsg.Channels, audioMsg.BitsPerSample)

	w.WriteHeader(http.StatusOK)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Register client
	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	// Unregister client on disconnect
	defer func() {
		mutex.Lock()
		delete(clients, conn)
		mutex.Unlock()
	}()

	log.Printf("Browser client connected, total clients: %d", len(clients))

	// Keep connection alive and handle client messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}
}

func broadcastAudio(audioData []byte, sampleRate uint32, channels uint16, bitsPerSample uint16) {
	mutex.RLock()
	defer mutex.RUnlock()

	if len(clients) == 0 {
		return
	}

	msg := AudioMessage{
		Type:          "audio",
		AudioData:     base64.StdEncoding.EncodeToString(audioData),
		SampleRate:    sampleRate,
		Channels:      channels,
		Format:        "pcm",
		BitsPerSample: bitsPerSample,
	}

	for client := range clients {
		if err := client.WriteJSON(msg); err != nil {
			log.Printf("Failed to send audio data: %v", err)
			client.Close()
			delete(clients, client)
		}
	}
}

func main() {
	// Handle audio data from capture client
	http.HandleFunc("/audio", handleAudioData)

	// Handle WebSocket connections from browsers
	http.HandleFunc("/ws", handleWebSocket)

	// Serve HTML page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, htmlPage)
	})

	log.Println("Audio server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

const htmlPage = `
<!DOCTYPE html>
<html>
<head>
    <title>Audio Stream</title>
</head>
<body>
    <h1>Audio Stream Server</h1>
    <button id="start">Start Audio</button>
    <button id="stop">Stop Audio</button>
    <div id="status">Ready</div>
    <div>
        <label>Volume: <input type="range" id="volume" min="0" max="100" value="50"></label>
    </div>
    <div id="audioInfo">Audio format info will appear here</div>

    <script>
        const startBtn = document.getElementById('start');
        const stopBtn = document.getElementById('stop');
        const status = document.getElementById('status');
        const audioInfo = document.getElementById('audioInfo');
        const volumeSlider = document.getElementById('volume');
        let ws = null;
        let audioContext = null;
        let gainNode = null;
        let audioBuffer = [];
        let nextPlayTime = 0;

        function setStatus(msg) {
            status.textContent = msg;
            console.log(msg);
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

        function scheduleAudioPlayback() {
            while (audioBuffer.length > 0) {
                const audioData = audioBuffer.shift();
                const source = audioContext.createBufferSource();
                source.buffer = audioData.buffer;
                source.connect(gainNode);
                gainNode.connect(audioContext.destination);

                if (nextPlayTime < audioContext.currentTime) {
                    nextPlayTime = audioContext.currentTime;
                }

                source.start(nextPlayTime);
                nextPlayTime += audioData.buffer.duration;
            }
        }

        function playAudioData(audioData, sampleRate, channels, bitsPerSample) {
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

                audioBuffer.push({ buffer: webAudioBuffer });

                if (audioBuffer.length > 10) {
                    audioBuffer.shift();
                }

                scheduleAudioPlayback();
            } catch (error) {
                console.error('Error playing audio:', error);
            }
        }

        volumeSlider.addEventListener('input', (e) => {
            if (gainNode) {
                gainNode.gain.value = e.target.value / 100;
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
                    setStatus('Connected - receiving audio stream');
                };

                ws.onmessage = (event) => {
                    try {
                        const message = JSON.parse(event.data);
                        if (message.type === 'audio') {
                            audioInfo.textContent = "Format: " + message.sampleRate + "Hz, " + message.channels + "ch, " + message.bitsPerSample + "bit";
                            playAudioData(message.audioData, message.sampleRate, message.channels, message.bitsPerSample);
                        }
                    } catch (err) {
                        console.error('Error handling message:', err);
                    }
                };

                ws.onerror = (err) => {
                    setStatus('WebSocket error');
                    console.error('WebSocket error:', err);
                };

                ws.onclose = () => {
                    setStatus('Disconnected');
                };

                startBtn.disabled = true;
                stopBtn.disabled = false;
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
            startBtn.disabled = false;
            stopBtn.disabled = true;
            setStatus('Stopped');
        });

        stopBtn.disabled = true;
    </script>
</body>
</html>
`
