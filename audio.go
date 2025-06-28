package main

import (
	"fmt"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

func getAudioDeviceCollection() (*wca.IMMDeviceCollection, error) {
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return nil, fmt.Errorf("Failed to initialize OLE: %v", err)
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return nil, fmt.Errorf("Failed to create MMDeviceEnumerator: %v", err)
	}
	defer mmde.Release()

	var mdc *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.EAll, wca.DEVICE_STATEMASK_ALL, &mdc); err != nil {
		return nil, fmt.Errorf("Failed to enumerate audio endpoints: %v", err)
	}
	defer mdc.Release()

	var count uint32
	if err := mdc.GetCount(&count); err != nil {
		return nil, fmt.Errorf("Failed to get device count: %v", err)
	}
	fmt.Printf("Number of audio devices: %d\n", count)

	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err := mdc.Item(i, &mmd); err != nil {
			return nil, fmt.Errorf("Failed to get device at index %d: %v", i, err)
		}
		defer mmd.Release()

		var id string
		if err := mmd.GetId(&id); err != nil {
			return nil, fmt.Errorf("Failed to get device ID at index %d: %v", i, err)
		}

		var state uint32
		if err := mmd.GetState(&state); err != nil {
			return nil, fmt.Errorf("Failed to get device state at index %d: %v", i, err)
		}

		var ps *wca.IPropertyStore
		if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			return nil, fmt.Errorf("Failed to open property store at index %d: %v", i, err)
		}
		defer ps.Release()

		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			return nil, fmt.Errorf("Failed to get device friendly name at index %d: %v", i, err)
		}

		fmt.Printf("Device %d ID: %s State: %d Friendly Name: %s\n", i, id, state, pv.String())
	}

	var mmd *wca.IMMDevice
	if err := mdc.Item(0, &mmd); err != nil {
		return nil, fmt.Errorf("Failed to get device at index %d: %v", 0, err)
	}
	defer mmd.Release()

	var id string
	if err := mmd.GetId(&id); err != nil {
		return nil, fmt.Errorf("Failed to get device ID at index %d: %v", 0, err)
	}
	var state uint32
	if err := mmd.GetState(&state); err != nil {
		return nil, fmt.Errorf("Failed to get device state at index %d: %v", 0, err)
	}

	var ps *wca.IPropertyStore
	if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return nil, fmt.Errorf("Failed to open property store at index %d: %v", 0, err)
	}
	defer ps.Release()
	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return nil, fmt.Errorf("Failed to get device friendly name at index %d: %v", 0, err)
	}

	fmt.Printf("Device %d ID: %s State: %d Friendly Name: %s\n", 0, id, state, pv.String())

	return mdc, nil
}

func setDefaultMicrophoneVolume() error {
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

	if err := aev.SetMasterVolumeLevelScalar(1.0, nil); err != nil {
		return fmt.Errorf("Failed to set master volume level: %v", err)
	}

	return nil
}
