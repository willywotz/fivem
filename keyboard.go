package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

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
