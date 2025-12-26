package main

/*
#cgo CFLAGS: -I./

#include "audio.h"
*/
import "C"

func main() {
	C.hello()

	// fatal(ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED))
	// defer ole.CoUninitialize()

	// var mmde *wca.IMMDeviceEnumerator
	// fatal(wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde))
	// defer mmde.Release()

	// var mdc *wca.IMMDeviceCollection
	// fatal(mmde.EnumAudioEndpoints(wca.EAll, wca.DEVICE_STATE_ACTIVE|wca.DEVICE_STATE_UNPLUGGED, &mdc))
	// defer mdc.Release()

	// var count uint32
	// fatal(mdc.GetCount(&count))

	// for i := uint32(0); i < count; i++ {
	// 	var mmd *wca.IMMDevice
	// 	fatal(mdc.Item(i, &mmd))
	// 	defer mmd.Release()

	// 	var id string
	// 	fatal(mmd.GetId(&id))

	// 	var state uint32
	// 	fatal(mmd.GetState(&state))

	// 	var ps *wca.IPropertyStore
	// 	fatal(mmd.OpenPropertyStore(wca.STGM_READ, &ps))
	// 	defer ps.Release()

	// 	var pv wca.PROPVARIANT
	// 	fatal(ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv))
	// 	name := pv.String()

	// 	fmt.Printf("Device %d: ID=%s, Name=%s, State=%d\n", i, id, name, state)
	// }

	// var mmd *wca.IMMDevice
	// fatal(mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mmd))
	// defer mmd.Release()

	// id, name, state, err := deviceProperty(mmd)
	// fatal(err)
	// fmt.Printf("Default Audio Capture Device: ID=%s, Name=%s, State=%d\n", id, name, state)

	// fmt.Scanln()
}

// func deviceProperty(mmd *wca.IMMDevice) (string, string, uint32, error) {
// 	var id string
// 	if err := mmd.GetId(&id); err != nil {
// 		return "", "", 0, fmt.Errorf("failed to get device ID: %w", err)
// 	}

// 	var state uint32
// 	if err := mmd.GetState(&state); err != nil {
// 		return "", "", 0, fmt.Errorf("failed to get device state: %w", err)
// 	}

// 	var ps *wca.IPropertyStore
// 	if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
// 		return "", "", 0, fmt.Errorf("failed to open property store: %w", err)
// 	}
// 	defer ps.Release()

// 	var pv wca.PROPVARIANT
// 	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
// 		return "", "", 0, fmt.Errorf("failed to get device friendly name: %w", err)
// 	}
// 	name := pv.String()

// 	return id, name, state, nil
// }
