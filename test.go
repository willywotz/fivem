package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/pion/webrtc/v4"
)

func test() {
	_ = godotenv.Load(".env")

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM)
	wait := func() { <-signalCh }

	ctx := context.Background()

	cmd := &GetICEServersCommand{
		TURN_KEY_ID:        os.Getenv("TURN_KEY_ID"),
		TURN_KEY_API_TOKEN: os.Getenv("TURN_KEY_API_TOKEN"),
		TTL:                86400, // 24 hours
	}

	iceServers, err := GetICEServers(ctx, cmd)
	if err != nil {
		fmt.Printf("Error fetching ICE servers: %v\n", err)
		return
	}

	peerConnection, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: iceServers,
	})
	if err != nil {
		fmt.Printf("Error creating PeerConnection: %v\n", err)
		return
	}
	defer peerConnection.Close()

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		fmt.Printf("New ICE candidate: %s\n", candidate.ToJSON().Candidate)
	})

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("PeerConnection state changed: %s\n", state.String())
	})

	wait()
}

type GetICEServersCommand struct {
	TURN_KEY_ID        string `json:"turn_key_id"`
	TURN_KEY_API_TOKEN string `json:"turn_key_api_token"`
	TTL                int    `json:"ttl"` // Time to live for the credentials in seconds
}

// getTurnCredentials fetches TURN credentials from Cloudflare
func GetICEServers(ctx context.Context, cmd *GetICEServersCommand) ([]webrtc.ICEServer, error) {
	if cmd.TURN_KEY_ID == "" || cmd.TURN_KEY_API_TOKEN == "" {
		return nil, fmt.Errorf("TURN_KEY_ID and TURN_KEY_API_TOKEN must be provided")
	}

	if cmd.TTL <= 0 {
		cmd.TTL = 86400 // Default to 24 hours if TTL is not provided
	}

	data := map[string]interface{}{
		"ttl": cmd.TTL,
	}

	jsonData, _ := json.Marshal(data)

	url := fmt.Sprintf("https://rtc.live.cloudflare.com/v1/turn/keys/%s/credentials/generate-ice-servers", cmd.TURN_KEY_ID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cmd.TURN_KEY_API_TOKEN))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ICE servers: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("failed to fetch ICE servers: received status code %d", resp.StatusCode)
	}

	var v struct {
		ICEServers []webrtc.ICEServer `json:"iceServers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("failed to decode ICE servers response: %w", err)
	}

	return v.ICEServers, nil
}
