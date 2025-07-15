package main

import (
	"embed"
	"fmt"
	"sync"
	"time"

	webview "github.com/webview/webview_go"
)

//go:embed templates/*
var templates embed.FS

var indexFile, _ = templates.ReadFile("templates/index.html")

func ui() {
	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("fivem")
	w.SetSize(480, 320, webview.HintNone)

	_ = w.Bind("getVersion", func() string {
		if version == "" {
			return "unknown"
		}
		return version
	})

	_ = w.Bind("getAudioInputDevices", func() []AudioDevice {
		devices, err := getAudioInputDevices()
		if err != nil {
			fmt.Printf("Error getting audio input devices: %v\n", err)
			w.Eval(fmt.Sprintf("alert('Error getting audio input devices: %v');", err.Error()))
			return []AudioDevice{}
		}
		return devices
	})

	var volumeMu sync.Mutex
	var currentEndpointId string    // Store the currently selected endpoint ID
	var currentVolume float32 = 1.0 // Default volume level (100%)

	go func() {
		for {
			volumeMu.Lock()
			if currentEndpointId != "" && currentVolume >= 0 && currentVolume <= 1.0 {
				if err := setAudioVolume(currentEndpointId, currentVolume); err != nil {
					fmt.Printf("Error setting volume: %v\n", err)
					w.Eval(fmt.Sprintf("alert('Error setting volume: %v');", err.Error()))
				}
			}
			volumeMu.Unlock()

			<-time.After(100 * time.Millisecond)
		}
	}()

	_ = w.Bind("setVolumeEndpointId", func(endpointId string) {
		volumeMu.Lock()
		defer volumeMu.Unlock()

		if endpointId == "" {
			fmt.Println("Endpoint ID cannot be empty.")
			w.Eval("alert('Endpoint ID cannot be empty.');")
			return
		}

		currentEndpointId = endpointId
		// fmt.Printf("Current endpoint ID set to: %s\n", currentEndpointId)
	})

	_ = w.Bind("setVolume", func(volume int) {
		volumeMu.Lock()
		defer volumeMu.Unlock()

		if volume < 0 || volume > 100 {
			fmt.Printf("Invalid volume level: %d. Must be between 0 and 100.\n", volume)
			w.Eval(fmt.Sprintf("alert('Invalid volume level: %d. Must be between 0 and 100.');", volume))
			return
		}

		currentVolume = float32(volume) / 100.0
		// fmt.Printf("Setting volume to %f\n", currentVolume)
	})

	w.SetHtml(string(indexFile))
	w.Run()
}
