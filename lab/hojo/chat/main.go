package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/pion/webrtc/v4"
)

type Peer struct {
	conn      *webrtc.PeerConnection
	data      *webrtc.DataChannel
	signalOut chan []byte
}

var (
	peersMu sync.Mutex
	peers   = map[string]*Peer{}
)

func main() {
	http.HandleFunc("/signal", signalHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, html)
	})
	log.Println("Open http://localhost:8081 in multiple tabs")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func broadcast(senderID string, msg string) {
	peersMu.Lock()
	defer peersMu.Unlock()
	for id, p := range peers {
		if id == senderID || p.data == nil {
			continue
		}
		_ = p.data.SendText(msg)
	}
}

func signalHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		// Receive offer/answer/candidate from client
		var req struct {
			ID        string                     `json:"id"`
			SDP       *webrtc.SessionDescription `json:"sdp,omitempty"`
			Candidate *webrtc.ICECandidateInit   `json:"candidate,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		defer r.Body.Close()

		peersMu.Lock()
		p, exists := peers[req.ID]
		peersMu.Unlock()

		if !exists {
			// New peer, create connection
			peerConn, err := webrtc.NewPeerConnection(webrtc.Configuration{})
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			signalCh := make(chan []byte, 8)
			p = &Peer{conn: peerConn, signalOut: signalCh}
			peersMu.Lock()
			peers[req.ID] = p
			peersMu.Unlock()

			// Data channel handler
			peerConn.OnDataChannel(func(dc *webrtc.DataChannel) {
				p.data = dc
				dc.OnMessage(func(msg webrtc.DataChannelMessage) {
					broadcast(req.ID, string(msg.Data))
				})
			})

			// ICE candidate handler
			peerConn.OnICECandidate(func(c *webrtc.ICECandidate) {
				if c == nil {
					return
				}
				b, _ := json.Marshal(map[string]interface{}{
					"candidate": c.ToJSON(),
				})
				signalCh <- b
			})
		}

		if req.SDP != nil {
			switch req.SDP.Type {
			case webrtc.SDPTypeOffer:
				if err := p.conn.SetRemoteDescription(*req.SDP); err != nil {
					http.Error(w, err.Error(), 400)
					return
				}
				dc, err := p.conn.CreateDataChannel("chat", nil)
				if err == nil {
					p.data = dc
					dc.OnMessage(func(msg webrtc.DataChannelMessage) {
						broadcast(req.ID, string(msg.Data))
					})
				}
				answer, err := p.conn.CreateAnswer(nil)
				if err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				if err := p.conn.SetLocalDescription(answer); err != nil {
					http.Error(w, err.Error(), 500)
					return
				}
				resp, _ := json.Marshal(map[string]interface{}{
					"sdp": p.conn.LocalDescription(),
				})
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(resp)
				return
			case webrtc.SDPTypeAnswer:
				_ = p.conn.SetRemoteDescription(*req.SDP)
			}
		}
		if req.Candidate != nil {
			_ = p.conn.AddICECandidate(*req.Candidate)
		}
		_, _ = w.Write([]byte("{}"))
	case "GET":
		// Long-poll for new ICE candidates from server to client
		id := r.URL.Query().Get("id")
		peersMu.Lock()
		p, exists := peers[id]
		peersMu.Unlock()
		if !exists {
			http.Error(w, "peer not found", 404)
			return
		}
		_, _ = w.Write(<-p.signalOut)
	default:
		w.WriteHeader(405)
	}
}

const html = `
<!doctype html>
<html>
<body>
  <h2>Multi-User WebRTC Chat Room (pion/webrtc, single file demo)</h2>
  <pre id="log"></pre>
  <input id="msg" placeholder="Type message...">
  <button onclick="send()">Send</button>
<script>
let pc, dc, id = Math.random().toString(36).substr(2,8), logEl = document.getElementById('log')
function log(msg) { logEl.textContent += msg + '\n' }
async function start() {
  pc = new RTCPeerConnection()
  dc = pc.createDataChannel('chat')
  dc.onopen = () => log('[system] connected!')
  dc.onmessage = e => log(e.data)
  pc.onicecandidate = e => {
    if (e.candidate)
      fetch('/signal', {method:"POST",body:JSON.stringify({id, candidate:e.candidate})})
  }
  let offer = await pc.createOffer()
  await pc.setLocalDescription(offer)
  let r = await fetch('/signal',{method:"POST",body:JSON.stringify({id,sdp:pc.localDescription})})
  let resp = await r.json()
  await pc.setRemoteDescription(resp.sdp)
  // Poll for ICE from server
  ;(function poll(){
    fetch('/signal?id='+id).then(r=>r.json().then(d=>{
      if (d.candidate)
        pc.addIceCandidate(d.candidate)
      setTimeout(poll,100)
    }))
  })()
}
start()
function send() {
  let val = msg.value
  dc.send(val)
  log("[me] "+val)
  msg.value = ""
}
</script>
</body>
</html>
`
