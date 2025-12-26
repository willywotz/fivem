package main

// import (
// 	"fmt"
// 	"syscall"
// 	"time"
// 	"unsafe"
// )

// var (
// 	user32   = syscall.NewLazyDLL("user32.dll")
// 	kernel32 = syscall.NewLazyDLL("kernel32.dll")

// 	// procEnumWindows              = user32.NewProc("EnumWindows")
// 	// procGetWindowTextW           = user32.NewProc("GetWindowTextW")
// 	// procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
// 	// procIsWindowVisible          = user32.NewProc("IsWindowVisible")

// 	procFindWindowW              = user32.NewProc("FindWindowW")
// 	procPostMessageW             = user32.NewProc("PostMessageW")
// 	procMapVirtualKeyW           = user32.NewProc("MapVirtualKeyW")
// 	procAttachThreadInput        = user32.NewProc("AttachThreadInput")
// 	procGetCurrentThreadId       = kernel32.NewProc("GetCurrentThreadId")
// 	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
// 	procSendInput                = user32.NewProc("SendInput")
// 	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
// )

// // type WindowInfo struct {
// // 	HWND    syscall.Handle
// // 	Title   string
// // 	Visible bool
// // }

// // var foundWindows []WindowInfo

// // func enumWindowsCallback(hwnd syscall.Handle, lParam uintptr) uintptr {
// // 	// Check if the window is visible (optional, remove if you want all windows)
// // 	ret, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
// // 	isVisible := ret != 0

// // 	// Get window title length
// // 	titleLen, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
// // 	if titleLen == 0 {
// // 		// No title or error, skip
// // 		return 1 // Continue enumeration
// // 	}

// // 	// Allocate a buffer for the title
// // 	buf := make([]uint16, titleLen+1) // +1 for null terminator
// // 	_, _, _ = procGetWindowTextW.Call(
// // 		uintptr(hwnd),
// // 		uintptr(unsafe.Pointer(&buf[0])),
// // 		uintptr(titleLen+1),
// // 	)

// // 	title := syscall.UTF16ToString(buf)

// // 	// Store the information
// // 	foundWindows = append(foundWindows, WindowInfo{
// // 		HWND:    hwnd,
// // 		Title:   title,
// // 		Visible: isVisible,
// // 	})

// // 	return 1 // Return TRUE (1) to continue enumeration
// // }

// func PostMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) error {
// 	ret, _, err := procPostMessageW.Call(hwnd, uintptr(msg), wParam, lParam)
// 	if ret == 0 {
// 		return err
// 	}
// 	return nil
// }

// func SetForegroundWindow(hwnd uintptr) error {
// 	ret, _, err := procSetForegroundWindow.Call(hwnd)
// 	if ret == 0 {
// 		return err
// 	}
// 	return nil
// }

// func utf16PtrFromString(s string) *uint16 {
// 	ptr, _ := syscall.UTF16PtrFromString(s)
// 	return ptr
// }

// // // Input types
// // const (
// // 	INPUT_MOUSE    = 0
// // 	INPUT_KEYBOARD = 1
// // 	INPUT_HARDWARE = 2

// // 	MAPVK_VK_TO_VSC    = 0 // Map virtual key to scan code
// // 	MAPVK_VSC_TO_VK    = 1 // Map scan code to virtual key
// // 	MAPVK_VK_TO_CHAR   = 2 // Map virtual key to character
// // 	MAPVK_VSC_TO_VK_EX = 3 // Map extended scan code to virtual key
// // 	MAPVK_VK_TO_VSC_EX = 4 // Map virtual key to extended scan code
// // )

// // // Keyboard event flags
// // const (
// // 	KEYEVENTF_EXTENDEDKEY = 0x0001
// // 	KEYEVENTF_KEYUP       = 0x0002
// // 	KEYEVENTF_UNICODE     = 0x0004 // Important for sending characters directly
// // 	KEYEVENTF_SCANCODE    = 0x0008
// // )

