package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	comctl32 = windows.NewLazySystemDLL("comctl32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procShowWindow          = user32.NewProc("ShowWindow")
	procUpdateWindow        = user32.NewProc("UpdateWindow")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostQuitMessage     = user32.NewProc("PostQuitMessage")
	procLoadCursorW         = user32.NewProc("LoadCursorW")
	procSetWindowTextW      = user32.NewProc("SetWindowTextW")
	procSendMessageW        = user32.NewProc("SendMessageW")
	procGetClientRect       = user32.NewProc("GetClientRect")
	procMoveWindow          = user32.NewProc("MoveWindow")
	procGetDlgItem          = user32.NewProc("GetDlgItem")
	procBeginPaint          = user32.NewProc("BeginPaint")
	procEndPaint            = user32.NewProc("EndPaint")
	procFillRect            = user32.NewProc("FillRect")
	procCreateSolidBrush    = gdi32.NewProc("CreateSolidBrush")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procCreateFontIndirectW = gdi32.NewProc("CreateFontIndirectW")
	procGetStockObject      = gdi32.NewProc("GetStockObject")

	procInitCommonControlsEx = comctl32.NewProc("InitCommonControlsEx")
)

const (
	// Window styles
	WS_OVERLAPPEDWINDOW = 0x00CF0000
	WS_VISIBLE          = 0x10000000
	WS_CHILD            = 0x40000000
	WS_CLIPSIBLINGS     = 0x04000000

	WS_EX_CLIENTEDGE = 0x00000200

	// Common controls / trackbar
	ICC_BAR_CLASSES = 0x00000004

	// Messages
	WM_CREATE    = 0x0001
	WM_DESTROY   = 0x0002
	WM_SIZE      = 0x0005
	WM_PAINT     = 0x000F
	WM_COMMAND   = 0x0111
	WM_HSCROLL   = 0x0114
	WM_SETFONT   = 0x0030
	WM_GETFONT   = 0x0031
	CB_ADDSTRING = 0x0143
	CB_SETCURSEL = 0x014E

	// Trackbar messages
	TBM_SETRANGE  = 0x0400 + 6
	TBM_SETPOS    = 0x0400 + 5
	TBM_GETPOS    = 0x0400
	TBS_AUTOTICKS = 0x0001
	TBS_HORZ      = 0x0000

	// Control classes
	WC_STATIC      = "STATIC"
	WC_BUTTON      = "BUTTON"
	WC_COMBOBOX    = "COMBOBOX"
	TRACKBAR_CLASS = "msctls_trackbar32"

	// Static styles
	SS_LEFT  = 0x00000000
	SS_RIGHT = 0x00000002

	// Combo styles
	CBS_DROPDOWNLIST = 0x0003
	WS_VSCROLL       = 0x00200000

	// Others
	SW_SHOW = 5

	IDC_ARROW = 32512

	// Control IDs
	ID_TITLE     = 101
	ID_VERSION   = 102
	ID_SEP       = 103
	ID_MIC_LABEL = 104
	ID_MIC_COMBO = 105
	ID_VOL_LABEL = 106
	ID_VOL_TRACK = 107

	// Stock objects
	WHITE_BRUSH = 0
)

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     windows.Handle
	HIcon         windows.Handle
	HCursor       windows.Handle
	HbrBackground windows.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       windows.Handle
}

type MSG struct {
	HWnd    windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type RECT struct {
	Left, Top, Right, Bottom int32
}

type PAINTSTRUCT struct {
	Hdc         windows.Handle
	FErase      int32
	RcPaint     RECT
	FRestore    int32
	FIncUpdate  int32
	RgbReserved [32]byte
}

type INITCOMMONCONTROLSEX struct {
	DwSize uint32
	DwICC  uint32
}

type LOGFONTW struct {
	LfHeight         int32
	LfWidth          int32
	LfEscapement     int32
	LfOrientation    int32
	LfWeight         int32
	LfItalic         byte
	LfUnderline      byte
	LfStrikeOut      byte
	LfCharSet        byte
	LfOutPrecision   byte
	LfClipPrecision  byte
	LfQuality        byte
	LfPitchAndFamily byte
	LfFaceName       [32]uint16
}

var (
	hInst windows.Handle
	hFont windows.Handle
)

func wstr(s string) *uint16 { return windows.StringToUTF16Ptr(s) }

func mustOK(r1 uintptr, name string) {
	if r1 == 0 {
		panic(name + " failed")
	}
}

func initCommonControls() {
	var icc INITCOMMONCONTROLSEX
	icc.DwSize = uint32(unsafe.Sizeof(icc))
	icc.DwICC = ICC_BAR_CLASSES
	r1, _, _ := procInitCommonControlsEx.Call(uintptr(unsafe.Pointer(&icc)))
	mustOK(r1, "InitCommonControlsEx")
}

func createUIFont() windows.Handle {
	// Simple Segoe UI 18 (title looks nicer). You can change size/name.
	var lf LOGFONTW
	lf.LfHeight = -18 // negative = character height in logical units
	lf.LfWeight = 700 // bold
	copy(lf.LfFaceName[:], windows.StringToUTF16("Segoe UI"))

	r1, _, _ := procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&lf)))
	if r1 == 0 {
		// fallback to DEFAULT_GUI_FONT
		r2, _, _ := procGetStockObject.Call(uintptr(17)) // DEFAULT_GUI_FONT = 17
		return windows.Handle(r2)
	}
	return windows.Handle(r1)
}

