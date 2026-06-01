//go:build darwin || linux

package autostart

import "os"

// macOS and Linux both autostart from a file (a launchd plist / an XDG .desktop), so
// disable and status are shared here; each OS only supplies entryPath and Enable.

// Disable removes the autostart entry. Missing is treated as success.
func Disable() error {
	path, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsEnabled reports whether the autostart entry exists.
func IsEnabled() (bool, error) {
	path, err := entryPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