// // // Virtual key codes (for specific keys)
// // const (
// // 	VK_RETURN = 0x0D // Enter key
// // 	VK_SHIFT  = 0x10 // Shift key
// // 	VK_E      = 0x45 // Virtual key code for 'E'
// // )

// // // --- Corrected Structures for SendInput ---

// // // MOUSEINPUT and KEYBDINPUT: DwExtraInfo is now uint32 to match how
// // // golang.org/x/sys/windows defines it for correct sizing in INPUT union.
// // type MOUSEINPUT struct {
// // 	Dx          int32
// // 	Dy          int32
// // 	MouseData   uint32
// // 	DwFlags     uint32
// // 	Time        uint32
// // 	DwExtraInfo uint32 // Changed from uintptr to uint32
// // } // Size: 4*5 + 4 = 24 bytes

// // type KEYBDINPUT struct {
// // 	WVk         uint16
// // 	WScan       uint16
// // 	DwFlags     uint32
// // 	Time        uint32
// // 	DwExtraInfo uint32 // Changed from uintptr to uint32
// // } // Size: 2+2+4+4+4 = 16 bytes

// // type HARDWAREINPUT struct {
// // 	Umsg    uint32
// // 	WparamL uint16
// // 	WparamH uint16
// // } // Size: 8 bytes

// // // INPUT structure for SendInput
// // // This structure needs to be exactly 32 bytes on a 64-bit system.
// // // Type (4 bytes) + padding (4 bytes added by Go) + Data (24 bytes, max of KEYBDINPUT/MOUSEINPUT) = 32 bytes.
// // type INPUT struct {
// // 	Type uint32
// // 	// Go will add 4 bytes of padding here to align 'Data' to an 8-byte boundary.
// // 	// This padding is implicit and makes the struct size correct for the API.
// // 	Data [24]byte // This slice represents the union's memory, max size of MOUSEINPUT (24 bytes)
// // }

// // // sendInputWrapper is a helper to call the SendInput API and check its return
// // func sendInputWrapper(inputs []INPUT) {
// // 	// Debug print to confirm size
// // 	fmt.Printf("Size of INPUT struct: %d bytes\n", unsafe.Sizeof(inputs[0]))

// // 	ret, _, err := procSendInput.Call(
// // 		uintptr(len(inputs)),
// // 		uintptr(unsafe.Pointer(&inputs[0])),
// // 		uintptr(unsafe.Sizeof(inputs[0])), // This 'cbSize' must be exactly 32 on 64-bit Windows
// // 	)
// // 	if ret != uintptr(len(inputs)) {
// // 		fmt.Printf("SendInput failed to send all events (sent %d, expected %d): %v\n", ret, len(inputs), err)
// // 	}
// // }

// // // typeCharacter sends a single character (rune) using KEYEVENTF_UNICODE
// // func typeCharacter(r rune) {
// // 	// Create an INPUT struct
// // 	inputDown := INPUT{Type: INPUT_KEYBOARD}
// // 	// Populate its KEYBDINPUT part by casting Data to *KEYBDINPUT
// // 	kbInputDown := (*KEYBDINPUT)(unsafe.Pointer(&inputDown.Data[0]))
// // 	kbInputDown.WScan = uint16(r) // wScan holds the Unicode char
// // 	kbInputDown.DwFlags = KEYEVENTF_UNICODE

// // 	// Create another INPUT struct for key up
// // 	inputUp := INPUT{Type: INPUT_KEYBOARD}
// // 	kbInputUp := (*KEYBDINPUT)(unsafe.Pointer(&inputUp.Data[0]))
// // 	kbInputUp.WScan = uint16(r)
// // 	kbInputUp.DwFlags = KEYEVENTF_UNICODE | KEYEVENTF_KEYUP

