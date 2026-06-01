// Package singleton keeps SpeedTickr to one running instance per user session.
// Launching the binary again (a second double-click, or a duplicate "start at login")
// detects the instance already running and exits, instead of stacking another meter in
// the tray / menu bar. The lock is an OS primitive the system releases automatically
// when the process ends — including on a crash — so there is never a stale lock to
// clear:
//
//	Windows       : a named mutex (Local\…), scoped to the user's session
//	macOS / Linux : an advisory flock on a lock file in the config dir
package singleton

// Acquire tries to become the sole running instance:
//
//   - ok == true:  this is the only instance. Call the returned release on a clean exit
//     (the OS also drops the lock on process death, so this is best-effort tidiness).
//   - ok == false: another instance already holds the lock; the caller should exit.
//   - err != nil:  the check itself failed; the caller may choose to start anyway.
func Acquire() (release func(), ok bool, err error) {
	return acquire()
}
