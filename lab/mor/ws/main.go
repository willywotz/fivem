package main

/*
#include "stdio.h"
#include "string.h"
#include "miniaudio.h"

void data_callback(ma_device* pDevice, void* pOutput, const void* pInput, ma_uint32 frameCount)
{
    if (pDevice->capture.format != pDevice->playback.format || pDevice->capture.channels != pDevice->playback.channels) {
        return;
    }

    memcpy(pOutput, pInput, frameCount * ma_get_bytes_per_frame(pDevice->capture.format, pDevice->capture.channels));
}
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	"unsafe"

	"github.com/gorilla/websocket"
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
	deviceConfig.capture.format = C.ma_format_s16
	deviceConfig.capture.channels = 2
	deviceConfig.capture.shareMode = C.ma_share_mode_shared
	deviceConfig.playback.pDeviceID = nil
	deviceConfig.playback.format = C.ma_format_s16
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
	return

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/ws", wsHandler)

	log.Println("server listening on :8080")
	log.Println(http.ListenAndServe(":8080", nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

var upgrader = websocket.Upgrader{}

type Message struct {
	Type int
	Data []byte
}

const MessagePrefix uint16 = 24531

const (
	MethodPing uint16 = iota + 1
	MethodPong
	MethodText
	MethodJSON
)

func MatchMessagePrefix(b []byte) bool {
	return binary.LittleEndian.Uint16(b) == MessagePrefix
}

func MatchMessageMethod(b []byte, method uint16) bool {
	return binary.LittleEndian.Uint16(b) == method
}

func WriteBinaryMessage(writeCh chan Message, method uint16, data []byte) error {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, MessagePrefix); err != nil {
		return fmt.Errorf("failed to write message prefix: %v", err)
	}
	if err := binary.Write(&buf, binary.LittleEndian, method); err != nil {
		return fmt.Errorf("failed to write message method: %v", err)
	}
	if _, err := buf.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %v", err)
	}
	writeCh <- Message{Type: websocket.BinaryMessage, Data: buf.Bytes()}
	return nil
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("failed to upgrade connection: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	go func() {
		for range time.Tick(1 * time.Second) {
			deadline := time.Now().Add(5 * time.Second)
			err := conn.WriteControl(websocket.PingMessage, []byte{}, deadline)
			if err == nil {
				continue
			}
			log.Printf("failed to send ping: %v", err)
			break
		}
	}()

	writeCh := make(chan Message, 100)

	go func() {
		for msg := range writeCh {
			w, err := conn.NextWriter(msg.Type)
			if err != nil {
				log.Printf("failed to get next writer: %v", err)
				return
			}

			if _, err := w.Write(msg.Data); err != nil {
				log.Printf("failed to write message: %v", err)
				return
			}
		}
	}()

	// go func() {
	// 	for range time.Tick(1 * time.Second) {
	// 		var buf bytes.Buffer
	// 		_ = binary.Write(&buf, binary.LittleEndian, MessagePrefix)
	// 		_ = binary.Write(&buf, binary.LittleEndian, MethodPing)
	// 		writeCh <- Message{Type: websocket.BinaryMessage, Data: buf.Bytes()}
	// 	}
	// }()

	for {
		messageType, r, err := conn.NextReader()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseGoingAway) {
				break
			}
			log.Printf("failed to get next reader: %v", err)
			break
		}
		if messageType == websocket.TextMessage {
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				log.Printf("failed to read message: %v", err)
				continue
			}
			log.Printf("received text message: %s", buf.String())
		}
		if messageType == websocket.BinaryMessage {
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(r); err != nil {
				log.Printf("failed to read binary message: %v", err)
				continue
			}
			if !MatchMessagePrefix(buf.Next(2)) {
				log.Printf("invalid message prefix")
				continue
			}
			if MatchMessageMethod(buf.Next(2), MethodPing) {
				_ = WriteBinaryMessage(writeCh, MethodPong, nil)
				continue
			}
		}
	}
}
