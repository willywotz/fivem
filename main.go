package main

import (
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

func main() {
	if err := setDefaultMicrophoneVolume(1.0); err != nil {
		fmt.Printf("Error setting default microphone volume: %v\n", err)
	}
}

func setDefaultMicrophoneVolume(level float32) error {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return fmt.Errorf("Failed to initialize OLE: %v", err)
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return fmt.Errorf("Failed to create MMDeviceEnumerator: %v", err)
	}
	defer mmde.Release()

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.ECommunications, &mmd); err != nil {
		return fmt.Errorf("Failed to get default audio endpoint: %v", err)
	}
	defer mmd.Release()

	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return fmt.Errorf("Failed to activate audio endpoint volume: %v", err)
	}
	defer aev.Release()

	if err := aev.SetMasterVolumeLevelScalar(level, nil); err != nil {
		return fmt.Errorf("Failed to set master volume level: %v", err)
	}

	return nil
}
