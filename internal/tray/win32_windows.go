//go:build windows

package tray

import "golang.org/x/sys/windows"

// Win32 entry points used by the taskbar window. They are resolved lazily from the
// standard system DLLs; nothing here needs cgo.
var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procRegisterClassExW           = user32.NewProc("RegisterClassExW")
	procCreateWindowExW            = user32.NewProc("CreateWindowExW")
	procDefWindowProcW             = user32.NewProc("DefWindowProcW")
	procShowWindow                 = user32.NewProc("ShowWindow")
	procGetMessageW                = user32.NewProc("GetMessageW")
	procTranslateMessage           = user32.NewProc("TranslateMessage")
	procDispatchMessageW           = user32.NewProc("DispatchMessageW")
	procPostMessageW               = user32.NewProc("PostMessageW")
	procSendMessageW               = user32.NewProc("SendMessageW")
	procPostQuitMessage            = user32.NewProc("PostQuitMessage")
	procFindWindowW                = user32.NewProc("FindWindowW")
	procFindWindowExW              = user32.NewProc("FindWindowExW")
	procSetParent                  = user32.NewProc("SetParent")
	procSetWindowLongPtrW          = user32.NewProc("SetWindowLongPtrW")
	procSetWindowPos               = user32.NewProc("SetWindowPos")
	procGetWindowRect              = user32.NewProc("GetWindowRect")
	procGetClientRect              = user32.NewProc("GetClientRect")
	procBeginPaint                 = user32.NewProc("BeginPaint")
	procEndPaint                   = user32.NewProc("EndPaint")
	procFillRect                   = user32.NewProc("FillRect")
	procInvalidateRect             = user32.NewProc("InvalidateRect")
	procLoadCursorW                = user32.NewProc("LoadCursorW")
	procSetLayeredWindowAttributes = user32.NewProc("SetLayeredWindowAttributes")
	procReleaseCapture             = user32.NewProc("ReleaseCapture")
	procDrawTextW                  = user32.NewProc("DrawTextW")

	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procSelectObject     = gdi32.NewProc("SelectObject")
	procSetTextColor     = gdi32.NewProc("SetTextColor")
	procSetBkMode        = gdi32.NewProc("SetBkMode")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
)

const (
	// Window styles.
	wsPopup   = 0x80000000
	wsChild   = 0x40000000
	wsVisible = 0x10000000

	wsExLayered    = 0x00080000
	wsExToolWindow = 0x00000080
	wsExNoActivate = 0x08000000
	wsExTopmost    = 0x00000008

	// GWL_STYLE (-16) as an arch-independent uintptr.
	gwlStyle = ^uintptr(15)

	// Messages.
	wmDestroy       = 0x0002
	wmPaint         = 0x000F
	wmClose         = 0x0010
	wmLButtonDown   = 0x0201
	wmNCLButtonDown = 0x00A1
	wmAppUpdate     = 0x8000 + 1 // WM_APP+1: "speed changed, repaint"

	// ShowWindow / SetWindowPos.
	swShowNoActivate = 4
	swpNoActivate    = 0x0010
	swpNoZorder      = 0x0004
	swpShowWindow    = 0x0040
	swpFrameChanged  = 0x0020
	hwndTopmost      = ^uintptr(0) // HWND_TOPMOST (-1)

	// Layering, fonts, text.
	lwaColorKey           = 0x00000001
	colorKey              = 0x00FF00FF // magenta, keyed out to transparency
	textColor             = 0x00FFFFFF // white (COLORREF 0x00BBGGRR)
	transparent           = 1
	fwSemibold            = 600
	defaultCharset        = 1
	nonAntialiasedQuality = 3

	dtCenter     = 0x0001
	dtVCenter    = 0x0004
	dtSingleLine = 0x0020
	dtNoPrefix   = 0x0800

	htCaption = 2
	idcArrow  = 32512
	margin    = 6
)

type point struct{ X, Y int32 }

type rect struct{ Left, Top, Right, Bottom int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rect
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}
