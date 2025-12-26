package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type WebRTCOffer struct {
	SDP string `json:"sdp"`
}

type WebRTCAnswer struct {
	SDP string `json:"sdp"`
}

var (
	webrtcClients   = make(map[string]*WebRTCClient)
	webrtcMutex     sync.RWMutex
	webrtcAudioChan = make(chan []byte, 100)
)

type WebRTCClient struct {
	pc         *webrtc.PeerConnection
	audioTrack *webrtc.TrackLocalStaticSample
	id         string
}

func createWebRTCPeerConnection(clientID string) (*WebRTCClient, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.cloudflare.com:3478",
					"stun:stun.cloudflare.com:53",
				},
			},
			{
				URLs: []string{
					"turn:turn.cloudflare.com:3478?transport=udp",
					"turn:turn.cloudflare.com:3478?transport=tcp",
					"turns:turn.cloudflare.com:5349?transport=tcp",
					"turn:turn.cloudflare.com:53?transport=udp",
					"turn:turn.cloudflare.com:80?transport=tcp",
					"turns:turn.cloudflare.com:443?transport=tcp",
				},
				Username:   "g0e799cc6bc8b752d76c2d58ffe38357b6ca35e440c7df8c8bf9d1ad286192d7",
				Credential: "e7c118593b988f9f79086ae45c73593f1305f7cb4f38f2dc24290caed09b95a3",
			},
		},
	}

	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, err
	}

	// Create audio track
	audioTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypePCMU},
		"audio",
		"microphone",
	)
	if err != nil {
		pc.Close()
		return nil, err
	}

	// Add track to peer connection
	if _, err := pc.AddTrack(audioTrack); err != nil {
		pc.Close()
		return nil, err
	}

	client := &WebRTCClient{
		pc:         pc,
		audioTrack: audioTrack,
		id:         clientID,
	}

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("WebRTC client %s connection state: %s", clientID, state.String())
		if state == webrtc.PeerConnectionStateClosed || state == webrtc.PeerConnectionStateFailed {
			webrtcMutex.Lock()
			delete(webrtcClients, clientID)
			webrtcMutex.Unlock()
		}
	})

	return client, nil
}

func handleWebRTCOffer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-ID")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var offer WebRTCOffer
	if err := json.NewDecoder(r.Body).Decode(&offer); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	clientID := r.Header.Get("X-Client-ID")
	if clientID == "" {
		clientID = fmt.Sprintf("client_%d", time.Now().UnixNano())
	}

	client, err := createWebRTCPeerConnection(clientID)
	if err != nil {
		log.Printf("Failed to create WebRTC peer connection: %v", err)
		http.Error(w, "Failed to create peer connection", http.StatusInternalServerError)
		return
	}

	// Register client
	webrtcMutex.Lock()
	webrtcClients[clientID] = client
	webrtcMutex.Unlock()

	log.Printf("WebRTC client %s connected, total: %d", clientID, len(webrtcClients))

	// Parse and set remote description
	sdp := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  offer.SDP,
	}

	if err := client.pc.SetRemoteDescription(sdp); err != nil {
		log.Printf("SetRemoteDescription failed: %v", err)
		http.Error(w, "Failed to set remote description", http.StatusInternalServerError)
		return
	}

	// Create answer
	answer, err := client.pc.CreateAnswer(nil)
	if err != nil {
		log.Printf("CreateAnswer failed: %v", err)
		http.Error(w, "Failed to create answer", http.StatusInternalServerError)
		return
	}

	if err := client.pc.SetLocalDescription(answer); err != nil {
		log.Printf("SetLocalDescription failed: %v", err)
		http.Error(w, "Failed to set local description", http.StatusInternalServerError)
		return
	}

	// Wait for ICE gathering to complete
	<-webrtc.GatheringCompletePromise(client.pc)

	response := WebRTCAnswer{
		SDP: client.pc.LocalDescription().SDP,
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Client-ID", clientID)
	_ = json.NewEncoder(w).Encode(response)
}

func broadcastWebRTCAudio(audioData []byte) {
	webrtcMutex.RLock()
	defer webrtcMutex.RUnlock()

	if len(webrtcClients) == 0 {
		return
	}

	sample := media.Sample{
		Data:     audioData,
		Duration: time.Millisecond * 20,
	}

	for _, client := range webrtcClients {
		if err := client.audioTrack.WriteSample(sample); err != nil {
			log.Printf("Failed to write WebRTC audio sample: %v", err)
		}
	}
}

