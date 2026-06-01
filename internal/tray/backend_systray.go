//go:build !darwin

package tray

import "fyne.io/systray"

// On every platform except macOS, fyne.io/systray is cgo-free (Windows uses
// syscalls; Linux/BSD use D-Bus StatusNotifierItem), so we use it directly. macOS is
// the lone cgo path in systray, so it gets its own pure-Go backend instead
// (backend_darwin.go) and never imports this file.
func newBackend() backend { return systrayBackend{} }

type systrayBackend struct{}

func (systrayBackend) Run(onReady, onExit func()) { systray.Run(onReady, onExit) }
func (systrayBackend) Quit()                      { systray.Quit() }
func (systrayBackend) SetIcon(data []byte)        { systray.SetIcon(data) }
func (systrayBackend) SetTitle(s string)          { systray.SetTitle(s) }
func (systrayBackend) SetTooltip(s string)        { systray.SetTooltip(s) }
func (systrayBackend) AddSeparator()              { systray.AddSeparator() }

func (systrayBackend) AddMenuItem(title, tooltip string) menuItem {
	return systrayItem{systray.AddMenuItem(title, tooltip)}
}

func (systrayBackend) AddMenuItemCheckbox(title, tooltip string, checked bool) menuItem {
	return systrayItem{systray.AddMenuItemCheckbox(title, tooltip, checked)}
}

// systrayItem wraps *systray.MenuItem. It is a value type holding the pointer, so two
// wrappers of the same underlying item compare equal — which the radio-style menus
// rely on (mi == clicked).
type systrayItem struct{ mi *systray.MenuItem }

func (i systrayItem) AddSubMenuItemCheckbox(title, tooltip string, checked bool) menuItem {
	return systrayItem{i.mi.AddSubMenuItemCheckbox(title, tooltip, checked)}
}
func (i systrayItem) ClickedCh() <-chan struct{} { return i.mi.ClickedCh }
func (i systrayItem) Check()                     { i.mi.Check() }
func (i systrayItem) Uncheck()                   { i.mi.Uncheck() }
func (i systrayItem) Checked() bool              { return i.mi.Checked() }
