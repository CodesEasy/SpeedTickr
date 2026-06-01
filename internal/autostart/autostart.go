// Package autostart enables or disables launching SpeedTickr when the user signs
// in, using each OS's standard per-user mechanism (no admin rights needed):
//
//	Windows : HKCU\Software\Microsoft\Windows\CurrentVersion\Run registry value
//	macOS   : ~/Library/LaunchAgents/<label>.plist (launchd, RunAtLoad)
//	Linux   : ~/.config/autostart/speedtickr.desktop (XDG autostart)
//
// Each backend implements Enable, Disable, and IsEnabled.
package autostart

import (
	"os"
	"path/filepath"
)

const (
	appName = "SpeedTickr"               // Windows Run value name / Linux entry Name
	label   = "com.codeseasy.speedtickr" // macOS LaunchAgent label + filename
)

// exePath returns the absolute path to the running binary, resolving symlinks so the
// recorded autostart command keeps working if the binary is reached via a link.
func exePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(p); err == nil {
		p = resolved
	}
	return p, nil
}
