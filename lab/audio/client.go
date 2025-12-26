package main

import (
	"fmt"
	"log"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

type AudioClient struct {
	micClient       *wca.IAudioClient
	speakerClient   *wca.IAudioClient
	captureClient   *wca.IAudioCaptureClient
	renderClient    *wca.IAudioRenderClient
	micFormat       *wca.WAVEFORMATEX
	speakerFormat   *wca.WAVEFORMATEX
	formatsMismatch bool
}

func NewAudioClient() (*AudioClient, error) {
	// Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return nil, fmt.Errorf("CoInitializeEx failed: %v", err)
	}

	client := &AudioClient{}

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

	// Get default output device
	var speaker *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &speaker); err != nil {
		return nil, fmt.Errorf("GetDefaultAudioEndpoint (output) failed: %v", err)
	}
	defer speaker.Release()

	// Activate IAudioClient for both devices
	if err := mic.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&client.micClient,
	); err != nil {
		return nil, fmt.Errorf("Activate (mic) failed: %v", err)
	}

	if err := speaker.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&client.speakerClient,
	); err != nil {
		return nil, fmt.Errorf("Activate (speaker) failed: %v", err)
	}

	// Get mix formats
	if err := client.micClient.GetMixFormat(&client.micFormat); err != nil {
		return nil, fmt.Errorf("mic GetMixFormat failed: %v", err)
	}
	if err := client.speakerClient.GetMixFormat(&client.speakerFormat); err != nil {
		return nil, fmt.Errorf("speaker GetMixFormat failed: %v", err)
	}

	// Check if formats match
	client.formatsMismatch = client.micFormat.NChannels != client.speakerFormat.NChannels ||
		client.micFormat.NSamplesPerSec != client.speakerFormat.NSamplesPerSec ||
		client.micFormat.WBitsPerSample != client.speakerFormat.WBitsPerSample

	if client.formatsMismatch {
		fmt.Printf("Format mismatch detected:\n")
		fmt.Printf("Mic: %d channels, %d Hz, %d bits\n", client.micFormat.NChannels, client.micFormat.NSamplesPerSec, client.micFormat.WBitsPerSample)
		fmt.Printf("Speaker: %d channels, %d Hz, %d bits\n", client.speakerFormat.NChannels, client.speakerFormat.NSamplesPerSec, client.speakerFormat.WBitsPerSample)
		fmt.Printf("Auto-conversion enabled\n")
	}

	return client, nil
}

func (c *AudioClient) Initialize() error {
	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	if err := c.speakerClient.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		return fmt.Errorf("GetDevicePeriod failed: %v", err)
	}

	// Initialize audio clients (shared mode, event driven)
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

	if err := c.speakerClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		c.speakerFormat,
		nil,
	); err != nil {
		return fmt.Errorf("speakerClient Initialize failed: %v", err)
	}

	// Create event handles
	fakeAudioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	if err := c.micClient.SetEventHandle(fakeAudioReadyEvent); err != nil {
		return fmt.Errorf("micClient SetEventHandle failed: %v", err)
	}

	audioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	if err := c.speakerClient.SetEventHandle(audioReadyEvent); err != nil {
		return fmt.Errorf("speakerClient SetEventHandle failed: %v", err)
	}

	// Get service interfaces
	if err := c.micClient.GetService(wca.IID_IAudioCaptureClient, &c.captureClient); err != nil {
		return fmt.Errorf("mic GetService failed: %v", err)
	}

	if err := c.speakerClient.GetService(wca.IID_IAudioRenderClient, &c.renderClient); err != nil {
		return fmt.Errorf("speaker GetService failed: %v", err)
	}

	// Start audio clients
	if err := c.micClient.Start(); err != nil {
		return fmt.Errorf("micClient Start failed: %v", err)
	}
	if err := c.speakerClient.Start(); err != nil {
		return fmt.Errorf("speakerClient Start failed: %v", err)
	}

	return nil
}

func (c *AudioClient) convertAudio(srcData []byte, srcFormat, dstFormat *wca.WAVEFORMATEX) []byte {
	if !c.formatsMismatch {
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

func (c *AudioClient) StartAudioLoop() {
	bufferAccumulator := make([]byte, 0, 4096)
	targetBufferSize := int(c.micFormat.NSamplesPerSec * uint32(c.micFormat.NBlockAlign) / 50) // ~20ms of audio

	for {
		var packetLength uint32
		if err := c.captureClient.GetNextPacketSize(&packetLength); err != nil {
			log.Fatal("GetNextPacketSize failed:", err)
		}
		if packetLength == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		var data *byte
		var numFrames uint32
		var flags uint32
		if err := c.captureClient.GetBuffer(&data, &numFrames, &flags, nil, nil); err != nil {
			log.Fatal("GetBuffer failed:", err)
		}

		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
			srcData := unsafe.Slice(data, numFrames*uint32(c.micFormat.NBlockAlign))
			convertedData := c.convertAudio(srcData, c.micFormat, c.speakerFormat)

			// Accumulate audio data for smoother streaming
			bufferAccumulator = append(bufferAccumulator, srcData...)

			// Send larger chunks to reduce choppy audio
			if len(bufferAccumulator) >= targetBufferSize {
				broadcastAudio(bufferAccumulator, c.micFormat.NSamplesPerSec, c.micFormat.NChannels, c.micFormat.WBitsPerSample)
				bufferAccumulator = bufferAccumulator[:0] // Reset buffer
			}

			// Calculate frames for output
			outputFrames := uint32(len(convertedData)) / uint32(c.speakerFormat.NBlockAlign)

			var renderData *byte
			if err := c.renderClient.GetBuffer(outputFrames, &renderData); err != nil {
				log.Fatal("renderClient GetBuffer failed:", err)
			}

			copy(
				unsafe.Slice(renderData, uint32(len(convertedData))),
				convertedData,
			)

			if err := c.renderClient.ReleaseBuffer(outputFrames, 0); err != nil {
				log.Fatal("renderClient ReleaseBuffer failed:", err)
			}
		}

		if err := c.captureClient.ReleaseBuffer(numFrames); err != nil {
			log.Fatal("captureClient ReleaseBuffer failed:", err)
		}
	}
}

func (c *AudioClient) Close() {
	if c.captureClient != nil {
		c.captureClient.Release()
	}
	if c.renderClient != nil {
		c.renderClient.Release()
	}
	if c.micClient != nil {
		c.micClient.Release()
	}
	if c.speakerClient != nil {
		c.speakerClient.Release()
	}
	ole.CoUninitialize()
}
