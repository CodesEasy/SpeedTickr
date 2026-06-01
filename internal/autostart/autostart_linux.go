//go:build linux

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// entryPath is the XDG autostart desktop entry, read by the desktop environment at
// login (~/.config/autostart/speedtickr.desktop).
func entryPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "autostart", "speedtickr.desktop"), nil
}

// Enable writes the autostart desktop entry per the freedesktop Autostart spec.
func Enable() error {
	exe, err := exePath()
	if err != nil {
		return err
	}
	path, err := entryPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// Exec is interpolated inside double quotes, so escape the characters the
	// Desktop Entry spec requires there: backslash, double quote, backtick, $.
	escExe := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(exe)
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Exec="%s"
Terminal=false
X-GNOME-Autostart-enabled=true
Hidden=false
`, appName, escExe)
	return os.WriteFile(path, []byte(entry), 0o644)
}
