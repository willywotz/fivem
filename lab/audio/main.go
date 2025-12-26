package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

type AudioMessage struct {
	Type          string `json:"type"`
	AudioData     []byte `json:"audioData"`
	SampleRate    uint32 `json:"sampleRate"`
	Channels      uint16 `json:"channels"`
	Format        string `json:"format"`
	BitsPerSample uint16 `json:"bitsPerSample"`
}

type AudioClient struct {
	micClient     *wca.IAudioClient
	captureClient *wca.IAudioCaptureClient
	micFormat     *wca.WAVEFORMATEX
	serverURL     string
}

func NewAudioClient(serverURL string) (*AudioClient, error) {
	// Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return nil, fmt.Errorf("CoInitializeEx failed: %v", err)
	}

	client := &AudioClient{serverURL: serverURL}

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
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal audio message: %v", err)
	}

	resp, err := http.Post(c.serverURL+"/audio", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send audio data: %v", err)
	}
	defer resp.Body.Close()

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
	serverURL := "http://localhost:8080"
	if len(os.Args) > 1 {
		serverURL = os.Args[1]
	}

	log.Printf("Starting audio client, connecting to server: %s", serverURL)

	// Initialize audio client
	audioClient, err := NewAudioClient(serverURL)
	if err != nil {
		log.Fatal("Failed to create audio client:", err)
	}
	defer audioClient.Close()

	// Initialize audio capture
	if err := audioClient.Initialize(); err != nil {
		log.Fatal("Failed to initialize audio client:", err)
	}

	// Start audio capture loop
	audioClient.StartCapture()
}
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

                // Initialize audio context
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

