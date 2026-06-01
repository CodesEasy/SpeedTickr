//go:build !windows

package singleton

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// acquire takes an exclusive advisory lock on a file in the config dir. flock conflicts
// across open file descriptions regardless of process, and the kernel releases it when
// the file descriptor is closed — which happens on exit, clean or not.
func acquire() (func(), bool, error) {
	path, err := lockPath()
	if err != nil {
		return nil, false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil // another instance holds the lock
		}
		return nil, false, err
	}
	// Keep f open for the whole run: the closure is the only reference, so the fd (and
	// thus the lock) stays alive until release runs. The OS frees it on exit regardless.
	release := func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}
	return release, true, nil
}

// lockPath is the advisory lock file, kept next to the config in the user config dir.
func lockPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "speedtickr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "speedtickr.lock"), nil
}
