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
	w.SetSize(480, 320, webview.HintFixed)

	_ = w.Bind("getVersion", func() string { return version })

	_ = w.Bind("getAudioInputDevices", func() []AudioDevice {
		devices, err := getAudioInputDevices()
		if err != nil {
			failed("Error getting audio input devices: %v", err)
			w.Eval(fmt.Sprintf("alert('Error getting audio input devices: %v');", err.Error()))
			return []AudioDevice{}
		}
		return devices
	})

	var volumeMu sync.Mutex
	var currentEndpointId string
	var currentVolume float32 = 1.0

	go func() {
		for range time.Tick(100 * time.Millisecond) {
			volumeMu.Lock()
			a, b := currentEndpointId, currentVolume
			volumeMu.Unlock()

			if a == "" || b < 0 || b > 1.0 {
				return
			}

			if err := setAudioVolume(a, b); err != nil {
				failed("Error setting volume: %v", err)
				w.Eval(fmt.Sprintf("alert('Error setting volume: %v');", err.Error()))
			}
		}
	}()

	_ = w.Bind("setVolumeEndpointId", func(endpointId string) {
		volumeMu.Lock()
		defer volumeMu.Unlock()

		if endpointId == "" {
			failed("Endpoint ID cannot be empty.")
			w.Eval("alert('Endpoint ID cannot be empty.');")
			return
		}

		currentEndpointId = endpointId
	})

	_ = w.Bind("setVolume", func(volume int) {
		volumeMu.Lock()
		defer volumeMu.Unlock()

		if volume < 0 || volume > 100 {
			failed("Invalid volume level: %d. Must be between 0 and 100.", volume)
			w.Eval(fmt.Sprintf("alert('Invalid volume level: %d. Must be between 0 and 100.');", volume))
			return
		}

		currentVolume = float32(volume) / 100.0
	})

	w.SetHtml(string(indexFile))
	w.Run()
}
