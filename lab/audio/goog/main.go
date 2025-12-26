package main

/*
#cgo LDFLAGS: -lvorbisenc -lvorbis -logg -lm -static

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>
#include <vorbis/vorbisenc.h>
#include <ogg/ogg.h>

static float** get_vorbis_analysis_buffer(vorbis_dsp_state *v, int samples) {
    return vorbis_analysis_buffer(v, samples);
}

// C helper function to safely copy data from an ogg_page member
// This ensures that the pointer passed to GoBytes is to C-allocated memory
// that *you* control, not internal libogg buffers.
unsigned char* copy_ogg_data(unsigned char* src, int len) {
    if (src == NULL || len <= 0) {
        return NULL;
    }
    unsigned char* dest = (unsigned char*) malloc(len);
    if (dest != NULL) {
        memcpy(dest, src, len);
    }
    return dest;
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// func main() {
// 	if len(os.Args) < 2 {
// 		fmt.Println("Usage: go run . <vorbis_file.ogg>")
// 		return
// 	}
// 	filePath := os.Args[1]

// 	// Convert Go string to C string
// 	cFilePath := C.CString(filePath)
// 	defer C.free(unsafe.Pointer(cFilePath))

// 	// Open the file using the C helper function
// 	cFile := C.open_vorbis_file(cFilePath)
// 	if cFile == nil {
// 		fmt.Printf("Error: Could not open file %s\n", filePath)
// 		return
// 	}
// 	defer C.fclose(cFile)

// 	var vf C.OggVorbis_File
// 	ret := C.ov_open(cFile, &vf, nil, 0)
// 	if ret < 0 {
// 		fmt.Printf("Error: Not an Ogg Vorbis file or error opening: %d\n", ret)
// 		return
// 	}
// 	defer C.ov_clear(&vf) // Clean up the vorbis file handle

// 	vi := C.ov_info(&vf, -1) // Get info for the first (or only) logical bitstream
// 	if vi == nil {
// 		fmt.Println("Error: Could not get Vorbis info.")
// 		return
// 	}

// 	fmt.Printf("File: %s\n", filePath)
// 	fmt.Printf("Channels: %d\n", vi.channels)
// 	fmt.Printf("Sample Rate: %d\n", vi.rate)
// 	fmt.Printf("Bitrate (Nominal): %d bps\n", vi.bitrate_nominal)
// 	fmt.Printf("Bitrate (Min): %d bps\n", vi.bitrate_lower)
// 	fmt.Printf("Bitrate (Max): %d bps\n", vi.bitrate_upper)
// 	fmt.Printf("Vorbis Version: %d\n", vi.version)

// 	// // Example of getting comments (optional, more complex with string parsing)
// 	vc := C.ov_comment(&vf, -1)
// 	if vc != nil {
// 		fmt.Println("\nComments:")
// 		for i := 0; i < int(vc.comments); i++ {
// 			// Access user_comments[i]
// 			commentPtr := *(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(vc.user_comments)) + uintptr(i)*unsafe.Sizeof(uintptr(0))))
// 			commentLen := *(*C.int)(unsafe.Pointer(uintptr(unsafe.Pointer(vc.comment_lengths)) + uintptr(i)*unsafe.Sizeof(C.int(0))))
// 			commentGoBytes := C.GoBytes(unsafe.Pointer(commentPtr), commentLen)
// 			fmt.Printf("  %s\n", string(commentGoBytes))
// 		}
// 		// Vendor string is also available: C.GoString(vc.vendor)
// 	}
// }

func main() {
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	audioCh := make(chan []byte, 128)

	go func() {
		serverAddr, err := net.ResolveUDPAddr("udp", "192.168.6.4:8080")
		if err != nil {
			log.Println("Error resolving UDP address:", err)
			return
		}

		conn, err := net.DialUDP("udp", nil, serverAddr)
		if err != nil {
			log.Println("Error dialing UDP:", err)
			return
		}
		defer conn.Close()

		for data := range audioCh {
			if len(data) == 0 {
				continue
			}

			// Send audio data over UDP
			_, err := conn.Write(data)
			if err != nil {
				log.Println("Error sending data over UDP:", err)
				return
			}
		}
	}()

	// 1. Initialize COM
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		log.Fatal("CoInitializeEx failed:", err)
	}
	defer ole.CoUninitialize()

	// 2. Get IMMDeviceEnumerator
	var enumerator *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(
		wca.CLSID_MMDeviceEnumerator,
		0,
		wca.CLSCTX_ALL,
		wca.IID_IMMDeviceEnumerator,
		&enumerator,
	); err != nil {
		log.Fatal("CoCreateInstance failed:", err)
	}
	defer enumerator.Release()

	// 3. Get default input (microphone) device
	var mic *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ECapture, wca.EConsole, &mic); err != nil {
		log.Fatal("GetDefaultAudioEndpoint (input) failed:", err)
	}
	defer mic.Release()

	// 4. Get default output device
	var speaker *wca.IMMDevice
	if err := enumerator.GetDefaultAudioEndpoint(wca.ERender, wca.EConsole, &speaker); err != nil {
		log.Fatal("GetDefaultAudioEndpoint (output) failed:", err)
	}
	defer speaker.Release()

	// 5. Activate IAudioClient for both devices
	var micClient *wca.IAudioClient
	if err := mic.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&micClient,
	); err != nil {
		log.Fatal("Activate (mic) failed:", err)
	}
	defer micClient.Release()

	var speakerClient *wca.IAudioClient
	if err := speaker.Activate(
		wca.IID_IAudioClient,
		wca.CLSCTX_ALL,
		nil,
		&speakerClient,
	); err != nil {
		log.Fatal("Activate (speaker) failed:", err)
	}
	defer speakerClient.Release()

	// 6. Get mix format (shared mode, must match for zero-copy)
	var micFormat *wca.WAVEFORMATEX
	if err := micClient.GetMixFormat(&micFormat); err != nil {
		log.Fatal("mic GetMixFormat failed:", err)
	}
	var speakerFormat *wca.WAVEFORMATEX
	if err := speakerClient.GetMixFormat(&speakerFormat); err != nil {
		log.Fatal("speaker GetMixFormat failed:", err)
	}

	var defaultPeriod wca.REFERENCE_TIME
	var minimumPeriod wca.REFERENCE_TIME
	var latency time.Duration
	if err := speakerClient.GetDevicePeriod(&defaultPeriod, &minimumPeriod); err != nil {
		log.Fatal("GetDevicePeriod failed:", err)
		return
	}
	latency = time.Duration(int(defaultPeriod) * 100)
	_ = latency // Use this latency for your application logic

	// 7. Initialize audio clients (shared mode, event driven)
	if err := micClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		micFormat,
		nil,
	); err != nil {
		log.Fatal("micClient Initialize failed:", err)
	}
	if err := speakerClient.Initialize(
		wca.AUDCLNT_SHAREMODE_SHARED,
		wca.AUDCLNT_STREAMFLAGS_EVENTCALLBACK,
		minimumPeriod,
		0,
		speakerFormat,
		nil,
	); err != nil {
		log.Fatal("speakerClient Initialize failed:", err)
	}

	fakeAudioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	defer func() { _ = wca.CloseHandle(fakeAudioReadyEvent) }()

	if err := micClient.SetEventHandle(fakeAudioReadyEvent); err != nil {
		log.Fatal("micClient SetEventHandle failed:", err)
		return
	}

	audioReadyEvent := wca.CreateEventExA(0, 0, 0, wca.EVENT_MODIFY_STATE|wca.SYNCHRONIZE)
	defer func() { _ = wca.CloseHandle(audioReadyEvent) }()

	if err := speakerClient.SetEventHandle(audioReadyEvent); err != nil {
		log.Fatal("speakerClient SetEventHandle failed:", err)
		return
	}

	// 8. Get IAudioCaptureClient and IAudioRenderClient
	var captureClient *wca.IAudioCaptureClient
	if err := micClient.GetService(wca.IID_IAudioCaptureClient, &captureClient); err != nil {
		log.Fatal("mic GetService failed:", err)
	}
	defer captureClient.Release()

	var renderClient *wca.IAudioRenderClient
	if err := speakerClient.GetService(wca.IID_IAudioRenderClient, &renderClient); err != nil {
		log.Fatal("speaker GetService failed:", err)
	}
	defer renderClient.Release()

	// 9. Start audio clients
	if err := micClient.Start(); err != nil {
		log.Fatal("micClient Start failed:", err)
	}
	if err := speakerClient.Start(); err != nil {
		log.Fatal("speakerClient Start failed:", err)
	}

	const (
		sampleRate = 48000 // Common sample rate for audio
		channels   = 2     // Stereo
		quality    = 0.5   // VBR quality (0.0 to 1.0)
	)

	encoder, err := NewOggVorbisEncoder(sampleRate, channels, quality)
	if err != nil {
		log.Fatalf("failed to create encoder: %v", err)
	}
	defer encoder.Close()

	buf := bytes.NewBuffer(nil)

	if err := encoder.WriteHeaders(buf); err != nil {
		log.Fatalf("failed to write headers to TCP stream: %v", err)
	}
	log.Println("Ogg Vorbis headers written to TCP stream.")

	func() { _ = encoder.EndStream(buf) }()
	func() { _, _ = buf.WriteTo(os.Stdout) }()

	for {
		select {
		case <-signalCh:
			return
		default:
		}

		var packetLength uint32
		if err := captureClient.GetNextPacketSize(&packetLength); err != nil {
			log.Fatal("GetNextPacketSize failed:", err)
		}
		if packetLength == 0 {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		var data *byte
		var numFrames uint32
		var flags uint32
		if err := captureClient.GetBuffer(&data, &numFrames, &flags, nil, nil); err != nil {
			log.Fatal("GetBuffer failed:", err)
		}

		if flags&wca.AUDCLNT_BUFFERFLAGS_SILENT == 0 {
			srcData := unsafe.Slice(data, numFrames*uint32(micFormat.NBlockAlign))

			// Calculate frames for output
			outputFrames := uint32(len(srcData)) / uint32(speakerFormat.NBlockAlign)

			var renderData *byte
			if err := renderClient.GetBuffer(outputFrames, &renderData); err != nil {
				log.Fatal("renderClient GetBuffer failed:", err)
			}

			copy(
				unsafe.Slice(renderData, uint32(len(srcData))),
				srcData,
			)

			if err := renderClient.ReleaseBuffer(outputFrames, 0); err != nil {
				log.Fatal("renderClient ReleaseBuffer failed:", err)
			}

			audioCh <- srcData

			// if err := encoder.EncodePCM(srcData, conn); err != nil {
			// 	log.Printf("failed to encode PCM chunk and write to TCP: %v", err)
			// 	break // Stop streaming on error
			// }
		}

		if err := captureClient.ReleaseBuffer(numFrames); err != nil {
			log.Fatal("captureClient ReleaseBuffer failed:", err)
		}
	}
}

// OggVorbisEncoder encapsulates the libvorbis and libogg states
type OggVorbisEncoder struct {
	vi C.vorbis_info
	vc C.vorbis_comment
	vd *C.vorbis_dsp_state
	vb *C.vorbis_block
	os *C.ogg_stream_state // Renamed to avoid conflict with standard library 'os'

	sampleRate int
	channels   int
	quality    float32   // 0.0 (low) to 1.0 (high)
	pcmBuffer  []float32 // Internal buffer for PCM data conversion
}

// NewOggVorbisEncoder initializes a new encoder.
// sampleRate: e.g., 44100
// channels: 1 for mono, 2 for stereo
// quality: 0.0 (lowest quality, smallest file) to 1.0 (highest quality, largest file)
func NewOggVorbisEncoder(sampleRate, channels int, quality float32) (*OggVorbisEncoder, error) {
	vd := (*C.vorbis_dsp_state)(C.malloc(C.size_t(unsafe.Sizeof(C.vorbis_dsp_state{}))))
	vb := (*C.vorbis_block)(C.malloc(C.size_t(unsafe.Sizeof(C.vorbis_block{}))))
	os := (*C.ogg_stream_state)(C.malloc(C.size_t(unsafe.Sizeof(C.ogg_stream_state{}))))

	enc := &OggVorbisEncoder{
		vd: vd,
		vb: vb,
		os: os,

		sampleRate: sampleRate,
		channels:   channels,
		quality:    quality,
	}

	C.vorbis_info_init(&enc.vi)
	C.vorbis_comment_init(&enc.vc)

	// // Set up vorbis_info for encoding (VBR)
	// // -1: VBR with nominal bitrate
	// // Quality is float from 0.0 to 1.0. libvorbis expects quality * 10.
	ret := C.vorbis_encode_init_vbr(&enc.vi, C.long(channels), C.long(sampleRate), C.float(quality))
	if ret != 0 {
		return nil, fmt.Errorf("vorbis_encode_init_vbr failed with code: %d", ret)
	}

	C.vorbis_comment_add_tag(&enc.vc, C.CString("ENCODER"), C.CString("Go CGo libvorbis Stream Encoder"))

	// // Initialize DSP state and block
	C.vorbis_analysis_init(enc.vd, &enc.vi)
	C.vorbis_block_init(enc.vd, enc.vb)

	// // Initialize Ogg stream
	C.srand(C.uint(C.time(nil)))
	C.ogg_stream_init(enc.os, C.rand())

	// // Pre-allocate a PCM buffer for float32 conversion
	// // A common frame size is 1024 or 2048 samples per channel.
	// // We'll use 4096 samples (2048 per channel for stereo) as a typical chunk size.
	// // This size can be adjusted based on latency requirements and CPU usage.
	enc.pcmBuffer = make([]float32, 4096*channels)

	return enc, nil
}

// Close frees all allocated C memory. This *must* be called when done.
func (enc *OggVorbisEncoder) Close() {
	defer C.free(unsafe.Pointer(enc.vd))
	defer C.free(unsafe.Pointer(enc.vb))
	defer C.free(unsafe.Pointer(enc.os))

	C.ogg_stream_clear(enc.os)
	C.vorbis_block_clear(enc.vb)
	C.vorbis_dsp_clear(enc.vd)
	C.vorbis_comment_clear(&enc.vc)
	C.vorbis_info_clear(&enc.vi)
	// log.Println("OggVorbisEncoder resources freed.")
}

// WriteHeaders writes the initial Ogg Vorbis headers to the output writer.
func (enc *OggVorbisEncoder) WriteHeaders(w io.Writer) error {
	var header, headerComm, headerCode C.ogg_packet

	// Generate Vorbis headers
	C.vorbis_analysis_headerout(enc.vd, &enc.vc, &header, &headerComm, &headerCode)

	// Submit headers to the Ogg stream
	C.ogg_stream_packetin(enc.os, &header)
	C.ogg_stream_packetin(enc.os, &headerComm)
	C.ogg_stream_packetin(enc.os, &headerCode)

	// Flush Ogg pages for headers
	for {
		var page C.ogg_page
		ret := C.ogg_stream_flush(enc.os, &page)
		if ret == 0 { // No more pages
			break
		}
		// Write the page header and body to the output
		// if _, err := w.Write(C.GoBytes(unsafe.Pointer(page.header), C.int(page.header_len))); err != nil {
		// 	return fmt.Errorf("failed to write Ogg header page header: %w", err)
		// }
		// if _, err := w.Write(C.GoBytes(unsafe.Pointer(page.body), C.int(page.body_len))); err != nil {
		// 	return fmt.Errorf("failed to write Ogg header page body: %w", err)
		// }

		// --- BEGIN FIX FOR PANIC ---
		// Copy header data to new C-allocated memory
		cHeaderCopy := C.copy_ogg_data(page.header, C.int(page.header_len))
		if cHeaderCopy == nil && page.header_len > 0 { // Check if malloc failed for non-zero length
			return fmt.Errorf("failed to allocate C memory for Ogg page header copy")
		}
		defer C.free(unsafe.Pointer(cHeaderCopy)) // IMPORTANT: Free the C-allocated copy!

		// Copy body data to new C-allocated memory
		cBodyCopy := C.copy_ogg_data(page.body, C.int(page.body_len))
		if cBodyCopy == nil && page.body_len > 0 { // Check if malloc failed for non-zero length
			return fmt.Errorf("failed to allocate C memory for Ogg page body copy")
		}
		defer C.free(unsafe.Pointer(cBodyCopy)) // IMPORTANT: Free the C-allocated copy!

		// Now use C.GoBytes on the *newly C-allocated* and controlled pointers
		if _, err := w.Write(C.GoBytes(unsafe.Pointer(cHeaderCopy), C.int(page.header_len))); err != nil {
			return fmt.Errorf("failed to write Ogg header page header: %w", err)
		}
		if _, err := w.Write(C.GoBytes(unsafe.Pointer(cBodyCopy), C.int(page.body_len))); err != nil {
			return fmt.Errorf("failed to write Ogg header page body: %w", err)
		}
		// --- END FIX FOR PANIC ---
	}
	return nil
}

// EncodePCM takes raw int16 PCM data, converts it, encodes it, and writes
// resulting Ogg Vorbis packets to the provided writer.
// Call this function repeatedly with chunks of PCM data.
func (enc *OggVorbisEncoder) EncodePCM(pcm []int16, w io.Writer) error {
	if len(pcm) == 0 {
		return nil
	}

	// Convert int16 PCM to float32 and normalize
	// Use the pre-allocated buffer to minimize allocations.
	// Ensure the pcmBuffer is large enough for the current PCM chunk.
	if cap(enc.pcmBuffer) < len(pcm) {
		enc.pcmBuffer = make([]float32, len(pcm))
	} else {
		enc.pcmBuffer = enc.pcmBuffer[:len(pcm)]
	}

	for i, sample := range pcm {
		enc.pcmBuffer[i] = float32(sample) / float32(math.MaxInt16) // Normalize to [-1.0, 1.0]
	}

	// Calculate number of samples *per channel*
	samplesPerChannel := len(enc.pcmBuffer) / enc.channels

	// Get Vorbis analysis buffer
	// `C.get_vorbis_analysis_buffer` is our CGo helper function.
	// It returns a `float**`. We need to carefully copy our Go slice into it.
	// The `buffer` variable points to an array of `float*`, one for each channel.
	buffer := C.get_vorbis_analysis_buffer(enc.vd, C.int(samplesPerChannel))

	// Copy data from our Go float32 slice to the C float** buffer
	// This assumes interleaved PCM (LRLR...)
	for ch := 0; ch < enc.channels; ch++ {
		// Get the pointer to the start of the current channel's buffer in C
		cChannelBufferPtr := (*C.float)(unsafe.Pointer(
			uintptr(unsafe.Pointer(buffer)) + uintptr(ch)*unsafe.Sizeof(uintptr(0)), // This gets the float* for the channel
		))

		// Copy samples for this channel
		for i := 0; i < samplesPerChannel; i++ {
			// Calculate index in the interleaved Go slice
			goSliceIndex := i*enc.channels + ch
			// Calculate address in the C channel buffer
			cTargetPtr := (*C.float)(unsafe.Pointer(uintptr(unsafe.Pointer(cChannelBufferPtr)) + uintptr(i)*unsafe.Sizeof(C.float(0))))

			*cTargetPtr = C.float(enc.pcmBuffer[goSliceIndex])
		}
	}

	// Notify Vorbis how many samples were written
	C.vorbis_analysis_wrote(enc.vd, C.int(samplesPerChannel))

	return enc.flushPackets(w)
}

// Flush any pending Vorbis packets and Ogg pages.
// This should be called after EncodePCM and at the end of the stream (with EndStream).
func (enc *OggVorbisEncoder) flushPackets(w io.Writer) error {
	var op C.ogg_packet // Vorbis packet
	var og C.ogg_page   // Ogg page

	// Process and get Vorbis packets
	for C.vorbis_analysis_blockout(enc.vd, enc.vb) == 1 {
		C.vorbis_analysis(enc.vb, nil)    // Perform encoding on the block
		C.vorbis_bitrate_addblock(enc.vb) // Add block to bitrate management

		// Get Ogg packets from the Vorbis encoder
		for C.vorbis_bitrate_flushpacket(enc.vd, &op) == 1 {
			// Submit Vorbis packet to Ogg stream
			C.ogg_stream_packetin(enc.os, &op)

			// Get Ogg pages from the Ogg stream
			for {
				ret := C.ogg_stream_pageout(enc.os, &og)
				if ret == 0 { // No more pages
					break
				}
				cHeaderCopy := C.copy_ogg_data(og.header, C.int(og.header_len))
				if cHeaderCopy == nil && og.header_len > 0 { // Check if malloc failed for non-zero length
					return fmt.Errorf("failed to allocate C memory for Ogg page header copy")
				}
				defer C.free(unsafe.Pointer(cHeaderCopy)) // IMPORTANT: Free the C-allocated copy!

				// Copy body data to new C-allocated memory
				cBodyCopy := C.copy_ogg_data(og.body, C.int(og.body_len))
				if cBodyCopy == nil && og.body_len > 0 { // Check if malloc failed for non-zero length
					return fmt.Errorf("failed to allocate C memory for Ogg page body copy")
				}
				defer C.free(unsafe.Pointer(cBodyCopy)) // IMPORTANT: Free the C-allocated copy!

				// Now use C.GoBytes on the *newly C-allocated* and controlled pointers
				if _, err := w.Write(C.GoBytes(unsafe.Pointer(cHeaderCopy), C.int(og.header_len))); err != nil {
					return fmt.Errorf("failed to write Ogg header page header: %w", err)
				}
				if _, err := w.Write(C.GoBytes(unsafe.Pointer(cBodyCopy), C.int(og.body_len))); err != nil {
					return fmt.Errorf("failed to write Ogg header page body: %w", err)
				}
				// Write the page header and body to the output
				// if _, err := w.Write(C.GoBytes(unsafe.Pointer(og.header), C.int(og.header_len))); err != nil {
				// 	return fmt.Errorf("failed to write Ogg page header: %w", err)
				// }
				// if _, err := w.Write(C.GoBytes(unsafe.Pointer(og.body), C.int(og.body_len))); err != nil {
				// 	return fmt.Errorf("failed to write Ogg page body: %w", err)
				// }
			}
		}
	}
	return nil
}

// EndStream signals the end of the PCM data and flushes any remaining buffered data.
func (enc *OggVorbisEncoder) EndStream(w io.Writer) error {
	// Signal end of stream by writing 0 samples
	C.vorbis_analysis_wrote(enc.vd, 0)
	return enc.flushPackets(w) // Flush all remaining data
}