// // 	inputs := []INPUT{inputDown, inputUp}
// // 	sendInputWrapper(inputs)
// // 	time.Sleep(50 * time.Millisecond) // Small delay between characters
// // }

// // // sendVirtualKeyPress sends a key down and key up event for a given virtual key code.
// // func sendVirtualKeyPress(vk uint16) {
// // 	inputDown := INPUT{Type: INPUT_KEYBOARD}
// // 	kbInputDown := (*KEYBDINPUT)(unsafe.Pointer(&inputDown.Data[0]))
// // 	kbInputDown.WVk = vk
// // 	kbInputDown.DwFlags = 0 // Key down

// // 	inputUp := INPUT{Type: INPUT_KEYBOARD}
// // 	kbInputUp := (*KEYBDINPUT)(unsafe.Pointer(&inputUp.Data[0]))
// // 	kbInputUp.WVk = vk
// // 	kbInputUp.DwFlags = KEYEVENTF_KEYUP // Key up

// // 	inputs := []INPUT{inputDown, inputUp}
// // 	sendInputWrapper(inputs)
// // 	time.Sleep(50 * time.Millisecond) // Small delay
// // }

// // func test(sendInputProc *syscall.LazyProc) {
// // 	type keyboardInput struct {
// // 		wVk         uint16
// // 		wScan       uint16
// // 		dwFlags     uint32
// // 		time        uint32
// // 		dwExtraInfo uint64
// // 	}

// // 	type input struct {
// // 		inputType uint32
// // 		ki        keyboardInput
// // 		padding   uint64
// // 	}

// // 	var i input
// // 	i.inputType = 1 // INPUT_KEYBOARD
// // 	i.ki.wVk = 0x45 // virtual key code for e
// // 	ret, _, err := sendInputProc.Call(
// // 		uintptr(1),
// // 		uintptr(unsafe.Pointer(&i)),
// // 		uintptr(unsafe.Sizeof(i)),
// // 	)
// // 	log.Printf("ret: %v error: %v", ret, err)
// // }

// // func run() {
// // 	className := "grcWindow"
// // 	title := "FiveM® by Cfx.re - Fantastic Projects"

// // 	hwnd, _, _ := procFindWindowW.Call(
// // 		uintptr(unsafe.Pointer(utf16PtrFromString(className))),
// // 		uintptr(unsafe.Pointer(utf16PtrFromString(title))),
// // 	)

// // 	if hwnd == 0 {
// // 		log.Fatalf("Window not found: class=%s, title=%s", className, title)
// // 	}

// // 	_ = SetForegroundWindow(hwnd)

// // 	time.Sleep(500 * time.Millisecond)

// // 	log.Printf("Found window: HWND=0x%X, class=%s, title=%s", hwnd, className, title)

// // 	// targetThreadId, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
// // 	// currentThreadId, _, _ := procGetCurrentThreadId.Call()

// // 	// _, _, _ = procAttachThreadInput.Call(currentThreadId, targetThreadId, 1)
// // 	// defer func() {
// // 	// 	_, _, _ = procAttachThreadInput.Call(currentThreadId, targetThreadId, 0)
// // 	// }()

// // 	test(procSendInput)
// // }

// // --- Windows API Constants ---
// // Input Types
// const (
// 	INPUT_MOUSE    uint32 = 0
// 	INPUT_KEYBOARD uint32 = 1
// 	INPUT_HARDWARE uint32 = 2
// )

// // Keyboard Event Flags
// const (
// 	KEYEVENTF_EXTENDEDKEY uint32 = 0x0001
// 	KEYEVENTF_KEYUP       uint32 = 0x0002
// 	KEYEVENTF_UNICODE     uint32 = 0x0004 // For sending Unicode characters directly
// 	KEYEVENTF_SCANCODE    uint32 = 0x0008
// )

