package main

import (
	"fmt"

	"github.com/moutend/go-wca/pkg/wca"
)

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
		return nil, fmt.Errorf("failed to create MMDeviceEnumerator: %w", err)
	}
	defer mmde.Release()

	var mdc *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mdc); err != nil {
		return nil, fmt.Errorf("failed to enumerate audio endpoints: %w", err)
	}
	defer mdc.Release()

	var count uint32
	if err := mdc.GetCount(&count); err != nil {
		return nil, fmt.Errorf("failed to get device count: %w", err)
	}

	var devices []AudioDevice

	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err := mdc.Item(i, &mmd); err != nil {
			return nil, fmt.Errorf("failed to get device at index %d: %w", i, err)
		}
		defer mmd.Release()

		var id string
		if err := mmd.GetId(&id); err != nil {
			return nil, fmt.Errorf("failed to get device ID at index %d: %w", i, err)
		}

		var state uint32
		if err := mmd.GetState(&state); err != nil {
			return nil, fmt.Errorf("failed to get device state at index %d: %w", i, err)
		}

		var ps *wca.IPropertyStore
		if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			return nil, fmt.Errorf("failed to open property store at index %d: %w", i, err)
		}
		defer ps.Release()

		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			return nil, fmt.Errorf("failed to get device friendly name at index %d: %w", i, err)
		}

		devices = append(devices, AudioDevice{
			ID:    id,
			Name:  pv.String(),
			State: AudioDeviceState(state),
		})
	}

	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.ECommunications, &mmd); err != nil {
		return nil, fmt.Errorf("failed to get default audio endpoint: %w", err)
	}
	defer mmd.Release()

	var defaultId string
	if err := mmd.GetId(&defaultId); err != nil {
		return nil, fmt.Errorf("failed to get default device ID: %w", err)
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
		return fmt.Errorf("volume level must be between 0.0 and 1.0, got %f", volumeLevel)
	}

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return fmt.Errorf("failed to create MMDeviceEnumerator: %v", err)
	}
	defer mmde.Release()

	var mdc *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mdc); err != nil {
		return fmt.Errorf("failed to enumerate audio endpoints: %w", err)
	}
	defer mdc.Release()

	var count uint32
	if err := mdc.GetCount(&count); err != nil {
		return fmt.Errorf("failed to get device count: %w", err)
	}

	for i := uint32(0); i < count; i++ {
		var mmd *wca.IMMDevice
		if err := mdc.Item(i, &mmd); err != nil {
			return fmt.Errorf("failed to get device at index %d: %w", i, err)
		}
		defer mmd.Release()

		var id string
		if err := mmd.GetId(&id); err != nil {
			return fmt.Errorf("failed to get device ID at index %d: %w", i, err)
		}

		if id != endpointId {
			continue
		}

		var aev *wca.IAudioEndpointVolume
		if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
			return fmt.Errorf("failed to activate audio endpoint volume: %v", err)
		}
		defer aev.Release()

		if err := aev.SetMasterVolumeLevelScalar(volumeLevel, nil); err != nil {
			return fmt.Errorf("failed to set master volume level: %v", err)
		}

		break
	}

	return nil
}
