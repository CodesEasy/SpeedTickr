//go:build windows

package singleton

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
)

// acquire creates a named mutex. The first instance creates it; a later instance gets
// the same name back with ERROR_ALREADY_EXISTS and bows out. Windows frees the mutex
// when the process exits (including a crash), so there's nothing to clean up. The
// Local\ namespace scopes it to the user's session, matching the per-user model.
func acquire() (func(), bool, error) {
	name, err := windows.UTF16PtrFromString(`Local\SpeedTickr-SingleInstance`)
	if err != nil {
		return nil, false, err
	}
	// CreateMutexW returns a valid handle even when the mutex already exists; the
	// distinguishing signal is GetLastError(), which Call surfaces as its third result.
	h, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, false, fmt.Errorf("CreateMutexW: %w", callErr)
	}
	if errors.Is(callErr, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(windows.Handle(h)) // drop our handle to the existing mutex
		return nil, false, nil
	}
	release := func() { windows.CloseHandle(windows.Handle(h)) }
	return release, true, nil
}
