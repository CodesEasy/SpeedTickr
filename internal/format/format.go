// Package format turns a bytes-per-second rate into compact, human-readable text.
//
// It is the single owner of unit math and rounding so the taskbar, tooltip, and
// menu-bar title stay consistent everywhere. Each unit is pinned (no auto-scaling);
// decimal SI (base 1000), the networking norm.
package format

import (
	"fmt"
	"math"
	"strconv"
)

// Unit is a fixed bit-rate display unit for a throughput rate.
type Unit int

const (
	Bps  Unit = iota // bit/s
	Kbps             // Kbit/s — kilobits per second
	Mbps             // Mbit/s — megabits per second
	Gbps             // Gbit/s — gigabits per second
	Tbps             // Tbit/s — terabits per second
)

var prefixes = []string{"", "K", "M", "G", "T"}

type unitInfo struct {
	token string // config token / -unit value
	label string // menu label
	short string // compact suffix for the taskbar, e.g. "Mbps"
	pow   int    // SI power: 0=bit, 1=Kbit, 2=Mbit, 3=Gbit, 4=Tbit
}

// unitTable is indexed by the Unit constant and ordered for the menu. Every unit is
// bit-based; pow is the SI power (0=bit, 1=Kbit, 2=Mbit, 3=Gbit, 4=Tbit).
var unitTable = []unitInfo{
	Bps:  {"bps", "bit/s (bps)", "bps", 0},
	Kbps: {"kbps", "Kbit/s (Kbps)", "Kbps", 1},
	Mbps: {"mbps", "Mbit/s (Mbps)", "Mbps", 2},
	Gbps: {"gbps", "Gbit/s (Gbps)", "Gbps", 3},
	Tbps: {"tbps", "Tbit/s (Tbps)", "Tbps", 4},
}

// Units returns every selectable unit, in menu order.
func Units() []Unit {
	all := make([]Unit, len(unitTable))
	for i := range unitTable {
		all[i] = Unit(i)
	}
	return all
}

func (u Unit) info() unitInfo {
	if int(u) < 0 || int(u) >= len(unitTable) {
		return unitTable[Kbps]
	}
	return unitTable[u]
}

// Label is the menu text for a unit.
func (u Unit) Label() string { return u.info().label }

// String returns the config token for a Unit.
func (u Unit) String() string { return u.info().token }

// MarshalText implements encoding.TextMarshaler so config files stay readable.
func (u Unit) MarshalText() ([]byte, error) { return []byte(u.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler.
func (u *Unit) UnmarshalText(b []byte) error {
	v, ok := ParseUnit(string(b))
	if !ok {
		return fmt.Errorf("format: unknown unit %q", b)
	}
	*u = v
	return nil
}

// ParseUnit resolves a token to a Unit.
func ParseUnit(s string) (Unit, bool) {
	for i, in := range unitTable {
		if in.token == s {
			return Unit(i), true
		}
	}
	return Kbps, false
}

// scaled reduces a bytes/sec rate to a value and its SI prefix. Every unit is
// bit-based, so the rate is converted from bytes to bits here.
func scaled(bytesPerSec float64, in unitInfo) (value float64, prefix string) {
	value = bytesPerSec * 8
	for p := 0; p < in.pow; p++ {
		value /= 1000
	}
	return value, prefixes[in.pow]
}

// num formats a scaled value compactly: one decimal below 10, none at/above 10.
// The branch is chosen from the *rounded* value so 9.96 shows as "10" (not "10.0")
// and a negligible rate that rounds to zero shows as "0" (not "0.0").
func num(v float64) string {
	if v <= 0 {
		return "0"
	}
	switch r := math.Round(v*10) / 10; {
	case r < 0.05: // rounds to zero at one decimal
		return "0"
	case r >= 10:
		return strconv.FormatFloat(v, 'f', 0, 64)
	default:
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
}

// Full renders a complete, labelled rate, e.g. "850 Kbit/s" or "4.2 Mbit/s".
func Full(bytesPerSec float64, u Unit) string {
	v, prefix := scaled(bytesPerSec, u.info())
	return fmt.Sprintf("%s %sbit/s", num(v), prefix)
}

// Short renders a compact rate for the taskbar / menu-bar with the full unit, e.g.
// "850Kbps" or "4.2Mbps".
func Short(bytesPerSec float64, u Unit) string {
	in := u.info()
	v, _ := scaled(bytesPerSec, in)
	return num(v) + in.short
}

// Tooltip renders the full download/upload figures shown on hover, shared by every
// platform's display so the wording stays identical.
func Tooltip(down, up float64, u Unit) string {
	return fmt.Sprintf("Download %s   •   Upload %s", Full(down, u), Full(up, u))
}