// // Virtual Key Codes (common ones, based on winuser.h)
// const (
// 	VK_LBUTTON  uint16 = 0x01 // Left mouse button
// 	VK_RBUTTON  uint16 = 0x02 // Right mouse button
// 	VK_CANCEL   uint16 = 0x03 // Control-break processing
// 	VK_MBUTTON  uint16 = 0x04 // Middle mouse button (three-button mouse)
// 	VK_XBUTTON1 uint16 = 0x05 // X1 mouse button
// 	VK_XBUTTON2 uint16 = 0x06 // X2 mouse button
// 	VK_BACK     uint16 = 0x08 // BACKSPACE key
// 	VK_TAB      uint16 = 0x09 // TAB key
// 	VK_CLEAR    uint16 = 0x0C // CLEAR key
// 	VK_RETURN   uint16 = 0x0D // ENTER key
// 	VK_SHIFT    uint16 = 0x10 // SHIFT key
// 	VK_CONTROL  uint16 = 0x11 // CTRL key
// 	VK_MENU     uint16 = 0x12 // ALT key
// 	VK_PAUSE    uint16 = 0x13 // PAUSE key
// 	VK_CAPITAL  uint16 = 0x14 // CAPS LOCK key
// 	VK_ESCAPE   uint16 = 0x1B // ESC key
// 	VK_SPACE    uint16 = 0x20 // SPACEBAR
// 	VK_PRIOR    uint16 = 0x21 // PAGE UP key
// 	VK_NEXT     uint16 = 0x22 // PAGE DOWN key
// 	VK_END      uint16 = 0x23 // END key
// 	VK_HOME     uint16 = 0x24 // HOME key
// 	VK_LEFT     uint16 = 0x25 // LEFT ARROW key
// 	VK_UP       uint16 = 0x26 // UP ARROW key
// 	VK_RIGHT    uint16 = 0x27 // RIGHT ARROW key
// 	VK_DOWN     uint16 = 0x28 // DOWN ARROW key
// 	VK_SELECT   uint16 = 0x29 // SELECT key
// 	VK_PRINT    uint16 = 0x2A // PRINT key
// 	VK_EXECUTE  uint16 = 0x2B // EXECUTE key
// 	VK_SNAPSHOT uint16 = 0x2C // PRINT SCREEN key
// 	VK_INSERT   uint16 = 0x2D // INS key
// 	VK_DELETE   uint16 = 0x2E // DEL key
// 	VK_HELP     uint16 = 0x2F // HELP key

// 	VK_0 uint16 = 0x30
// 	VK_1 uint16 = 0x31
// 	VK_2 uint16 = 0x32
// 	VK_3 uint16 = 0x33
// 	VK_4 uint16 = 0x34
// 	VK_5 uint16 = 0x35
// 	VK_6 uint16 = 0x36
// 	VK_7 uint16 = 0x37
// 	VK_8 uint16 = 0x38
// 	VK_9 uint16 = 0x39

// 	VK_A uint16 = 0x41
// 	VK_B uint16 = 0x42
// 	VK_C uint16 = 0x43
// 	VK_D uint16 = 0x44
// 	VK_E uint16 = 0x45
// 	VK_F uint16 = 0x46
// 	VK_G uint16 = 0x47
// 	VK_H uint16 = 0x48
// 	VK_I uint16 = 0x49
// 	VK_J uint16 = 0x4A
// 	VK_K uint16 = 0x4B
// 	VK_L uint16 = 0x4C
// 	VK_M uint16 = 0x4D
// 	VK_N uint16 = 0x4E
// 	VK_O uint16 = 0x4F
// 	VK_P uint16 = 0x50
// 	VK_Q uint16 = 0x51
// 	VK_R uint16 = 0x52
// 	VK_S uint16 = 0x53
// 	VK_T uint16 = 0x54
// 	VK_U uint16 = 0x55
// 	VK_V uint16 = 0x56
// 	VK_W uint16 = 0x57
// 	VK_X uint16 = 0x58
// 	VK_Y uint16 = 0x59
// 	VK_Z uint16 = 0x5A
// )