func setFont(hwnd windows.Handle, font windows.Handle) {
	procSendMessageW.Call(uintptr(hwnd), WM_SETFONT, uintptr(font), 1)
}

func sendText(hwnd windows.Handle, text string) {
	procSetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(wstr(text))))
}

func createControl(exStyle uint32, class, text string, style uint32, x, y, w, h int32, parent windows.Handle, id int) windows.Handle {
	hwnd, _, _ := procCreateWindowExW.Call(
		uintptr(exStyle),
		uintptr(unsafe.Pointer(wstr(class))),
		uintptr(unsafe.Pointer(wstr(text))),
		uintptr(style),
		uintptr(x), uintptr(y), uintptr(w), uintptr(h),
		uintptr(parent),
		uintptr(id),
		uintptr(hInst),
		0,
	)
	if hwnd == 0 {
		panic("CreateWindowExW failed for " + class)
	}
	return windows.Handle(hwnd)
}

func getClientRect(hwnd windows.Handle) RECT {
	var rc RECT
	procGetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&rc)))
	return rc
}

func move(hwnd windows.Handle, x, y, w, h int32) {
	procMoveWindow.Call(uintptr(hwnd), uintptr(x), uintptr(y), uintptr(w), uintptr(h), 1)
}

func getDlgItem(hwnd windows.Handle, id int) windows.Handle {
	h, _, _ := procGetDlgItem.Call(uintptr(hwnd), uintptr(id))
	return windows.Handle(h)
}

func trackbarSetRange(hwnd windows.Handle, min, max int) {
	// wParam TRUE = redraw
	procSendMessageW.Call(uintptr(hwnd), TBM_SETRANGE, 1, uintptr((max<<16)|min))
}

func trackbarSetPos(hwnd windows.Handle, pos int) {
	procSendMessageW.Call(uintptr(hwnd), TBM_SETPOS, 1, uintptr(pos))
}

func trackbarGetPos(hwnd windows.Handle) int {
	r1, _, _ := procSendMessageW.Call(uintptr(hwnd), TBM_GETPOS, 0, 0)
	return int(r1)
}

func comboAddString(hwnd windows.Handle, s string) {
	procSendMessageW.Call(uintptr(hwnd), CB_ADDSTRING, 0, uintptr(unsafe.Pointer(wstr(s))))
}

func comboSetCurSel(hwnd windows.Handle, idx int) {
	procSendMessageW.Call(uintptr(hwnd), CB_SETCURSEL, uintptr(idx), 0)
}

func wndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_CREATE:
		// Title row
		hTitle := createControl(0, WC_STATIC, "fivem tools", WS_CHILD|WS_VISIBLE|SS_LEFT, 16, 14, 300, 28, hwnd, ID_TITLE)
		hVer := createControl(0, WC_STATIC, "v1.0.80", WS_CHILD|WS_VISIBLE|SS_RIGHT, 0, 18, 120, 22, hwnd, ID_VERSION)

		// Separator (we'll draw in WM_PAINT too, but keep dummy control if you want)
		_ = createControl(0, WC_STATIC, "", WS_CHILD|WS_VISIBLE, 0, 52, 0, 1, hwnd, ID_SEP)

		// Mic row
		createControl(0, WC_STATIC, "เลือกอุปกรณ์นำเสียงเข้า:", WS_CHILD|WS_VISIBLE|SS_LEFT, 16, 78, 220, 22, hwnd, ID_MIC_LABEL)
		hCombo := createControl(WS_EX_CLIENTEDGE, WC_COMBOBOX, "", WS_CHILD|WS_VISIBLE|CBS_DROPDOWNLIST|WS_VSCROLL, 250, 74, 460, 260, hwnd, ID_MIC_COMBO)

		// Volume row (right aligned area like screenshot)
		hVolLabel := createControl(0, WC_STATIC, "ระดับเสียง: 100%", WS_CHILD|WS_VISIBLE|SS_LEFT, 250, 124, 220, 20, hwnd, ID_VOL_LABEL)
		hTrack := createControl(0, TRACKBAR_CLASS, "", WS_CHILD|WS_VISIBLE|TBS_HORZ|TBS_AUTOTICKS, 250, 148, 460, 34, hwnd, ID_VOL_TRACK)

		// Font: title bold big, other controls normal
		hFont = createUIFont()
		setFont(hTitle, hFont)

		// Make a normal font for the rest (non-bold, 14)
		var lf LOGFONTW
		lf.LfHeight = -14
		lf.LfWeight = 400
		copy(lf.LfFaceName[:], windows.StringToUTF16("Segoe UI"))
		r1, _, _ := procCreateFontIndirectW.Call(uintptr(unsafe.Pointer(&lf)))
		hFontNormal := windows.Handle(r1)
		if hFontNormal == 0 {
			hFontNormal = hFont
		}
		setFont(hVer, hFontNormal)
		setFont(hVolLabel, hFontNormal)
		setFont(hCombo, hFontNormal)

		// Trackbar setup
		trackbarSetRange(hTrack, 0, 100)
		trackbarSetPos(hTrack, 100)

		// Populate devices (mock)
		devs := []string{
			"Microphone (HyperX SoloCast)",
			"Microphone (Realtek(R) Audio)",
			"Line In (USB Audio Device)",
		}
		for _, d := range devs {
			comboAddString(hCombo, d)
		}
		comboSetCurSel(hCombo, 0)

		// Store normal font handle in window user data if you want cleanup; for simplicity keep global-less.

		return 0

	case WM_SIZE:
		// Responsive layout: keep margins similar, right column stretches
		rc := getClientRect(hwnd)
		width := rc.Right - rc.Left

		// Version label pinned to right
		hVer := getDlgItem(hwnd, ID_VERSION)
		move(hVer, width-16-120, 18, 120, 22)

		// Separator line full width
		hSep := getDlgItem(hwnd, ID_SEP)
		move(hSep, 0, 56, width, 1)

		// Combo and slider stretch
		rightX := int32(250)
		rightW := width - rightX - 16
		if rightW < 200 {
			rightW = 200
		}

		hCombo := getDlgItem(hwnd, ID_MIC_COMBO)
		move(hCombo, rightX, 74, rightW, 260)

		hVolLabel := getDlgItem(hwnd, ID_VOL_LABEL)
		move(hVolLabel, rightX, 124, rightW, 20)

		hTrack := getDlgItem(hwnd, ID_VOL_TRACK)
		move(hTrack, rightX, 148, rightW, 34)

		return 0

	case WM_HSCROLL:
		// Slider moved -> update "ระดับเสียง: %d%%"
		hTrack := getDlgItem(hwnd, ID_VOL_TRACK)
		// lParam is handle of trackbar for WM_HSCROLL in many cases
		if windows.Handle(lParam) == hTrack || hTrack != 0 {
			pos := trackbarGetPos(hTrack)
			hVolLabel := getDlgItem(hwnd, ID_VOL_LABEL)
			sendText(hVolLabel, fmt.Sprintf("ระดับเสียง: %d%%", pos))
		}
		return 0

	case WM_PAINT:
		// Fill white background and draw a thin separator line (optional)
		var ps PAINTSTRUCT
		hdc, _, _ := procBeginPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		if hdc != 0 {
			// Fill background (white)
			rBrush, _, _ := procGetStockObject.Call(WHITE_BRUSH)
			procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&ps.RcPaint)), rBrush)

			// You can draw separator line with a control; keeping paint minimal
			procEndPaint.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&ps)))
		}
		return 0

	case WM_DESTROY:
		// cleanup font handle (if it was CreateFontIndirect)
		if hFont != 0 {
			procDeleteObject.Call(uintptr(hFont))
			hFont = 0
		}
		procPostQuitMessage.Call(0)
		return 0
	}

	r1, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return r1
}

func main() {
	initCommonControls()

	r1, r2, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	mustOK(r1, "GetModuleHandleW")
	hInst = windows.Handle(r2)

	className := wstr("GoWin32FivemTools")

	// Cursor
	hCursor, _, _ := procLoadCursorW.Call(0, uintptr(IDC_ARROW))
	mustOK(hCursor, "LoadCursorW")

	// Background brush: handled in WM_PAINT (white), but set anyway
	hbr, _, _ := procGetStockObject.Call(WHITE_BRUSH)

	wcx := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		Style:         0,
		LpfnWndProc:   windows.NewCallback(wndProc),
		CbClsExtra:    0,
		CbWndExtra:    0,
		HInstance:     hInst,
		HIcon:         0,
		HCursor:       windows.Handle(hCursor),
		HbrBackground: windows.Handle(hbr),
		LpszMenuName:  nil,
		LpszClassName: className,
		HIconSm:       0,
	}

	r1, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wcx)))
	mustOK(r1, "RegisterClassExW")

	// Create main window
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(wstr("fivem"))),
		uintptr(WS_OVERLAPPEDWINDOW|WS_VISIBLE|WS_CLIPSIBLINGS),
		uintptr(200), uintptr(120), uintptr(780), uintptr(460),
		0, 0, uintptr(hInst), 0,
	)
	mustOK(hwnd, "CreateWindowExW(main)")

	procShowWindow.Call(hwnd, SW_SHOW)
	procUpdateWindow.Call(hwnd)

	// Message loop
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) <= 0 {
			break // WM_QUIT or error
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
