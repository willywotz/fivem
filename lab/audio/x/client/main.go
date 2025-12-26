package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

var (
	serverURL  = flag.String("server", "http://localhost:8080", "Server URL")
	clientName = flag.String("name", "", "Client name (optional)")
)

type AudioMessage struct {
	Type          string `json:"type"`
	AudioData     []byte `json:"audioData"`
	SampleRate    uint32 `json:"sampleRate"`
	Channels      uint16 `json:"channels"`
	Format        string `json:"format"`
	BitsPerSample uint16 `json:"bitsPerSample"`
	ClientName    string `json:"clientName,omitempty"`
	MachineID     string `json:"machineId"`
}

type AudioClient struct {
	micClient     *wca.IAudioClient
	captureClient *wca.IAudioCaptureClient
	micFormat     *wca.WAVEFORMATEX
	serverURL     string
	httpClient    *http.Client
	machineID     string
	clientName    string
}

func generateMachineID() string {
	// Get machine-specific identifiers
	hostname, _ := os.Hostname()
	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}

	// Create a unique identifier based on hostname and username
	identifier := fmt.Sprintf("%s-%s-%d", hostname, username, os.Getpid())

	// Hash it to create a consistent machine ID
	hash := sha256.Sum256([]byte(identifier))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars
}

func NewAudioClient(serverURL string, clientName string) (*AudioClient, error) {
	// Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return nil, fmt.Errorf("CoInitializeEx failed: %v", err)
	}

	machineID := generateMachineID()
	if clientName == "" {
		hostname, _ := os.Hostname()
		clientName = fmt.Sprintf("Audio Client (%s)", hostname)
	}

	client := &AudioClient{
		serverURL:  serverURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		machineID:  machineID,
		clientName: clientName,
	}

	log.Printf("Generated Machine ID: %s", machineID)
	log.Printf("Client Name: %s", clientName)

	// Get IMMDeviceEnumerator
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return nil, fmt.Errorf("CoCreateInstance failed: %v", err)
	}
	defer enumerator.Release()

	// Get default input (microphone) device
	var mic *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mic); err != nil {
		return nil, fmt.Errorf("GetDefaultAudioEndpoint (input) failed: %v", err)
	}
	defer mic.Release()

	// Activate IAudioClient
	if err := mic.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&client.micClient,
	); err != nil {
		return nil, fmt.Errorf("Activate (mic) failed: %v", err)
	}

	// Get mix format
	if err := client.micClient.GetMixFormat(&client.micFormat); err != nil {
		return nil, fmt.Errorf("mic GetMixFormat failed: %v", err)
	}

	return client, nil
}

func (c *AudioClient) Initialize() error {
	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	if err := c.micClient.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		return fmt.Errorf("GetDevicePeriod failed: %v", err)
	}

	// Initialize audio client
	if err := c.micClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		c.micFormat,
		nil,
	); err != nil {
		return fmt.Errorf("micClient Initialize failed: %v", err)
	}

	// Create event handle
	fakeAudioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	if err := c.micClient.SetEventHandle(fakeAudioReadyEvent); err != nil {
		return fmt.Errorf("micClient SetEventHandle failed: %v", err)
	}

	// Get capture client
	if err := c.micClient.GetService(wca.IID_IAudioCaptureClient, &c.captureClient); err != nil {
		return fmt.Errorf("mic GetService failed: %v", err)
	}

	// Start audio client
	if err := c.micClient.Start(); err != nil {
		return fmt.Errorf("micClient Start failed: %v", err)
	}

	return nil
}

func (c *AudioClient) sendAudioToServer(audioData []byte) error {
	msg := AudioMessage{
		Type:          "audio",
		AudioData:     audioData,
		SampleRate:    c.micFormat.NSamplesPerSec,
		Channels:      c.micFormat.NChannels,
		Format:        "pcm",
		BitsPerSample: c.micFormat.WBitsPerSample,
		ClientName:    c.clientName,
		MachineID:     c.machineID,
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal audio message: %v", err)
	}

	resp, err := c.httpClient.Post(c.serverURL+"/audio", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send audio data: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status: %d", resp.StatusCode)
	}

	return nil
}

func (c *AudioClient) testServerConnection() error {
	resp, err := c.httpClient.Get(c.serverURL + "/health")
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server health check failed: %d", resp.StatusCode)
	}

	return nil
}

func (c *AudioClient) StartCapture() {
	bufferAccumulator := make([]byte, 0, 4096)
	targetBufferSize := int(c.micFormat.NSamplesPerSec * uint32(c.micFormat.NBlockAlign) / 50) // ~20ms of audio

	log.Printf("Starting audio capture - Format: %d Hz, %d channels, %d bits",
		c.micFormat.NSamplesPerSec, c.micFormat.NChannels, c.micFormat.WBitsPerSample)

	for {
		var packetLength uint32
		if err := c.captureClient.GetNextPacketSize(&packetLength); err != nil {
			log.Printf("GetNextPacketSize failed: %v", err)
			continue
		}
		if packetLength == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		var data *byte
		var numFrames uint32
		var flags uint32
		if err := c.captureClient.GetBuffer(&data, &numFrames, &flags, nil, nil); err != nil {
			log.Printf("GetBuffer failed: %v", err)
			continue
		}

		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
			srcData := unsafe.Slice(data, numFrames*uint32(c.micFormat.NBlockAlign))

			// Accumulate audio data
			bufferAccumulator = append(bufferAccumulator, srcData...)

			// Send larger chunks to reduce network overhead
			if len(bufferAccumulator) >= targetBufferSize {
				if err := c.sendAudioToServer(bufferAccumulator); err != nil {
					log.Printf("Failed to send audio to server: %v", err)
				}
				bufferAccumulator = bufferAccumulator[:0] // Reset buffer
			}
		}

		if err := c.captureClient.ReleaseBuffer(numFrames); err != nil {
			log.Printf("ReleaseBuffer failed: %v", err)
		}
	}
}

func (c *AudioClient) Close() {
	if c.captureClient != nil {
		c.captureClient.Release()
	}
	if c.micClient != nil {
		c.micClient.Release()
	}
	ole.CoUninitialize()
}

func main() {
	flag.Parse()

	serverURL := *serverURL
	clientName := *clientName

	// Check for environment variable overrides
	if envURL := os.Getenv("SERVER_URL"); envURL != "" {
		serverURL = envURL
	}
	if envName := os.Getenv("CLIENT_NAME"); envName != "" {
		clientName = envName
	}

	log.Printf("Starting audio client, connecting to server: %s", serverURL)
	if clientName != "" {
		log.Printf("Using client name: %s", clientName)
	}

	// Initialize audio client
	audioClient, err := NewAudioClient(serverURL, clientName)
	if err != nil {
		log.Fatal("Failed to create audio client:", err)
	}
	defer audioClient.Close()

	// Test server connection
	log.Printf("Testing server connection...")
	if err := audioClient.testServerConnection(); err != nil {
		log.Printf("Warning: Server connection test failed: %v", err)
		log.Printf("Continuing anyway, will retry on audio send failures...")
	} else {
		log.Printf("Server connection successful")
	}

	// Initialize audio capture
	if err := audioClient.Initialize(); err != nil {
		log.Fatal("Failed to initialize audio client:", err)
	}

	log.Printf("Audio client ready - Machine ID: %s", audioClient.machineID)

	// Start audio capture loop
	audioClient.StartCapture()
}
