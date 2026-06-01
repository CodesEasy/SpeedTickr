package tray

import "github.com/codeseasy/speedtickr/internal/format"

// display is how a speed reading reaches the user. Each OS implements it in the
// most native way available — see the per-platform files for why they differ:
//
//	macOS / Linux : real text shown as the tray title via backend.SetTitle
//	Windows       : real text painted into our own taskbar window (SetTitle is a no-op there)
//
// newDisplay is defined per platform with a build constraint and is handed the
// backend so the display can set the icon/title/tooltip without touching a toolkit.
type display interface {
	// Init sets the initial icon/title before the first sample arrives.
	Init()
	// Update reflects the latest download/upload rate (bytes/sec) in the given unit.
	Update(down, up float64, u format.Unit)
	// Close releases any OS resources (e.g. the Windows taskbar window).
	Close()
}
