//go:build windows

package tray

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"
)

// taskbarWindow is a borderless, layered window that paints the current speed as
// two lines of text directly on the Windows taskbar.
//
// It is reparented into the taskbar (Shell_TrayWnd) and positioned just left of the
// clock/tray area — the technique TrafficMonitor uses, which works on both Windows
// 10 and 11. If reparenting fails it falls back to a draggable always-on-top overlay
// floated over the taskbar. Transparency is done with a colour key (the magenta
// background is keyed out, leaving only white text over the real taskbar).
//
// The window owns a dedicated OS thread running a Win32 message loop; updates arrive
// via PostMessage so only that thread touches GDI/USER state.
type taskbarWindow struct {
	mu       sync.Mutex
	top, bot string // latest formatted lines, e.g. "4.2M" / "312K"

	hwnd     uintptr
	parent   uintptr // taskbar we reparented into (0 when floating)
	font     uintptr
	keyBrush uintptr
	embedded bool
	clientH  int32 // taskbar height
	glyphPx  int32 // current font glyph height (window thread only)
	scalePct int32 // font height as a % of taskbar height (atomic)

	ready chan struct{} // closed once create() has run (success or failure)
}

// theWindow is the single live instance; the C window procedure dispatches to it.
var theWindow *taskbarWindow

func newTaskbarWindow(scalePct int32) *taskbarWindow {
	return &taskbarWindow{top: "--", bot: "--", scalePct: scalePct, ready: make(chan struct{})}
}

// setScale changes the font height (as a percent of the taskbar height) and
// triggers a rebuild+repaint. Safe to call from any goroutine.
func (w *taskbarWindow) setScale(pct int32) {
	atomic.StoreInt32(&w.scalePct, pct)
	<-w.ready // window is created (or known-failed) by now, so hwnd is stable
	if w.hwnd != 0 {
		procPostMessageW.Call(w.hwnd, wmAppUpdate, 0, 0)
	}
}

// start launches the window's thread and message loop.
func (w *taskbarWindow) start() {
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		theWindow = w
		if err := w.create(); err != nil {
			slog.Warn("taskbar window unavailable", "err", err)
			close(w.ready)
			return
		}
		close(w.ready)
		w.loop()
	}()
}

// setText updates the displayed speed; it is safe to call from any goroutine.
func (w *taskbarWindow) setText(top, bot string) {
	<-w.ready // window is created (or known-failed) by now

	w.mu.Lock()
	changed := top != w.top || bot != w.bot
	w.top, w.bot = top, bot
	w.mu.Unlock()

	if changed && w.hwnd != 0 {
		procPostMessageW.Call(w.hwnd, wmAppUpdate, 0, 0)
	}
}

// stop tears the window down.
func (w *taskbarWindow) stop() {
	<-w.ready
	if w.hwnd != 0 {
		procPostMessageW.Call(w.hwnd, wmClose, 0, 0)
	}
}

func (w *taskbarWindow) create() error {
	hInst, _, _ := procGetModuleHandleW.Call(0)
	className, _ := windows.UTF16PtrFromString("SpeedTickrBar")
	cursor, _, _ := procLoadCursorW.Call(0, idcArrow)

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProcCallback,
		hInstance:     hInst,
		hCursor:       cursor,
		lpszClassName: className,
	}
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %w", err)
	}

	exStyle := uintptr(wsExLayered | wsExToolWindow | wsExNoActivate | wsExTopmost)
	hwnd, _, err := procCreateWindowExW.Call(
		exStyle,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		uintptr(wsPopup),
		0, 0, 90, 40,
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %w", err)
	}
	w.hwnd = hwnd

	// Colour-key transparency: the magenta background becomes see-through.
	procSetLayeredWindowAttributes.Call(hwnd, colorKey, 0, lwaColorKey)
	w.keyBrush, _, _ = procCreateSolidBrush.Call(colorKey)

	w.embed()
	w.reposition() // also sizes the font to the taskbar

	procShowWindow.Call(hwnd, swShowNoActivate)
	return nil
}

// embed reparents the window into the taskbar. On failure it stays a floating
// top-most overlay.
func (w *taskbarWindow) embed() {
	tray := findWindow("Shell_TrayWnd")
	if tray == 0 {
		return
	}
	if r, _, _ := procSetParent.Call(w.hwnd, tray); r == 0 {
		return
	}
	w.embedded = true
	w.parent = tray
	procSetWindowLongPtrW.Call(w.hwnd, gwlStyle, uintptr(wsChild|wsVisible))
}

// reposition keeps the window sized to the taskbar and parked left of the tray
// clock. It also re-embeds if Explorer restarted and replaced the taskbar.
func (w *taskbarWindow) reposition() {
	if w.hwnd == 0 {
		return
	}
	tray := findWindow("Shell_TrayWnd")

	// Not embedded yet but the taskbar is now available: try to embed. This
	// recovers when we launched before Explorer was ready (e.g. at login via
	// autostart), instead of staying stuck as a floating overlay forever.
	if !w.embedded && tray != 0 {
		w.embed()
	}

	if w.embedded {
		if tray == 0 {
			return
		}
		if tray != w.parent { // Explorer restarted; re-parent
			procSetParent.Call(w.hwnd, tray)
			w.parent = tray
		}
		var trayRect rect
		getWindowRect(tray, &trayRect)
		height := trayRect.Bottom - trayRect.Top
		w.clientH = height
		w.ensureFont()
		width := w.width()

		x := (trayRect.Right - trayRect.Left) - width - margin
		if notify := findWindowEx(tray, 0, "TrayNotifyWnd"); notify != 0 {
			var nr rect
			getWindowRect(notify, &nr)
			x = nr.Left - trayRect.Left - width - margin
		}
		if x < 0 {
			x = 0
		}
		setWindowPos(w.hwnd, 0, x, 0, width, height, swpNoActivate|swpNoZorder|swpShowWindow|swpFrameChanged)
		return
	}

	// Floating fallback: top-most overlay over the taskbar's right edge.
	var tr rect
	if tray != 0 {
		getWindowRect(tray, &tr)
	}
	height := tr.Bottom - tr.Top
	if height <= 0 {
		height = 40
	}
	w.clientH = height
	w.ensureFont()
	width := w.width()
	x := tr.Right - width - margin - 90
	if x < tr.Left {
		x = tr.Left
	}
	setWindowPos(w.hwnd, hwndTopmost, x, tr.Top, width, height, swpNoActivate|swpShowWindow)
}

