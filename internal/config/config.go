// Package config loads and persists user settings as JSON in the OS config dir.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/codeseasy/speedtickr/internal/format"
)

// Config holds the user-tunable settings.
type Config struct {
	Unit        format.Unit `json:"unit"`                  // bytes or bits
	Interval    Duration    `json:"interval"`              // poll interval
	Interface   string      `json:"interface,omitempty"`   // specific NIC, or "" for all
	FontSize    FontSize    `json:"font_size"`             // taskbar text size (Windows)
	AutoStarted bool        `json:"autostart_initialized"` // first-run autostart default applied
}

// Default returns the built-in configuration.
func Default() *Config {
	return &Config{
		Unit:     format.Kbps,
		Interval: Duration(time.Second),
		FontSize: FontSmall,
	}
}

// Load reads the config file, falling back to defaults when it is missing or
// invalid. A missing file is created with defaults so users have something to edit.
func Load() (*Config, error) {
	path, err := filePath()
	if err != nil {
		return Default(), err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		c := Default()
		_ = c.Save()
		return c, nil
	}
	if err != nil {
		return Default(), err
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return Default(), err
	}
	// Apply each field independently so one malformed value (e.g. hand-edited) falls
	// back to its default instead of discarding every other setting.
	c := Default()
	applyField(raw, "unit", &c.Unit)
	applyField(raw, "interval", &c.Interval)
	applyField(raw, "interface", &c.Interface)
	applyField(raw, "font_size", &c.FontSize)
	applyField(raw, "autostart_initialized", &c.AutoStarted)
	c.Normalize()
	return c, nil
}

// applyField unmarshals a single config field, leaving dst at its default if the
// field is absent or malformed — so one bad value can't reset the whole file.
func applyField[T any](raw map[string]json.RawMessage, key string, dst *T) {
	if v, ok := raw[key]; ok {
		var tmp T
		if json.Unmarshal(v, &tmp) == nil {
			*dst = tmp
		}
	}
}

// Save writes the config as indented JSON, creating the directory if needed.
func (c *Config) Save() error {
	path, err := filePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Normalize clamps out nonsensical values that could be hand-edited into the file
// or supplied via command-line flags.
func (c *Config) Normalize() {
	const minInterval = 200 * time.Millisecond
	const maxInterval = time.Hour
	switch {
	case c.Interval.D() < minInterval:
		c.Interval = Duration(time.Second)
	case c.Interval.D() > maxInterval:
		c.Interval = Duration(maxInterval)
	}
}

func filePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "speedtickr", "config.json"), nil
}

// Duration is a time.Duration that marshals to/from a human string like "1s",
// so the config file stays readable instead of holding raw nanoseconds.
type Duration time.Duration

// D returns the underlying time.Duration.
func (d Duration) D() time.Duration { return time.Duration(d) }

// MarshalText implements encoding.TextMarshaler.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (d *Duration) UnmarshalText(b []byte) error {
	v, err := time.ParseDuration(string(b))
	if err != nil {
		return err
	}
	*d = Duration(v)
	return nil
}

// FontSize selects the taskbar text size on Windows (ignored on macOS/Linux, where
// the OS renders the menu-bar/panel text).
type FontSize int

const (
	FontMedium FontSize = iota
	FontSmall
	FontLarge
)

func (f FontSize) String() string {
	switch f {
	case FontSmall:
		return "small"
	case FontLarge:
		return "large"
	default:
		return "medium"
	}
}

// MarshalText implements encoding.TextMarshaler.
func (f FontSize) MarshalText() ([]byte, error) { return []byte(f.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (f *FontSize) UnmarshalText(b []byte) error {
	switch string(b) {
	case "small":
		*f = FontSmall
	case "large":
		*f = FontLarge
	case "medium", "":
		*f = FontMedium
	default:
		return fmt.Errorf("config: unknown font size %q", b)
	}
	return nil
}
