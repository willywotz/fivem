package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/hashicorp/go-version"
	"github.com/moutend/go-wca/pkg/wca"
	webview "github.com/webview/webview_go"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var buildVersion string = "v0"

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

type IncrementResult struct {
	Count uint `json:"count"`
}

func main() {
	fmt.Printf("fivem started. version: %s\n", buildVersion)

	if b, err := handleAutoUpdate(); err != nil {
		fmt.Printf("Error handling auto update: %v\n", err)
	} else if b {
		fmt.Println("Auto update handled successfully, exiting.")
		return
	}

	if inService, _ := svc.IsWindowsService(); inService {
		runService(svcName, false)
		return
	}

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()
}

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
			fmt.Printf("Current endpoint ID: %s, Current volume: %f, Interval: %f seconds\n", currentEndpointId, currentVolume, currentVolumeInterval)
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
		fmt.Printf("Current endpoint ID set to: %s\n", currentEndpointId)
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
		fmt.Printf("Setting volume to %f\n", currentVolume)
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
		fmt.Printf("Setting volume interval to %f seconds\n", currentVolumeInterval)
	})

	w.SetHtml(html)
	w.Run()
}

var elog debug.Log

var svcName = "FiveM Service"

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		elog = debug.New(name)
	} else {
		elog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer elog.Close()

	_ = elog.Info(1, fmt.Sprintf("starting %s service, version: %s", name, buildVersion))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &exampleService{})
	if err != nil {
		_ = elog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	_ = elog.Info(1, fmt.Sprintf("%s service stopped", name))
}

type exampleService struct{}

func (m *exampleService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	changes <- svc.Status{State: svc.StartPending}
	fasttick := time.Tick(500 * time.Millisecond)
	slowtick := time.Tick(2 * time.Second)
	tick := fasttick
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
loop:
	for {
		select {
		case <-tick:
			beep()
			_ = elog.Info(1, "beep")
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus
				// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
				time.Sleep(100 * time.Millisecond)
				changes <- c.CurrentStatus
			case svc.Stop, svc.Shutdown:
				// golang.org/x/sys/windows/svc.TestExample is verifying this output.
				testOutput := strings.Join(args, "-")
				testOutput += fmt.Sprintf("-%d", c.Context)
				_ = elog.Info(1, testOutput)
				break loop
			case svc.Pause:
				changes <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
				tick = slowtick
			case svc.Continue:
				changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
				tick = fasttick
			default:
				_ = elog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
			}
		}
	}
	changes <- svc.Status{State: svc.StopPending}
	return
}

var beepFunc = syscall.MustLoadDLL("user32.dll").MustFindProc("MessageBeep")

func beep() {
	_, _, _ = beepFunc.Call(0xffffffff)
}

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		return "", fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			return "", fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}

func installService(name, desc string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{DisplayName: desc}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %w", err)
	}
	return nil
}

type AudioDevice struct {
	ID    string           `json:"id"`
	Name  string           `json:"name"`
	State AudioDeviceState `json:"state"`

	IsDefaultAudioEndpoint bool `json:"isDefaultAudioEndpoint"`
}

type AudioDeviceState int

const (
	AudioDeviceStateActive AudioDeviceState = iota
	AudioDeviceStateDisabled
	AudioDeviceStateNotPresent
	AudioDeviceStateUnplugged
)

func getAudioInputDevices() ([]AudioDevice, error) {
	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return nil, fmt.Errorf("Failed to create MMDeviceEnumerator: %w", err)
	}
	defer mmde.Release()

	var mdc *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mdc); err != nil {
		return nil, fmt.Errorf("Failed to enumerate audio endpoints: %w", err)
	}
	defer mdc.Release()

	var count uint32
	if err := mdc.GetCount(&count); err != nil {
		return nil, fmt.Errorf("Failed to get device count: %w", err)
	}

	var devices []AudioDevice

	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err := mdc.Item(i, &mmd); err != nil {
			return nil, fmt.Errorf("Failed to get device at index %d: %w", i, err)
		}
		defer mmd.Release()

		var id string
		if err := mmd.GetId(&id); err != nil {
			return nil, fmt.Errorf("Failed to get device ID at index %d: %w", i, err)
		}

		var state uint32
		if err := mmd.GetState(&state); err != nil {
			return nil, fmt.Errorf("Failed to get device state at index %d: %w", i, err)
		}

		var ps *wca.IPropertyStore
		if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			return nil, fmt.Errorf("Failed to open property store at index %d: %w", i, err)
		}
		defer ps.Release()

		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			return nil, fmt.Errorf("Failed to get device friendly name at index %d: %w", i, err)
		}

		devices = append(devices, AudioDevice{
			ID:    id,
			Name:  pv.String(),
			State: AudioDeviceState(state),
		})
	}

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.ECommunications, &mmd); err != nil {
		return nil, fmt.Errorf("Failed to get default audio endpoint: %w", err)
	}
	defer mmd.Release()

	var defaultId string
	if err := mmd.GetId(&defaultId); err != nil {
		return nil, fmt.Errorf("Failed to get default device ID: %w", err)
	}

	for i := range devices {
		if devices[i].ID == defaultId {
			devices[i].IsDefaultAudioEndpoint = true
		}
	}

	return devices, nil
}

