package main

import (
	"crypto/sha256"
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
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/hashicorp/go-version"
	"github.com/moutend/go-wca/pkg/wca"
	webview "github.com/webview/webview_go"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

var buildVersion string = "v0"

var binaryFileName string = "fivem-windows-amd64.exe"

func main() {
	_ = becomeAdmin()
	// _ = handleAutoUpdate()

	// if inService, _ := svc.IsWindowsService(); inService {
	// 	runService(svcName, false)
	// 	return
	// }

	// _ = removeService(svcName)

	// go func() { _ = installService(svcName, "FiveM Service") }()

	fmt.Printf("fivem started. version: %s\n", buildVersion)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		fmt.Printf("Failed to initialize OLE: %v", err)
		return
	}
	defer ole.CoUninitialize()

	ui()
}

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

func handleAutoUpdate() error {
	currentExePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("Failed to get current exe path: %w", err)
	}

	currentExeName := filepath.Base(currentExePath)
	if currentExeName != binaryFileName {
		// This is important to avoid deleting/moving a parent process, like go run, during development/testing
		return fmt.Errorf("current exe name does not match expected name: %s != %s", currentExeName, binaryFileName)
	}

	oldFilePath := filepath.Base(currentExePath) + binaryFileName + ".old"
	if _, err := os.Stat(oldFilePath); err != nil && !os.IsNotExist(err) {
		fmt.Printf("Failed to check old file existence: %v\n", err)
	} else if err := os.Remove(oldFilePath); err != nil {
		fmt.Printf("Failed to remove old file: %v\n", err)
	} else if err := hideFile(oldFilePath); err != nil {
		fmt.Printf("Failed to hide old file: %v\n", err)
	}

	release, err := getLatestRelease()
	if err != nil {
		return fmt.Errorf("Failed to get latest release: %w", err)
	}

	v1, _ := version.NewVersion(buildVersion)
	v2, _ := version.NewVersion(release.Version)

	if buildVersion == "" || !v1.LessThan(v2) {
		return fmt.Errorf("Current version %s is not less than latest version %s", buildVersion, release.Version)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		_ = controlService(svcName, svc.Stop, svc.Stopped)
	}

	log.Printf("Current version: %s, Latest version: %s", buildVersion, release.Version)
	log.Printf("Downloading latest release from %s", release.DownloadURL)

	resp, err := http.Get(release.DownloadURL)
	if err != nil {
		return fmt.Errorf("Failed to download latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Failed to download latest release: received status code %d", resp.StatusCode)
	}

	err = os.Rename(currentExePath, currentExePath+".old")
	if err != nil {
		return fmt.Errorf("Failed to rename current exe: %w", err)
	}

	f, err := os.Create(currentExePath)
	if err != nil {
		return fmt.Errorf("Failed to create new binary file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("Failed to write new binary file: %w", err)
	}

	f.Close()

	cmd := exec.Command(currentExePath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Failed to start new binary: %w", err)
	}

	if inService, _ := svc.IsWindowsService(); inService {
		if err := startService(svcName); err != nil {
			return fmt.Errorf("Failed to start service after update: %w", err)
		}
	}

	os.Exit(0)

	return nil
}

func hideFile(path string) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setFileAttributes := kernel32.NewProc("SetFileAttributesW")

	r1, _, err := setFileAttributes.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(path))), 2)

	if r1 == 0 {
		return err
	} else {
		return nil
	}
}

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
)

func becomeAdmin() error {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err == nil {
		return nil // Already running as admin
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if filepath.Base(exe) != binaryFileName {
		return nil
	}

	verb := "runas"

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	args := strings.Join(os.Args[1:], " ")

	verbPtr, _ := syscall.UTF16PtrFromString(verb)
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)
	argPtr, _ := syscall.UTF16PtrFromString(args)

	_, _, err = procShellExecuteW.Call(
		uintptr(0),                       // hwnd
		uintptr(unsafe.Pointer(verbPtr)), // lpOperation
		uintptr(unsafe.Pointer(exePtr)),  // lpFile
		uintptr(unsafe.Pointer(argPtr)),  // lpParameters
		uintptr(unsafe.Pointer(cwdPtr)),  // lpDirectory
		uintptr(1),                       // nShowCmd SW_NORMAL
	)
	if err != nil && err != syscall.Errno(0) /* ERROR_SUCCESS */ {
		return fmt.Errorf("failed to execute ShellExecute: %w", err)
	}

	os.Exit(0) // Exit the current process after starting the new one with admin privileges

	return nil
}