// // --- Windows API Structures (Manually Defined) ---

// // KEYBDINPUT structure (matches C definition)
// type KEYBDINPUT struct {
// 	WVk         uint16
// 	WScan       uint16
// 	DwFlags     uint32
// 	Time        uint32
// 	DwExtraInfo uintptr
// }

// // MOUSEINPUT structure (included for completeness for the INPUT union, though not used here)
// type MOUSEINPUT struct {
// 	Dx          int32
// 	Dy          int32
// 	MouseData   uint32
// 	DwFlags     uint32
// 	Time        uint32
// 	DwExtraInfo uintptr
// }

// // HARDWAREINPUT structure (included for completeness for the INPUT union, though not used here)
// type HARDWAREINPUT struct {
// 	Umsg    uint32
// 	LParamL uint16
// 	LParamH uint16
// }

// // INPUT structure - carefully constructed to match the C LAYOUT for SendInput
// // This struct accounts for the 'Type' field, followed by 4 bytes of padding (on 64-bit),
// // and then a union that is 28 bytes (the size of the largest member, MOUSEINPUT).
// // Total size: 4 (Type) + 4 (padding) + 28 (union) = 36 bytes.
// // Windows `sizeof(INPUT)` is 40 bytes on 64-bit, meaning there's another 4 bytes of padding
// // at the end to make it a multiple of 8 for alignment.
// type INPUT struct {
// 	Type uint32
// 	_    [4]byte // Padding to align the union on 64-bit systems

// 	// The `DUMMYUNIONNAME` field is an anonymous struct that occupies the same memory
// 	// as the union in the C `INPUT` struct. It must be large enough to hold the largest
// 	// member of the union (MOUSEINPUT is 28 bytes on 64-bit).
// 	DUMMYUNIONNAME struct {
// 		_ [28]byte // This byte array acts as the memory space for the union
// 	}
// 	_ [4]byte // Additional padding to make total struct size 40 bytes (matching Windows API)
// }

// // sendKeyInput is a helper function to send a single keyboard input event.
// // vkCode: Virtual-key code (e.g., VK_A, VK_RETURN). Use 0 if using scanCode with KEYEVENTF_UNICODE.
// // scanCode: Hardware scan code for the key. Use 0 if using vkCode. For KEYEVENTF_UNICODE, this is the character.
// // flags: Flags that specify various aspects of function operation (e.g., KEYEVENTF_KEYUP).
// func sendKeyInput(vkCode uint16, scanCode uint16, flags uint32) {
// 	var input INPUT
// 	input.Type = INPUT_KEYBOARD // Specify that this is a keyboard input event

// 	// Manually place the KEYBDINPUT data into the union's memory space.
// 	// We get a pointer to the start of the union's memory (`&input.DUMMYUNIONNAME`),
// 	// then cast it to a pointer of KEYBDINPUT, and dereference it to assign values.
// 	kbInput := (*KEYBDINPUT)(unsafe.Pointer(&input.DUMMYUNIONNAME))
// 	kbInput.WVk = vkCode
// 	kbInput.WScan = scanCode
// 	kbInput.DwFlags = flags
// 	kbInput.Time = 0        // System will provide the timestamp
// 	kbInput.DwExtraInfo = 0 // Additional data associated with the input

// 	nInputs := uintptr(1)
// 	sizeOfInput := uintptr(unsafe.Sizeof(input))

// 	ret, _, err := syscall.SyscallN(
// 		procSendInput.Addr(),
// 		nInputs,                         // cInputs
// 		uintptr(unsafe.Pointer(&input)), // pInputs
// 		sizeOfInput,                     // cbSize
// 		0, 0, 0,                         // Padding to 6 arguments for SyscallN
// 	)

// 	if ret != nInputs {
// 		fmt.Printf("Error: SendInput failed to send all inputs. Sent: %d, Expected: %d, Error: %v\n", ret, nInputs, err)
// 	}
// }

