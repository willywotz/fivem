package main

/*
#include "miniaudio.h"
#include "hello.h"
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

func main() {
	deviceConfig := C.ma_device_config_init(C.ma_device_type_duplex)
	// deviceConfig.capture.pDeviceID = nil
	// deviceConfig.capture.format = encoder.config.format
	// deviceConfig.capture.channels = encoder.config.channels
	// deviceConfig.sampleRate = encoder.config.sampleRate
	// deviceConfig.dataCallback = (C.ma_device_data_proc)(unsafe.Pointer(C.data_callback))
	// deviceConfig.pUserData = unsafe.Pointer(encoder)

	deviceConfig.capture.pDeviceID = nil
	deviceConfig.capture.format = C.ma_format_f32
	deviceConfig.capture.channels = 2
	deviceConfig.capture.shareMode = C.ma_share_mode_shared
	deviceConfig.playback.pDeviceID = nil
	deviceConfig.playback.format = C.ma_format_f32
	deviceConfig.playback.channels = 2
	deviceConfig.dataCallback = (C.ma_device_data_proc)(unsafe.Pointer(C.data_callback))

	device := (*C.ma_device)(C.malloc(C.size_t(unsafe.Sizeof(C.ma_device{}))))

	if result := C.ma_device_init(nil, &deviceConfig, device); result != C.MA_SUCCESS {
		fmt.Println("Failed to initialize device.")
		os.Exit(int(result))
	}
	defer C.ma_device_uninit(device)

	if result := C.ma_device_start(device); result != C.MA_SUCCESS {
		fmt.Println("Failed to start device.")
		os.Exit(int(result))
	}

	fmt.Println("Press Enter to quit...")
	fmt.Scanln()
}

var audioDataCh = make(chan []byte, 10)

func dataCallback(pCapture, pPlayback []byte, framecount uint32) {
	// The miniaudio callback gives you raw audio data in pCapture.
	// You can send this data to your network goroutine.
	if len(pCapture) > 0 {
		// Use a non-blocking send to the channel
		select {
		case audioDataCh <- pCapture:
		default:
			// Handle buffer overflow if needed (e.g., drop the packet)
			fmt.Println("Warning: Channel buffer full, dropping audio frame.")
		}
	}
}