func machineID() (string, error) {
	ss := make([]string, 0)

	// block, err := ghw.Block()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get Block info: %w", err)
	// }
	// ss = append(ss, block.String())

	// base, err := ghw.Baseboard()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get Baseboard info: %w", err)
	// }
	// ss = append(ss, base.String())

	// bios, err := ghw.BIOS()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get BIOS info: %w", err)
	// }
	// ss = append(ss, bios.String())

	// info, err := ghw.CPU()
	// if err != nil {
	// 	return "", fmt.Errorf("failed to get CPU info: %w", err)
	// }
	// ss = append(ss, info.String())

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Cryptography`, registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("failed to open registry key: %w", err)
	}
	defer k.Close()

	machineGuid, _, err := k.GetStringValue("MachineGuid")
	if err != nil {
		return "", fmt.Errorf("failed to get MachineGuid: %w", err)
	}
	ss = append(ss, machineGuid)

	h := sha256.New()
	h.Write([]byte(strings.Join(ss, "")))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

var (
	user32             = syscall.NewLazyDLL("user32.dll")
	procMapVirtualKeyW = user32.NewProc("MapVirtualKeyW")
	procSendInput      = user32.NewProc("SendInput")
)

// Input Types
const (
	INPUT_MOUSE    uint32 = 0
	INPUT_KEYBOARD uint32 = 1
	INPUT_HARDWARE uint32 = 2
)

// Keyboard Event Flags
const (
	KEYEVENTF_EXTENDEDKEY uint32 = 0x0001
	KEYEVENTF_KEYUP       uint32 = 0x0002
	KEYEVENTF_UNICODE     uint32 = 0x0004 // For sending Unicode characters directly
	KEYEVENTF_SCANCODE    uint32 = 0x0008
)

// Virtual Key Codes (common ones, based on winuser.h)
const (
	VK_LBUTTON  uint16 = 0x01 // Left mouse button
	VK_RBUTTON  uint16 = 0x02 // Right mouse button
	VK_CANCEL   uint16 = 0x03 // Control-break processing
	VK_MBUTTON  uint16 = 0x04 // Middle mouse button (three-button mouse)
	VK_XBUTTON1 uint16 = 0x05 // X1 mouse button
	VK_XBUTTON2 uint16 = 0x06 // X2 mouse button
	VK_BACK     uint16 = 0x08 // BACKSPACE key
	VK_TAB      uint16 = 0x09 // TAB key
	VK_CLEAR    uint16 = 0x0C // CLEAR key
	VK_RETURN   uint16 = 0x0D // ENTER key
	VK_SHIFT    uint16 = 0x10 // SHIFT key
	VK_CONTROL  uint16 = 0x11 // CTRL key
	VK_MENU     uint16 = 0x12 // ALT key
	VK_PAUSE    uint16 = 0x13 // PAUSE key
	VK_CAPITAL  uint16 = 0x14 // CAPS LOCK key
	VK_ESCAPE   uint16 = 0x1B // ESC key
	VK_SPACE    uint16 = 0x20 // SPACEBAR
	VK_PRIOR    uint16 = 0x21 // PAGE UP key
	VK_NEXT     uint16 = 0x22 // PAGE DOWN key
	VK_END      uint16 = 0x23 // END key
	VK_HOME     uint16 = 0x24 // HOME key
	VK_LEFT     uint16 = 0x25 // LEFT ARROW key
	VK_UP       uint16 = 0x26 // UP ARROW key
	VK_RIGHT    uint16 = 0x27 // RIGHT ARROW key
	VK_DOWN     uint16 = 0x28 // DOWN ARROW key
	VK_SELECT   uint16 = 0x29 // SELECT key
	VK_PRINT    uint16 = 0x2A // PRINT key
	VK_EXECUTE  uint16 = 0x2B // EXECUTE key
	VK_SNAPSHOT uint16 = 0x2C // PRINT SCREEN key
	VK_INSERT   uint16 = 0x2D // INS key
	VK_DELETE   uint16 = 0x2E // DEL key
	VK_HELP     uint16 = 0x2F // HELP key

	VK_0 uint16 = 0x30
	VK_1 uint16 = 0x31
	VK_2 uint16 = 0x32
	VK_3 uint16 = 0x33
	VK_4 uint16 = 0x34
	VK_5 uint16 = 0x35
	VK_6 uint16 = 0x36
	VK_7 uint16 = 0x37
	VK_8 uint16 = 0x38
	VK_9 uint16 = 0x39

	VK_A uint16 = 0x41
	VK_B uint16 = 0x42
	VK_C uint16 = 0x43
	VK_D uint16 = 0x44
	VK_E uint16 = 0x45
	VK_F uint16 = 0x46
	VK_G uint16 = 0x47
	VK_H uint16 = 0x48
	VK_I uint16 = 0x49
	VK_J uint16 = 0x4A
	VK_K uint16 = 0x4B
	VK_L uint16 = 0x4C
	VK_M uint16 = 0x4D
	VK_N uint16 = 0x4E
	VK_O uint16 = 0x4F
	VK_P uint16 = 0x50
	VK_Q uint16 = 0x51
	VK_R uint16 = 0x52
	VK_S uint16 = 0x53
	VK_T uint16 = 0x54
	VK_U uint16 = 0x55
	VK_V uint16 = 0x56
	VK_W uint16 = 0x57
	VK_X uint16 = 0x58
	VK_Y uint16 = 0x59
	VK_Z uint16 = 0x5A
)

// --- Windows API Structures (Manually Defined) ---

// KEYBDINPUT structure (matches C definition)
type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// MOUSEINPUT structure (included for completeness for the INPUT union, though not used here)
type MOUSEINPUT struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

// HARDWAREINPUT structure (included for completeness for the INPUT union, though not used here)
type HARDWAREINPUT struct {
	Umsg    uint32
	LParamL uint16
	LParamH uint16
}

// INPUT structure - carefully constructed to match the C LAYOUT for SendInput
// This struct accounts for the 'Type' field, followed by 4 bytes of padding (on 64-bit),
// and then a union that is 28 bytes (the size of the largest member, MOUSEINPUT).
// Total size: 4 (Type) + 4 (padding) + 28 (union) = 36 bytes.
// Windows `sizeof(INPUT)` is 40 bytes on 64-bit, meaning there's another 4 bytes of padding
// at the end to make it a multiple of 8 for alignment.
type INPUT struct {
	Type uint32
	_    [4]byte // Padding to align the union on 64-bit systems

	// The `DUMMYUNIONNAME` field is an anonymous struct that occupies the same memory
	// as the union in the C `INPUT` struct. It must be large enough to hold the largest
	// member of the union (MOUSEINPUT is 28 bytes on 64-bit).
	DUMMYUNIONNAME struct {
		_ [28]byte // This byte array acts as the memory space for the union
	}
	_ [4]byte // Additional padding to make total struct size 40 bytes (matching Windows API)
}

// sendKeyInput is a helper function to send a single keyboard input event.
// vkCode: Virtual-key code (e.g., VK_A, VK_RETURN). Use 0 if using scanCode with KEYEVENTF_UNICODE.
// scanCode: Hardware scan code for the key. Use 0 if using vkCode. For KEYEVENTF_UNICODE, this is the character.
// flags: Flags that specify various aspects of function operation (e.g., KEYEVENTF_KEYUP).
func sendKeyInput(vkCode uint16, scanCode uint16, flags uint32) {
	var input INPUT
	input.Type = INPUT_KEYBOARD // Specify that this is a keyboard input event

	// Manually place the KEYBDINPUT data into the union's memory space.
	// We get a pointer to the start of the union's memory (`&input.DUMMYUNIONNAME`),
	// then cast it to a pointer of KEYBDINPUT, and dereference it to assign values.
	kbInput := (*KEYBDINPUT)(unsafe.Pointer(&input.DUMMYUNIONNAME))
	kbInput.WVk = vkCode
	kbInput.WScan = scanCode
	kbInput.DwFlags = flags
	kbInput.Time = 0        // System will provide the timestamp
	kbInput.DwExtraInfo = 0 // Additional data associated with the input

	nInputs := uintptr(1)
	sizeOfInput := uintptr(unsafe.Sizeof(input))

	ret, _, err := syscall.SyscallN(
		procSendInput.Addr(),
		nInputs,                         // cInputs
		uintptr(unsafe.Pointer(&input)), // pInputs
		sizeOfInput,                     // cbSize
		0, 0, 0,                         // Padding to 6 arguments for SyscallN
	)

	if ret != nInputs {
		fmt.Printf("Error: SendInput failed to send all inputs. Sent: %d, Expected: %d, Error: %v\n", ret, nInputs, err)
	}
}

// PressAndRelease simulates pressing and then releasing a keyboard key.
// It takes a virtual key code (e.g., VK_A for 'A').
func PressAndRelease(vkCode uint16) {
	scanCodeVal, _, err := procMapVirtualKeyW.Call(uintptr(vkCode), uintptr(0)) // 0 means MAPVK_VK_TO_VSC
	if scanCodeVal == 0 && err != nil {
		fmt.Printf("Error mapping virtual key: %v\n", err)
		return
	}

	scanCode := uint16(scanCodeVal)

	fmt.Printf("Pressing key with VK_CODE: 0x%X\n", vkCode)

	// Press the key down
	sendKeyInput(0, scanCode, KEYEVENTF_SCANCODE) // Flags 0 means Key Down

	// Add a small delay, similar to pynput's time.sleep(0.1)
	time.Sleep(100 * time.Millisecond) // 0.1 seconds

	// Release the key
	sendKeyInput(0, scanCode, KEYEVENTF_SCANCODE|KEYEVENTF_KEYUP) // KEYEVENTF_KEYUP for Key Up
	fmt.Printf("Released key with VK_CODE: 0x%X\n", vkCode)
}

// PressAndReleaseChar simulates typing a single character.
// This uses KEYEVENTF_UNICODE flag, which is generally simpler for characters
// as it doesn't require mapping to virtual key codes or handling Shift state.
func PressAndReleaseChar(char rune) {
	fmt.Printf("Typing character: '%c' (Unicode: %U)\n", char, char)

	// Key Down with UNICODE flag (Vk is 0, Scan is the Unicode char)
	sendKeyInput(0, uint16(char), KEYEVENTF_UNICODE)

	time.Sleep(100 * time.Millisecond) // 0.1 seconds

	// Key Up with UNICODE and KEYUP flags
	sendKeyInput(0, uint16(char), KEYEVENTF_UNICODE|KEYEVENTF_KEYUP)
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
			// _ = elog.Info(1, "beep")
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

// var beepFunc = syscall.MustLoadDLL("user32.dll").MustFindProc("MessageBeep")

func beep() {
	// _ = exec.Command("powershell", "-c", "[console]::beep(500,300)").Run()
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
	err = s.Start()
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

func installService(name, desc string) error {
	exepath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if filepath.Base(exepath) != binaryFileName {
		return fmt.Errorf("executable is already named %s", binaryFileName)
	}

	userDir, _ := os.UserConfigDir()
	rootDir := filepath.Join(userDir, "willywotz", "fivem")

	if _, err := os.Stat(rootDir); os.IsNotExist(err) {
		if err := os.MkdirAll(rootDir, os.ModePerm); err != nil {
			return fmt.Errorf("failed to create root directory %s: %w", rootDir, err)
		}
	}

	if strings.HasPrefix(exepath, rootDir) {
	} else if _, err := os.Stat(filepath.Join(rootDir, binaryFileName)); err == nil {
		exepath = filepath.Join(rootDir, binaryFileName)
	} else if f, err := os.Create(filepath.Join(rootDir, binaryFileName)); err == nil {
		currentFile, err := os.Open(exepath)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to open current executable: %w", err)
		}
		_, err = io.Copy(f, currentFile)
		if err != nil {
			_ = f.Close()
			_ = currentFile.Close()
			return fmt.Errorf("failed to copy current executable: %w", err)
		}
		_ = f.Sync()
		_ = f.Close()
		_ = currentFile.Close()
		exepath = filepath.Join(rootDir, binaryFileName)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
	}
	defer func() {
		_ = m.Disconnect()
	}()
	s, err := m.OpenService(name)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", name)
	}
	s, err = m.CreateService(name, exepath, mgr.Config{StartType: mgr.StartAutomatic, DisplayName: desc})
	if err != nil {
		return fmt.Errorf("failed to create service %s: %w", name, err)
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(name, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		_ = s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return startService(name)
}

func removeService(name string) error {
	_ = controlService(name, svc.Stop, svc.Stopped)

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to service manager: %w", err)
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
		return fmt.Errorf("RemoveService() failed: %w", err)
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %w", err)
	}
	return nil
}
