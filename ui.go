package main

import (
	"fmt"
	"sync"
	"time"

	webview "github.com/webview/webview_go"
)

const html = `
<div style="text-align: center; margin-top: 20px;">
	<h1>fivem</h1>
	<p>Build Version: <span id="build-version"></span></p>
</div>

<script>
window.getBuildVersion().then(version => {
	document.getElementById("build-version").textContent = version;
});
</script>

<label for="audio-input">Select Audio Input Device:</label><br />
<select id="audio-input"></select><br />
<label for="volume-slider">Volume:</label><br />
<input type="range" id="volume-slider" min="0" max="100" value="100" /><br />
<label for="volume-value">Volume:</label><br />
<input type="text" id="volume-value" value="100" readonly /><br />
<label for="volume-interval">Interval in seconds:</label><br />
<input type="text" id="volume-interval" value="1" min="1" /><br />

<script>
	const audioInputElement = document.getElementById("audio-input");
	const volumeSlider = document.getElementById("volume-slider");
	const volumeValue = document.getElementById("volume-value");
	const volumeInterval = document.getElementById("volume-interval");

	let currentEndpointId = "";

	let currentVolume = 100;
	volumeSlider.value = currentVolume;
	volumeValue.value = currentVolume;
	window.setVolume(currentVolume);

	let currentVolumeInterval = 1;
	volumeInterval.value = currentVolumeInterval;
	window.setVolumeInterval(currentVolumeInterval);

	window.getAudioInputDevices().then(devices => {
		devices.forEach(device => {
			const option = document.createElement("option");
			option.value = device.id;
			option.textContent = device.name;
			audioInputElement.appendChild(option);
		});

		devices.filter(device => device.isDefaultAudioEndpoint).forEach(device => {
			currentEndpointId = device.id;
			audioInputElement.value = currentEndpointId;
			window.setVolumeEndpointId(currentEndpointId);
		});
	});

	audioInputElement.addEventListener("change", () => {
		currentEndpointId = audioInputElement.value;
		window.setVolumeEndpointId(currentEndpointId);
	});

	volumeSlider.addEventListener("change", () => {
		onVolumeChange(volumeSlider.value);
	});

	volumeValue.addEventListener("input", () => {
		onVolumeChange(volumeValue.value);
	});

	volumeInterval.addEventListener("input", () => {
		const value = volumeInterval.value;

		if (value === "") {
			return;
		}

		if (/^[0-9]+$/.test(value) === false) {
			alert("Please enter a valid number.");
			volumeInterval.value = currentVolumeInterval;
			return;
		}

		if (value === "" || isNaN(value) || parseInt(value, 10) < 1) {
			alert("Please enter a valid interval in seconds (minimum 1).");
			volumeInterval.value = currentVolumeInterval;
			return;
		}

		currentVolumeInterval = parseInt(value, 10);
		window.setVolumeInterval(currentVolumeInterval);
	});

	function onVolumeChange(value) {
		if (value === "") {
			return;
		}

		if (/^[0-9]+$/.test(value) === false) {
			alert("Please enter a valid number.");
			volumeValue.value = currentVolume;
			return;
		}

		const volume = parseInt(value, 10);

		if (isNaN(volume) || volume < 0 || volume > 100) {
			alert("Please enter a valid volume between 0 and 100.");
			volumeValue.value = currentVolume;
			volumeSlider.value = currentVolume;
			return;
		}

		currentVolume = volume;
		volumeSlider.value = currentVolume;
		volumeValue.value = currentVolume;
		window.setVolume(currentVolume);
	}
</script>
`

func ui() {
	w := webview.New(false)
	defer w.Destroy()

	w.SetTitle("fivem")
	w.SetSize(480, 320, webview.HintNone)

	_ = w.Bind("getBuildVersion", func() string {
		if buildVersion == "" {
			return "unknown"
		}
		return buildVersion
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
	var currentEndpointId string            // Store the currently selected endpoint ID
	var currentVolume float32 = 1.0         // Default volume level (100%)
	var currentVolumeInterval float32 = 1.0 // Default volume interval in seconds

	go func() {
		for {
			volumeMu.Lock()
			// fmt.Printf("Current endpoint ID: %s, Current volume: %f, Interval: %f seconds\n", currentEndpointId, currentVolume, currentVolumeInterval)
			if currentEndpointId != "" && currentVolume >= 0 && currentVolume <= 1.0 {
				if err := setAudioVolume(currentEndpointId, currentVolume); err != nil {
					fmt.Printf("Error setting volume: %v\n", err)
					w.Eval(fmt.Sprintf("alert('Error setting volume: %v');", err.Error()))
				}
			}
			d := 100 * time.Millisecond
			if currentVolumeInterval >= 1.0 {
				d = time.Duration(currentVolumeInterval * float32(time.Second))
			}
			volumeMu.Unlock()

			<-time.After(d)
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

	_ = w.Bind("setVolumeInterval", func(interval float32) {
		volumeMu.Lock()
		defer volumeMu.Unlock()

		if interval < 1 {
			fmt.Printf("Invalid interval: %f. Must be at least 1 second.\n", interval)
			w.Eval(fmt.Sprintf("alert('Invalid interval: %f. Must be at least 1 second.');", interval))
			return
		}

		currentVolumeInterval = interval
		// fmt.Printf("Setting volume interval to %f seconds\n", currentVolumeInterval)
	})

	w.SetHtml(html)
	w.Run()
}