// width sizes the window to its content so the text sits snug beside the clock.
// Seven glyph-widths comfortably holds a line like "↓ 1000Mbps".
func (w *taskbarWindow) width() int32 {
	width := w.glyphPx * 7
	if width < 80 {
		width = 80
	}
	return width
}

// ensureFont rebuilds the font whenever the glyph height would change — i.e. the
// taskbar height changed (DPI/resolution) or the user picked a different size.
// Glyph height is scalePct of the taskbar height, so it tracks the bar like the clock.
func (w *taskbarWindow) ensureFont() {
	px := w.clientH * atomic.LoadInt32(&w.scalePct) / 100
	if px < 9 {
		px = 9 // floor only: the percent already bounds the size to the bar height
	}
	if w.font != 0 && px == w.glyphPx {
		return
	}
	if w.font != 0 {
		procDeleteObject.Call(w.font)
		w.font = 0
	}
	w.glyphPx = px
	face, _ := windows.UTF16PtrFromString("Segoe UI")
	w.font, _, _ = procCreateFontW.Call(
		uintptr(-px), 0, 0, 0, // negative = glyph height in pixels
		fwSemibold, 0, 0, 0,
		defaultCharset, 0, 0, nonAntialiasedQuality,
		0, uintptr(unsafe.Pointer(face)),
	)
}

func (w *taskbarWindow) paint(hwnd uintptr) {
	var ps paintStruct
	hdc, _, _ := procBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
	if hdc == 0 {
		return
	}
	defer procEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))

	var rc rect
	procGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
	procFillRect.Call(hdc, uintptr(unsafe.Pointer(&rc)), w.keyBrush) // keyed-out background

	if w.font != 0 {
		procSelectObject.Call(hdc, w.font)
	}
	procSetBkMode.Call(hdc, transparent)
	procSetTextColor.Call(hdc, textColor)

	w.mu.Lock()
	top, bot := w.top, w.bot
	w.mu.Unlock()

	// Two tightly-stacked lines, centered vertically as a block (like the clock).
	lineH := w.glyphPx + 3
	y := (rc.Bottom - lineH*2) / 2
	if y < 0 {
		y = 0
	}
	drawLine(hdc, "↓ "+top, rect{rc.Left, y, rc.Right, y + lineH})
	drawLine(hdc, "↑ "+bot, rect{rc.Left, y + lineH, rc.Right, y + lineH*2})
}

func (w *taskbarWindow) loop() {
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 { // 0 = WM_QUIT, -1 = error
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	if w.font != 0 {
		procDeleteObject.Call(w.font)
	}
	if w.keyBrush != 0 {
		procDeleteObject.Call(w.keyBrush)
	}
}

// wndProcCallback is the window procedure. There is a single window, so it routes
// to theWindow rather than threading state through the user data slot.
var wndProcCallback = windows.NewCallback(func(hwnd, msg, wParam, lParam uintptr) uintptr {
	w := theWindow
	switch msg {
	case wmPaint:
		if w != nil {
			w.paint(hwnd)
		}
		return 0
	case wmAppUpdate:
		if w != nil {
			w.reposition()
			procInvalidateRect.Call(hwnd, 0, 1)
		}
		return 0
	case wmLButtonDown:
		if w != nil && !w.embedded { // let the floating overlay be dragged
			procReleaseCapture.Call()
			procSendMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
		}
		return 0
	case wmDestroy:
		procPostQuitMessage.Call(0)
		return 0
	default:
		r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
		return r
	}
})

// --- thin Win32 helpers ---

func findWindow(class string) uintptr {
	c, _ := windows.UTF16PtrFromString(class)
	r, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(c)), 0)
	return r
}

func findWindowEx(parent, after uintptr, class string) uintptr {
	c, _ := windows.UTF16PtrFromString(class)
	r, _, _ := procFindWindowExW.Call(parent, after, uintptr(unsafe.Pointer(c)), 0)
	return r
}

func getWindowRect(hwnd uintptr, r *rect) {
	procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(r)))
}

func setWindowPos(hwnd, after uintptr, x, y, cx, cy int32, flags uintptr) {
	procSetWindowPos.Call(hwnd, after, uintptr(x), uintptr(y), uintptr(cx), uintptr(cy), flags)
}

func drawLine(hdc uintptr, s string, r rect) {
	u, err := windows.UTF16PtrFromString(s)
	if err != nil {
		return
	}
	procDrawTextW.Call(
		hdc,
		uintptr(unsafe.Pointer(u)),
		^uintptr(0), // -1: NUL-terminated
		uintptr(unsafe.Pointer(&r)),
		uintptr(dtSingleLine|dtCenter|dtVCenter|dtNoPrefix),
	)
}
