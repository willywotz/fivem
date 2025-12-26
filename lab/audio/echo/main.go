package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

func main() {
	fmt.Println(run())
}

func run() error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("CoInitializeEx failed: %v", err)
	}
	defer ole.CoUninitialize()

	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		return fmt.Errorf("CoCreateInstance failed: %v", err)
	}
	defer enumerator.Release()

	var mic *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mic); err != nil {
		return fmt.Errorf("GetDefaultAudioEndpoint (input) failed: %v", err)
	}
	defer mic.Release()

	var micClient *wca.IAudioClient
	if err := mic.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&micClient,
	); err != nil {
		return fmt.Errorf("Activate (mic) failed: %v", err)
	}
	defer micClient.Release()

	var micFormat *wca.WAVEFORMATEX
	if err := micClient.GetMixFormat(&micFormat); err != nil {
		return fmt.Errorf("mic GetMixFormat failed: %v", err)
	}

	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	if err := micClient.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		return fmt.Errorf("GetDevicePeriod failed: %v", err)
	}

	if err := micClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		micFormat,
		nil,
	); err != nil {
		return fmt.Errorf("micClient Initialize failed: %v", err)
	}

	fakeAudioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	if err := micClient.SetEventHandle(fakeAudioReadyEvent); err != nil {
		return fmt.Errorf("micClient SetEventHandle failed: %v", err)
	}

	var captureClient *wca.IAudioCaptureClient
	if err := micClient.GetService(wca.IID_IAudioCaptureClient, &captureClient); err != nil {
		return fmt.Errorf("mic GetService failed: %v", err)
	}
	defer captureClient.Release()

	if err := micClient.Start(); err != nil {
		return fmt.Errorf("micClient Start failed: %v", err)
	}

	bufferAccumulator := make([]byte, 0, 4096)
	targetBufferSize := int(micFormat.NSamplesPerSec * uint32(micFormat.NBlockAlign) / 50) // ~20ms of audio

	log.Printf("Starting audio capture - Format: %d Hz, %d channels, %d bits",
		micFormat.NSamplesPerSec, micFormat.NChannels, micFormat.WBitsPerSample)

	audioCh := make(chan []byte, 128)

	http.HandleFunc("/audio", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/raw")
		w.Header().Set("Transfer-Encoding", "chunked")

		for audioData := range audioCh {
			if _, err := w.Write(audioData); err != nil {
				log.Printf("Failed to write audio data: %v", err)
				return
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`
<button onclick="fetchAudio()">Start Audio</button>
<script>
let audioCtx = new (window.AudioContext || window.webkitAudioContext)({sampleRate: 48000});
let source;
let bufferQueue = [];
let playing = false;

async function fetchAudio() {
    const response = await fetch('/audio');
    const reader = response.body.getReader();

    while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        if (value) bufferQueue.push(value.buffer);
        if (!playing) playQueue();
    }
}

function playQueue() {
    if (bufferQueue.length === 0) {
        playing = false;
        return;
    }
    playing = true;
    let raw = bufferQueue.shift();
	if (raw.byteLength % 2 !== 0) {
        raw = raw.slice(0, raw.byteLength - (raw.byteLength % 2));
    }
    let samples = new Int16Array(raw);
    let floatSamples = new Float32Array(samples.length);
    for (let i = 0; i < samples.length; i++) {
        floatSamples[i] = samples[i] / 32768;
    }
    let audioBuffer = audioCtx.createBuffer(1, floatSamples.length, 48000);
    audioBuffer.getChannelData(0).set(floatSamples);
    source = audioCtx.createBufferSource();
    source.buffer = audioBuffer;
    source.connect(audioCtx.destination);
    source.onended = playQueue;
    source.start();
}
</script>
		`))
	})

	go func() {
		log.Println(http.ListenAndServe(":8080", nil))
	}()

	// Network usage tracking
	var bytesSentThisSecond int
	lastLogTime := time.Now()

	// dir, _ := os.Getwd()
	// name := filepath.Join(dir, "ffmpeg.exe")
	// args := []string{
	// 	"-i", "pipe:0",
	// 	"-acodec", "pcm_f32le", "-ar", "48000", "-ac", "2",
	// 	"-c:a", "libvorbis", "-f", "nut",
	// 	"pipe:1",
	// }
	// cmd := exec.Command(name, args...)
	// reader, writer, _ := os.Pipe()
	// playReader, playWriter, _ := os.Pipe()
	// cmd.Stdin = reader
	// cmd.Stdout = playWriter
	// cmd.Stderr = os.Stderr

	// playCmd := exec.Command(filepath.Join(dir, "ffplay.exe"), "-f", "s16le", "-ar", "48000", "-ch_layout", "stereo", "-i", "pipe:0", "-autoexit")
	// playCmd := exec.Command(filepath.Join(dir, "ffplay.exe"), "-f", "f32le", "-i", "pipe:0")
	// playCmd.Stdin = playReader
	// playCmd.Stdout = os.Stdout
	// playCmd.Stderr = os.Stderr

	// _ = cmd.Start()
	// _ = playCmd.Start()

	for {
		var packetLength uint32
		if err := captureClient.GetNextPacketSize(&packetLength); err != nil {
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
		if err := captureClient.GetBuffer(&data, &numFrames, &flags, nil, nil); err != nil {
			log.Printf("GetBuffer failed: %v", err)
			continue
		}

		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
			srcData := unsafe.Slice(data, numFrames*uint32(micFormat.NBlockAlign))
			// _, _ = playWriter.Write(float32leToS16le(srcData))

			// Accumulate audio data
			bufferAccumulator = append(bufferAccumulator, srcData...)

			// Send larger chunks to reduce network overhead
			if len(bufferAccumulator) >= targetBufferSize {
				// dir, _ := os.Getwd()
				// name := filepath.Join(dir, "ffplay.exe")
				// args := []string{
				// 	"-i", "pipe:0",
				// 	"-f", "f32le",
				// 	"-autoexit",
				// }
				// cmd := exec.Command(name, args...)
				// cmd.Stdin = bytes.NewReader(bufferAccumulator)
				// cmd.Stderr = os.Stderr

				// if err := cmd.Run(); err != nil {
				// 	log.Printf("Failed to play audio: %v", err)
				// 	return err
				// }

				go func(data []byte, sampleRate uint32, channels uint16, bitsPerSample uint16) {
					if bitsPerSample == 32 {
						data = float32leToS16le(data) // Convert 32-bit to 16-bit
						// bitsPerSample = 16
					}

					// dir, _ := os.Getwd()
					// name := filepath.Join(dir, "opusenc.exe")
					// args := []string{
					// 	"--raw",
					// 	"--raw-bits", fmt.Sprintf("%d", bitsPerSample),
					// 	"--raw-rate", fmt.Sprintf("%d", sampleRate),
					// 	"--raw-chan", fmt.Sprintf("%d", channels),
					// 	"--raw-endianness", "0",
					// 	"-", "-",
					// }
					// cmd := exec.Command(name, args...)
					// cmd.Stdin = bytes.NewReader(data)
					// encData := bytes.NewBuffer(nil)
					// cmd.Stdout = encData
					// // cmd.Stderr = os.Stderr

					// if err := cmd.Run(); err != nil {
					// 	log.Printf("Failed to encode audio: %v", err)
					// 	return
					// }

					bytesSentThisSecond += len(data)
					audioCh <- data
				}(bufferAccumulator, micFormat.NSamplesPerSec, micFormat.NChannels, micFormat.WBitsPerSample)

				bufferAccumulator = bufferAccumulator[:0] // Reset buffer
			}
		}

		// Log network usage every second
		now := time.Now()
		if now.Sub(lastLogTime) >= time.Second {
			log.Printf("Network usage: %d bytes sent in the last second", bytesSentThisSecond)
			bytesSentThisSecond = 0
			lastLogTime = now
		}

		if err := captureClient.ReleaseBuffer(numFrames); err != nil {
			log.Printf("ReleaseBuffer failed: %v", err)
		}
	}
}

func float32leToS16le(in []byte) []byte {
	out := make([]byte, 0, len(in)/2)
	for i := 0; i+3 < len(in); i += 4 {
		// Read float32 little-endian
		bits := uint32(in[i]) | uint32(in[i+1])<<8 | uint32(in[i+2])<<16 | uint32(in[i+3])<<24
		f := *(*float32)(unsafe.Pointer(&bits))
		// Clamp and convert to int16
		sample := int16(0)
		if f > 1.0 {
			sample = 32767
		} else if f < -1.0 {
			sample = -32768
		} else {
			sample = int16(f * 32767)
		}
		// Write int16 little-endian
		out = append(out, byte(sample), byte(sample>>8))
	}
	return out
}
