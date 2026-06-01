package tray

// backend is the OS tray/menu host. It owns the event loop, the status
// icon/title/tooltip, and the right-click menu, so the rest of the package never
// talks to an OS toolkit directly. Two implementations satisfy it, selected by build
// constraint via newBackend:
//
//	!darwin  fyne.io/systray (Windows syscalls, Linux/BSD D-Bus — all cgo-free)
//	darwin   a pure-Go AppKit backend driven through purego (see backend_darwin.go)
//
// Keeping macOS off systray is what lets the whole app stay cgo-free and
// cross-compile to every target from one machine.
type backend interface {
	// Run sets up the tray and blocks on the OS event loop until Quit. It must be
	// called on the main goroutine. onReady runs once the loop is live — build the
	// menu and start background work there; onExit runs as the app tears down.
	Run(onReady, onExit func())
	// Quit ends the event loop (and the app).
	Quit()
	// SetIcon sets the status icon from encoded image bytes — ICO on Windows, PNG on
	// macOS/Linux; the caller passes the format its platform expects.
	SetIcon(data []byte)
	// SetTitle sets the menu-bar / panel text (a no-op where the OS has no text slot,
	// e.g. the Windows tray, which draws its own taskbar window instead).
	SetTitle(s string)
	// SetTooltip sets the hover tooltip.
	SetTooltip(s string)
	// AddMenuItem appends a top-level row; AddMenuItemCheckbox one that shows a
	// checkmark; AddSeparator a divider.
	AddMenuItem(title, tooltip string) menuItem
	AddMenuItemCheckbox(title, tooltip string, checked bool) menuItem
	AddSeparator()
}

// menuItem is one row of the right-click menu. The shape mirrors fyne.io/systray's
// *MenuItem so the menu-building code reads the same on every platform.
type menuItem interface {
	// AddSubMenuItemCheckbox adds a checkable child row and returns it.
	AddSubMenuItemCheckbox(title, tooltip string, checked bool) menuItem
	// ClickedCh fires once per click on this item.
	ClickedCh() <-chan struct{}
	// Check / Uncheck set the checkmark; Checked reports its current state.
	Check()
	Uncheck()
	Checked() bool
}