func main() {
	// Start web server in goroutine
	go startWebServer()

	// 1. Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		log.Fatal("CoInitializeEx failed:", err)
	}
	defer ole.CoUninitialize()

	// 2. Get IMMDeviceEnumerator
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		log.Fatal("CoCreateInstance failed:", err)
	}
	defer enumerator.Release()

	// 3. Get default input (microphone) device
	var mic *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mic); err != nil {
		log.Fatal("GetDefaultAudioEndpoint (input) failed:", err)
	}
	defer mic.Release()

	// 4. Get default output device
	var speaker *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &speaker); err != nil {
		log.Fatal("GetDefaultAudioEndpoint (output) failed:", err)
	}
	defer speaker.Release()

	// 5. Activate IAudioClient for both devices
	var micClient *wca.IAudioClient
	if err := mic.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&micClient,
	); err != nil {
		log.Fatal("Activate (mic) failed:", err)
	}
	defer micClient.Release()

	var speakerClient *wca.IAudioClient
	if err := speaker.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&speakerClient,
	); err != nil {
		log.Fatal("Activate (speaker) failed:", err)
	}
	defer speakerClient.Release()

	// 6. Get mix format (shared mode, must match for zero-copy)
	var micFormat *wca.WAVEFORMATEX
	if err := micClient.GetMixFormat(&micFormat); err != nil {
		log.Fatal("mic GetMixFormat failed:", err)
	}
	var speakerFormat *wca.WAVEFORMATEX
	if err := speakerClient.GetMixFormat(&speakerFormat); err != nil {
		log.Fatal("speaker GetMixFormat failed:", err)
	}
	// For zerocopy: ideally micFormat == speakerFormat. If not, you must convert.
	// Check if formats match, if not we'll need to convert
	formatsMismatch := micFormat.NChannels != speakerFormat.NChannels ||
		micFormat.NSamplesPerSec != speakerFormat.NSamplesPerSec ||
		micFormat.WBitsPerSample != speakerFormat.WBitsPerSample

	if formatsMismatch {
		fmt.Printf("Format mismatch detected:\n")
		fmt.Printf("Mic: %d channels, %d Hz, %d bits\n", micFormat.NChannels, micFormat.NSamplesPerSec, micFormat.WBitsPerSample)
		fmt.Printf("Speaker: %d channels, %d Hz, %d bits\n", speakerFormat.NChannels, speakerFormat.NSamplesPerSec, speakerFormat.WBitsPerSample)
		fmt.Printf("Auto-conversion enabled\n")
	}

	// Simple conversion function
	convertAudio := func(srcData []byte, srcFormat, dstFormat *wca.WAVEFORMATEX) []byte {
		if !formatsMismatch {
			return srcData
		}

		srcBytesPerSample := int(srcFormat.WBitsPerSample / 8)
		dstBytesPerSample := int(dstFormat.WBitsPerSample / 8)
		srcFrames := len(srcData) / (int(srcFormat.NChannels) * srcBytesPerSample)

		dstData := make([]byte, srcFrames*int(dstFormat.NChannels)*dstBytesPerSample)

		for frame := 0; frame < srcFrames; frame++ {
			// Simple channel conversion (mono/stereo)
			if srcFormat.NChannels == 1 && dstFormat.NChannels == 2 {
				// Mono to stereo: duplicate channel
				srcOffset := frame * srcBytesPerSample
				dstOffset := frame * 2 * dstBytesPerSample
				copy(dstData[dstOffset:dstOffset+dstBytesPerSample], srcData[srcOffset:srcOffset+srcBytesPerSample])
				copy(dstData[dstOffset+dstBytesPerSample:dstOffset+2*dstBytesPerSample], srcData[srcOffset:srcOffset+srcBytesPerSample])
			} else if srcFormat.NChannels == 2 && dstFormat.NChannels == 1 {
				// Stereo to mono: mix channels
				srcOffset := frame * 2 * srcBytesPerSample
				dstOffset := frame * dstBytesPerSample
				if srcBytesPerSample == 2 { // 16-bit
					left := int16(srcData[srcOffset]) | int16(srcData[srcOffset+1])<<8
					right := int16(srcData[srcOffset+2]) | int16(srcData[srcOffset+3])<<8
					mixed := (left + right) / 2
					dstData[dstOffset] = byte(mixed)
					dstData[dstOffset+1] = byte(mixed >> 8)
				}
			} else {
				// Same channel count, just copy
				srcOffset := frame * int(srcFormat.NChannels) * srcBytesPerSample
				dstOffset := frame * int(dstFormat.NChannels) * dstBytesPerSample
				copy(dstData[dstOffset:dstOffset+int(dstFormat.NChannels)*dstBytesPerSample],
					srcData[srcOffset:srcOffset+int(srcFormat.NChannels)*srcBytesPerSample])
			}
		}
		return dstData
	}

	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	var latency time.Duration
	if err := speakerClient.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		log.Fatal("GetDevicePeriod failed:", err)
		return
	}
	latency = time.Duration(int(defaultPeriod) * 100)
	_ = latency // Use latency for logging or debugging

	// fmt.Println("Default period: ", defaultPeriod)
	// fmt.Println("Minimum period: ", minimumPeriod)
	// fmt.Println("Latency: ", latency)

	// 7. Initialize audio clients (shared mode, event driven)
	if err := micClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		micFormat,
		nil,
	); err != nil {
		log.Fatal("micClient Initialize failed:", err)
	}
	if err := speakerClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		speakerFormat,
		nil,
	); err != nil {
		log.Fatal("speakerClient Initialize failed:", err)
	}

	fakeAudioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	defer func() { _ = wca.CloseHandle(fakeAudioReadyEvent) }()

	if err := micClient.SetEventHandle(fakeAudioReadyEvent); err != nil {
		log.Fatal("micClient SetEventHandle failed:", err)
		return
	}

	audioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	defer func() { _ = wca.CloseHandle(audioReadyEvent) }()

	if err := speakerClient.SetEventHandle(audioReadyEvent); err != nil {
		log.Fatal("speakerClient SetEventHandle failed:", err)
		return
	}

	// 8. Get IAudioCaptureClient and IAudioRenderClient
	var captureClient *wca.IAudioCaptureClient
	if err := micClient.GetService(wca.IID_IAudioCaptureClient, &captureClient); err != nil {
		log.Fatal("mic GetService failed:", err)
	}
	defer captureClient.Release()

	var renderClient *wca.IAudioRenderClient
	if err := speakerClient.GetService(wca.IID_IAudioRenderClient, &renderClient); err != nil {
		log.Fatal("speaker GetService failed:", err)
	}
	defer renderClient.Release()

	// 9. Start audio clients
	if err := micClient.Start(); err != nil {
		log.Fatal("micClient Start failed:", err)
	}
	if err := speakerClient.Start(); err != nil {
		log.Fatal("speakerClient Start failed:", err)
	}

	// 10. Main loop: read from mic, write to speaker and stream
	bufferAccumulator := make([]byte, 0, 4096)                                             // Accumulate audio data
	targetBufferSize := int(micFormat.NSamplesPerSec * uint32(micFormat.NBlockAlign) / 50) // ~20ms of audio

	for {
		var packetLength uint32
		if err := captureClient.GetNextPacketSize(&packetLength); err != nil {
			log.Fatal("GetNextPacketSize failed:", err)
		}
		if packetLength == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		var data *byte
		var numFrames uint32
		var flags uint32
		if err := captureClient.GetBuffer(&data, &numFrames, &flags, nil, nil); err != nil {
			log.Fatal("GetBuffer failed:", err)
		}

		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
			srcData := unsafe.Slice(data, numFrames*uint32(micFormat.NBlockAlign))
			convertedData := convertAudio(srcData, micFormat, speakerFormat)

			// Accumulate audio data for smoother streaming
			bufferAccumulator = append(bufferAccumulator, srcData...)

			// Send larger chunks to reduce choppy audio
			if len(bufferAccumulator) >= targetBufferSize {
				broadcastAudio(bufferAccumulator, micFormat.NSamplesPerSec, micFormat.NChannels, micFormat.WBitsPerSample)
				bufferAccumulator = bufferAccumulator[:0] // Reset buffer
			}

			// Calculate frames for output
			outputFrames := uint32(len(convertedData)) / uint32(speakerFormat.NBlockAlign)

			var renderData *byte
			if err := renderClient.GetBuffer(outputFrames, &renderData); err != nil {
				log.Fatal("renderClient GetBuffer failed:", err)
			}

			copy(
				unsafe.Slice(renderData, uint32(len(convertedData))),
				convertedData,
			)

			if err := renderClient.ReleaseBuffer(outputFrames, 0); err != nil {
				log.Fatal("renderClient ReleaseBuffer failed:", err)
			}
		}

		if err := captureClient.ReleaseBuffer(numFrames); err != nil {
			log.Fatal("captureClient ReleaseBuffer failed:", err)
		}
	}
}