func setAudioVolume(endpointId string, volumeLevel float32) error {
	if volumeLevel < 0 || volumeLevel > 1 {
		return fmt.Errorf("Volume level must be between 0.0 and 1.0, got %f", volumeLevel)
	}

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return fmt.Errorf("Failed to create MMDeviceEnumerator: %v", err)
	}
	defer mmde.Release()

	var mdc *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mdc); err != nil {
		return fmt.Errorf("Failed to enumerate audio endpoints: %w", err)
	}
	defer mdc.Release()

	var count uint32
	if err := mdc.GetCount(&count); err != nil {
		return fmt.Errorf("Failed to get device count: %w", err)
	}

	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err := mdc.Item(i, &mmd); err != nil {
			return fmt.Errorf("Failed to get device at index %d: %w", i, err)
		}
		defer mmd.Release()

		var id string
		if err := mmd.GetId(&id); err != nil {
			return fmt.Errorf("Failed to get device ID at index %d: %w", i, err)
		}

		if id != endpointId {
			continue
		}

		var aev *wca.IAudioEndpointVolume
		if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
			return fmt.Errorf("Failed to activate audio endpoint volume: %v", err)
		}
		defer aev.Release()

		if err := aev.SetMasterVolumeLevelScalar(volumeLevel, nil); err != nil {
			return fmt.Errorf("Failed to set master volume level: %v", err)
		}

		break
	}

	return nil
}

type Release struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
}

func getLatestRelease() (*Release, error) {
	url := "https://api.github.com/repos/willywotz/fivem/releases/latest"

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch latest release: received status code %d", resp.StatusCode)
	}

	var data struct {
		TagName string `json:"tag_name"`

		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode latest release response: %w", err)
	}
	if data.TagName == "" {
		return nil, fmt.Errorf("latest release does not have a tag name")
	}
	if len(data.Assets) == 0 {
		return nil, fmt.Errorf("latest release does not have any assets")
	}

	release := Release{
		Version:     data.TagName,
		DownloadURL: data.Assets[0].URL,
	}

	return &release, nil
}

func launchProcessForked(binaryFilePath string, args ...string) {
	cmd := exec.Command(binaryFilePath, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	err := cmd.Start()
	if err != nil {
		panic(fmt.Errorf("start new process: %w", err))
	}
}

func handleAutoUpdate() (bool, error) {
	binaryFileName := "fivem-windows-amd64.exe"

	currentExePath, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("Failed to get current exe path: %w", err)
	}

	currentExeName := filepath.Base(currentExePath)
	if currentExeName != binaryFileName {
		// This is important to avoid deleting/moving a parent process, like go run, during development/testing
		return false, fmt.Errorf("current exe name does not match expected name: %s != %s", currentExeName, binaryFileName)
	}

	oldFilePath := filepath.Base(currentExePath) + binaryFileName + ".old"
	if _, err := os.Stat(oldFilePath); err == nil {
		_ = os.Remove(oldFilePath)
	}

	release, err := getLatestRelease()
	if err != nil {
		return false, fmt.Errorf("Failed to get latest release: %w", err)
	}

	v1, _ := version.NewVersion(buildVersion)
	v2, _ := version.NewVersion(release.Version)

	if buildVersion == "" || !v1.LessThan(v2) {
		return false, fmt.Errorf("Current version %s is not less than latest version %s", buildVersion, release.Version)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		_ = controlService(svcName, svc.Stop, svc.Stopped)
	}

	log.Printf("Current version: %s, Latest version: %s", buildVersion, release.Version)
	log.Printf("Downloading latest release from %s", release.DownloadURL)

	resp, err := http.Get(release.DownloadURL)
	if err != nil {
		return false, fmt.Errorf("Failed to download latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Failed to download latest release: received status code %d", resp.StatusCode)
	}

	err = os.Rename(currentExePath, currentExePath+".old")
	if err != nil {
		return false, fmt.Errorf("rename current exe: %w", err)
	}

	f, err := os.Create(currentExePath)
	if err != nil {
		return false, fmt.Errorf("Failed to create new binary file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return false, fmt.Errorf("Failed to write new binary file: %w", err)
	}

	f.Close()

	cmd := exec.Command(currentExePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("Failed to start new binary: %w", err)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		if err := startService(svcName); err != nil {
			return false, fmt.Errorf("Failed to start service after update: %w", err)
		}
	}

	return true, nil
}