// // PressAndRelease simulates pressing and then releasing a keyboard key.
// // It takes a virtual key code (e.g., VK_A for 'A').
// func PressAndRelease(vkCode uint16) {
// 	fmt.Printf("Pressing key with VK_CODE: 0x%X\n", vkCode)

// 	// Press the key down
// 	sendKeyInput(vkCode, 0, 0) // Flags 0 means Key Down

// 	// Add a small delay, similar to pynput's time.sleep(0.1)
// 	time.Sleep(100 * time.Millisecond) // 0.1 seconds

// 	// Release the key
// 	sendKeyInput(vkCode, 0, KEYEVENTF_KEYUP) // KEYEVENTF_KEYUP for Key Up
// 	fmt.Printf("Released key with VK_CODE: 0x%X\n", vkCode)
// }

// func sendKeyInputWithScanCode(scanCode uint16) {
// 	fmt.Printf("Pressing key with SCAN_CODE: 0x%X\n", scanCode)

// 	sendKeyInput(0, scanCode, KEYEVENTF_SCANCODE)

// 	time.Sleep(100 * time.Millisecond) // 0.1 seconds

// 	sendKeyInput(0, scanCode, KEYEVENTF_SCANCODE|KEYEVENTF_KEYUP) // KEYEVENTF_KEYUP for Key Up

// 	fmt.Printf("Released key with SCAN_CODE: 0x%X\n", scanCode)
// }

// // PressAndReleaseChar simulates typing a single character.
// // This uses KEYEVENTF_UNICODE flag, which is generally simpler for characters
// // as it doesn't require mapping to virtual key codes or handling Shift state.
// func PressAndReleaseChar(char rune) {
// 	fmt.Printf("Typing character: '%c' (Unicode: %U)\n", char, char)

// 	// Key Down with UNICODE flag (Vk is 0, Scan is the Unicode char)
// 	sendKeyInput(0, uint16(char), KEYEVENTF_UNICODE)

// 	time.Sleep(100 * time.Millisecond) // 0.1 seconds

// 	// Key Up with UNICODE and KEYUP flags
// 	sendKeyInput(0, uint16(char), KEYEVENTF_UNICODE|KEYEVENTF_KEYUP)
// }

// func run() {
// 	// className := "grcWindow"
// 	// title := "FiveM® by Cfx.re - Fantastic Projects"

// 	// className := "Cfx_GlobalInputWindow"
// 	// title := "CitizenFX Global Input Window"

// 	// hwnd, _, _ := procFindWindowW.Call(
// 	// 	uintptr(unsafe.Pointer(utf16PtrFromString(className))),
// 	// 	uintptr(unsafe.Pointer(utf16PtrFromString(title))),
// 	// )

// 	// if hwnd == 0 {
// 	// 	log.Fatalf("Window not found: class=%s, title=%s", className, title)
// 	// }

// 	// _ = SetForegroundWindow(hwnd)

// 	time.Sleep(2000 * time.Millisecond)

// 	// log.Printf("Found window: HWND=0x%X, class=%s, title=%s", hwnd, className, title)

// 	// targetThreadId, _, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
// 	// currentThreadId, _, _ := procGetCurrentThreadId.Call()

// 	// _, _, _ = procAttachThreadInput.Call(currentThreadId, targetThreadId, 1)
// 	// defer func() {
// 	// 	_, _, _ = procAttachThreadInput.Call(currentThreadId, targetThreadId, 0)
// 	// }()

// 	scanCodeVal, _, _ := procMapVirtualKeyW.Call(uintptr(VK_E), uintptr(0))
// 	scanCode := uint16(scanCodeVal)

// 	sendKeyInputWithScanCode(scanCode) // Press and release the 'E' key
// }

// func main() {
// 	run()