func startWebRTCServer() {
	http.HandleFunc("/webrtc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, webrtcHtmlPage)
	})

	http.HandleFunc("/webrtc/offer", handleWebRTCOffer)

	// Start WebRTC audio streaming goroutine
	go func() {
		for audioData := range webrtcAudioChan {
			broadcastWebRTCAudio(audioData)
		}
	}()

	log.Println("WebRTC P2P server available at :8080/webrtc")
}

const webrtcHtmlPage = `
<!DOCTYPE html>
<html>
<head>
    <title>WebRTC P2P Audio Stream</title>
</head>
<body>
    <h1>WebRTC P2P Audio Stream (No WebSocket)</h1>
    <button id="start-webrtc">Start P2P Connection</button>
    <button id="stop-webrtc">Stop P2P Connection</button>
    <div id="webrtc-status">Ready for P2P</div>
    <audio id="webrtc-audio" controls autoplay></audio>

    <script>
        const startBtn = document.getElementById('start-webrtc');
        const stopBtn = document.getElementById('stop-webrtc');
        const status = document.getElementById('webrtc-status');
        const audio = document.getElementById('webrtc-audio');
        let pc = null;
        let clientId = null;

        function setStatus(msg) {
            status.textContent = msg;
            console.log('WebRTC:', msg);
        }

        startBtn.addEventListener('click', async () => {
            try {
                setStatus('Creating P2P connection...');

                clientId = 'client_' + Date.now();

                pc = new RTCPeerConnection({
                    iceServers: [
                        {
                            urls: [
                                "stun:stun.cloudflare.com:3478",
                                "stun:stun.cloudflare.com:53"
                            ]
                        },
                        {
                            urls: [
                                "turn:turn.cloudflare.com:3478?transport=udp",
                                "turn:turn.cloudflare.com:3478?transport=tcp",
                                "turns:turn.cloudflare.com:5349?transport=tcp",
                                "turn:turn.cloudflare.com:53?transport=udp",
                                "turn:turn.cloudflare.com:80?transport=tcp",
                                "turns:turn.cloudflare.com:443?transport=tcp"
                            ],
                            username: "g0e799cc6bc8b752d76c2d58ffe38357b6ca35e440c7df8c8bf9d1ad286192d7",
                            credential: "e7c118593b988f9f79086ae45c73593f1305f7cb4f38f2dc24290caed09b95a3"
                        }
                    ],
                    iceCandidatePoolSize: 10
                });

                pc.ontrack = (event) => {
                    setStatus('P2P audio stream received');
                    audio.srcObject = event.streams[0];
                };

                pc.onconnectionstatechange = () => {
                    setStatus('P2P Connection: ' + pc.connectionState);
                    if (pc.connectionState === 'failed') {
                        setStatus('Connection failed - retrying...');
                        // Auto retry on failure
                        setTimeout(() => {
                            if (pc && pc.connectionState === 'failed') {
                                pc.restartIce();
                            }
                        }, 2000);
                    }
                };

                pc.oniceconnectionstatechange = () => {
                    console.log('ICE connection state:', pc.iceConnectionState);
                };

                pc.onicegatheringstatechange = () => {
                    console.log('ICE gathering state:', pc.iceGatheringState);
                };

                // Create offer and wait for ICE gathering
                const offer = await pc.createOffer({
                    offerToReceiveAudio: true
                });

                await pc.setLocalDescription(offer);
                setStatus('Gathering ICE candidates...');

                // Wait for ICE gathering to complete
                await new Promise((resolve) => {
                    if (pc.iceGatheringState === 'complete') {
                        resolve();
                    } else {
                        pc.addEventListener('icegatheringstatechange', () => {
                            if (pc.iceGatheringState === 'complete') {
                                resolve();
                            }
                        });
                    }
                });

                setStatus('Sending offer to server...');

                // Send offer to server via HTTP
                const response = await fetch('/webrtc/offer', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'X-Client-ID': clientId
                    },
                    body: JSON.stringify({
                        sdp: pc.localDescription.sdp
                    })
                });

                if (!response.ok) {
                    throw new Error('Failed to send offer');
                }

                const answerData = await response.json();

                const answer = {
                    type: 'answer',
                    sdp: answerData.sdp
                };

                await pc.setRemoteDescription(answer);
                setStatus('Waiting for connection...');

                startBtn.disabled = true;
                stopBtn.disabled = false;
            } catch (err) {
                setStatus('P2P Error: ' + err.message);
                console.error('WebRTC error:', err);
            }
        });

        stopBtn.addEventListener('click', () => {
            if (pc) {
                pc.close();
                pc = null;
            }
            if (audio.srcObject) {
                audio.srcObject = null;
            }
            startBtn.disabled = false;
            stopBtn.disabled = true;
            setStatus('P2P stopped');
        });

        stopBtn.disabled = true;
    </script>
</body>
</html>
`
