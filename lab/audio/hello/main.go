package main

/*
#cgo CFLAGS: -I.
#include "miniaudio.h"
#include <stdlib.h>
#include <stdio.h>

// --- C wrappers for use from Go ---

// Wrapper for the data callback for loopback recording.
static void go_data_callback(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount)
{
    ma_encoder_write_pcm_frames((ma_encoder*)pDevice->pUserData, pInput, frameCount, NULL);
    (void)pOutput;
}

// Initializes the encoder and device for loopback recording.
// Returns MA_SUCCESS (0) on success, other values on error.
int go_init_miniaudio(const char* outFile, ma_encoder* pEncoder, ma_device* pDevice) {
    ma_result result;
    ma_encoder_config encoderConfig;
    ma_device_config deviceConfig;
    ma_backend backends[] = { ma_backend_wasapi };

    encoderConfig = ma_encoder_config_init(ma_encoding_format_mp3, ma_format_s16, 2, 44100);

    result = ma_encoder_init_file(outFile, &encoderConfig, pEncoder);
    if (result != MA_SUCCESS) {
        printf("failed to initialize output file.\n");
        return -1;
    }

    deviceConfig = ma_device_config_init(ma_device_type_loopback);
    deviceConfig.capture.pDeviceID = NULL;
    deviceConfig.capture.format    = pEncoder->config.format;
    deviceConfig.capture.channels  = pEncoder->config.channels;
    deviceConfig.sampleRate        = pEncoder->config.sampleRate;
    deviceConfig.dataCallback      = go_data_callback;
    deviceConfig.pUserData         = pEncoder;

    result = ma_device_init_ex(backends, sizeof(backends)/sizeof(backends[0]), NULL, &deviceConfig, pDevice);
    if (result != MA_SUCCESS) {
        printf("failed to initialize loopback device.\n");
        return -2;
    }

    return 0;
}

int go_start_device(ma_device* pDevice) {
    ma_result result = ma_device_start(pDevice);
    if (result != MA_SUCCESS) {
        ma_device_uninit(pDevice);
        printf("failed to start device.\n");
        return -3;
    }
    return 0;
}
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("No output file.")
		os.Exit(1)
	}
	outputFile := C.CString(os.Args[1])
	defer C.free(unsafe.Pointer(outputFile))

	// var encoder C.ma_encoder
	// var device C.ma_device

	encoder := (*C.ma_encoder)(C.malloc(C.size_t(unsafe.Sizeof(C.ma_encoder{}))))
	defer C.free(unsafe.Pointer(encoder))
	device := (*C.ma_device)(C.malloc(C.size_t(unsafe.Sizeof(C.ma_device{}))))
	defer C.free(unsafe.Pointer(device))

	// Initialize encoder and device
	result := C.go_init_miniaudio(outputFile, encoder, device)
	if result != 0 {
		os.Exit(int(result))
	}

	// Start device
	result = C.go_start_device(device)
	if result != 0 {
		os.Exit(int(result))
	}

	fmt.Println("Press Enter to stop recording...")
	fmt.Scanln()

	C.ma_device_uninit(device)
	C.ma_encoder_uninit(encoder)
}