// 	// type keyboardInput struct {
// 	// 	wVk         uint16
// 	// 	wScan       uint16
// 	// 	dwFlags     uint32
// 	// 	time        uint32
// 	// 	dwExtraInfo uint64
// 	// }

// 	// type input struct {
// 	// 	inputType uint32
// 	// 	ki        keyboardInput
// 	// 	padding   uint64
// 	// }

// 	// scanCodeVal, _, _ := procMapVirtualKeyW.Call(uintptr(VK_E), uintptr(MAPVK_VK_TO_VSC))
// 	// scanCode := uint16(scanCodeVal)

// 	// a := make([]input, 0)
// 	// a = append(a, input{INPUT_KEYBOARD, keyboardInput{wScan: scanCode, dwFlags: KEYEVENTF_SCANCODE}, 0})
// 	// a = append(a, input{INPUT_KEYBOARD, keyboardInput{wScan: scanCode, dwFlags: KEYEVENTF_KEYUP | KEYEVENTF_SCANCODE}, 0})
// 	// // a = append(a, input{INPUT_KEYBOARD, keyboardInput{wVk: 0x45, dwFlags: 0, dwExtraInfo: 0}, 0})
// 	// // a = append(a, input{INPUT_KEYBOARD, keyboardInput{wVk: 0x45, dwFlags: KEYEVENTF_KEYUP, dwExtraInfo: 0}, 0})
// 	// log.Println(procSendInput.Call(uintptr(len(a)), uintptr(unsafe.Pointer(&a[0])), uintptr(unsafe.Sizeof(a[0]))))

// 	// sendVirtualKeyPress(VK_E) // Send the 'E' key down and up events

// 	// wParam := uintptr(VK_E) // Virtual key code

// 	// typeCharacter('e') // Send the character 'E'

// 	// test(procSendInput)

// 	// mainthread.Run(run)

// 	// robotgo.SetActiveWindow(hwnd)
// 	// robotgo.MilliSleep(500)
// 	// robotgo.TypeStr("e") // Send the character 'e'

// 	// lParam := uintptr((scanCode << 16) | 1) // Repeat count 1, rest 0s for keydown
// 	// lParamKeyUp := uintptr((scanCode << 16) | (1 << 31) | (1 << 30) | 1) // Set transition and previous state for keyup

// 	// PostMessage(hwnd, WM_CHAR, wParam, lParam)
// 	// time.Sleep(3 * time.Millisecond)
// 	// PostMessage(hwnd, WM_KEYUP, wParam, lParamKeyUp)

// 	// fmt.Println("Enumerating top-level windows...")

// 	// // Create a callback pointer for our Go function
// 	// callback := syscall.NewCallback(enumWindowsCallback)

// 	// // Call EnumWindows. The second parameter (lParam) is 0 as we're using a global slice.
// 	// // If you wanted to pass data to the callback, you'd put a uintptr representation of it here.
// 	// ret, _, err := procEnumWindows.Call(callback, 0)

// 	// if ret == 0 {
// 	// 	// EnumWindows returns 0 if it fails or if the callback returns FALSE.
// 	// 	// If the callback returns FALSE, GetLastError is not set by EnumWindows itself.
// 	// 	// So, if we stopped enumeration explicitly (e.g., found a window), this is expected.
// 	// 	// If it's a true API error, err will be non-nil.
// 	// 	if err != nil && err.(syscall.Errno) != 0 { // Check if it's a real error
// 	// 		log.Fatalf("EnumWindows failed: %v", err)
// 	// 	} else {
// 	// 		log.Println("Enumeration stopped by callback or completed.")
// 	// 	}
// 	// } else {
// 	// 	log.Println("EnumWindows completed successfully.")
// 	// }

// 	// log.Println("\n--- Found Windows ---")
// 	// for _, win := range foundWindows {
// 	// 	if win.Visible && win.Title != "" { // Only print visible windows with titles
// 	// 		fmt.Printf("HWND: 0x%X, Title: \"%s\"\n", win.HWND, win.Title)
// 	// 	}
// 	// }

// 	// // Example: Find a specific window (e.g., Notepad if it's open)
// 	// log.Println("\n--- Searching for Notepad ---")
// 	// var notepadHWND syscall.Handle
// 	// searchCallback := syscall.NewCallback(func(hwnd syscall.Handle, lParam uintptr) uintptr {
// 	// 	// Get window title
// 	// 	titleLen, _, _ := procGetWindowTextLengthW.Call(uintptr(hwnd))
// 	// 	if titleLen == 0 {
// 	// 		return 1 // Continue
// 	// 	}
// 	// 	buf := make([]uint16, titleLen+1)
// 	// 	_, _, _ = procGetWindowTextW.Call(
// 	// 		uintptr(hwnd),
// 	// 		uintptr(unsafe.Pointer(&buf[0])),
// 	// 		uintptr(titleLen+1),
// 	// 	)
// 	// 	title := syscall.UTF16ToString(buf)

// 	// 	// Check if it's Notepad (case-insensitive for robustness)
// 	// 	if procIsWindowVisible.Call(uintptr(hwnd)) != 0 && // Check visibility
// 	// 		len(title) >= len("Notepad") &&
// 	// 		title[len(title)-len("Notepad"):] == "Notepad" { // Simple check for ending with "Notepad"
// 	// 		notepadHWND = hwnd // Store the handle
// 	// 		return 0           // Stop enumeration (found it)
// 	// 	}
// 	// 	return 1 // Continue enumeration
// 	// })

// 	// procEnumWindows.Call(searchCallback, 0)

// 	// if notepadHWND != 0 {
// 	// 	fmt.Printf("Found Notepad window. HWND: 0x%X\n", notepadHWND)
// 	// } else {
// 	// 	fmt.Println("Notepad window not found.")
// 	// }
// }

// // func opencv() {
// // 	name := "test.png"
// // 	name1 := "test_001.png"
// // 	robotgo.SaveCapture(name1, 10, 10, 30, 30)
// // 	robotgo.SaveCapture(name)

// // 	fmt.Print("gcv find image: ")
// // 	fmt.Println(gcv.FindImgFile(name1, name))
// // 	fmt.Println(gcv.FindAllImgFile(name1, name))

// // 	bit := bitmap.Open(name1)
// // 	defer robotgo.FreeBitmap(bit)
// // 	fmt.Print("find bitmap: ")
// // 	fmt.Println(bitmap.Find(bit))

// // 	// bit0 := robotgo.CaptureScreen()
// // 	// img := robotgo.ToImage(bit0)
// // 	// bit1 := robotgo.CaptureScreen(10, 10, 30, 30)
// // 	// img1 := robotgo.ToImage(bit1)
// // 	// defer robotgo.FreeBitmapArr(bit0, bit1)
// // 	img, _ := robotgo.CaptureImg()
// // 	img1, _ := robotgo.CaptureImg(10, 10, 30, 30)

// // 	fmt.Print("gcv find image: ")
// // 	fmt.Println(gcv.FindImg(img1, img))
// // 	fmt.Println()

// // 	res := gcv.FindAllImg(img1, img)
// // 	fmt.Println(res[0].TopLeft.Y, res[0].Rects.TopLeft.X, res)
// // 	x, y := res[0].TopLeft.X, res[0].TopLeft.Y
// // 	robotgo.Move(x, y-rand.Intn(5))
// // 	robotgo.MilliSleep(100)
// // 	robotgo.Click()

// // 	res = gcv.FindAll(img1, img) // use find template and sift
// // 	fmt.Println("find all: ", res)
// // 	res1 := gcv.Find(img1, img)
// // 	fmt.Println("find: ", res1)

// // 	img2, _, _ := robotgo.DecodeImg("test_001.png")
// // 	x, y = gcv.FindX(img2, img)
// // 	fmt.Println(x, y)
// // }
