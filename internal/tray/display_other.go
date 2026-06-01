//go:build !windows

package tray

import (
	"bytes"
	"fmt"
	"image/png"
	"sync"

	"github.com/codeseasy/speedtickr/internal/config"
	"github.com/codeseasy/speedtickr/internal/format"
	"github.com/codeseasy/speedtickr/internal/icon"
)

// On macOS and Linux the tray supports real text, so we show the speed as the
// item's title (menu bar on macOS, top panel on Linux) and keep the full figures
// in the tooltip. A small icon sits alongside — some Linux trays need one to appear.
// cfg is unused here; the OS controls the menu-bar/panel font.
func newDisplay(cfg *config.Config, b backend) display { return &titleDisplay{b: b} }

// addPlatformMenu has no extra entries here — font sizing is Windows-only.
func (a *app) addPlatformMenu() {}

type titleDisplay struct {
	b         backend
	lastTitle string
	lastTip   string
}

func (d *titleDisplay) Init() {
	d.b.SetIcon(defaultIconPNG())
	d.b.SetTitle("…")
	d.b.SetTooltip("SpeedTickr")
}

func (d *titleDisplay) Update(down, up float64, u format.Unit) {
	if title := fmt.Sprintf("↓ %s  ↑ %s", format.Short(down, u), format.Short(up, u)); title != d.lastTitle {
		d.b.SetTitle(title)
		d.lastTitle = title
	}
	if tip := format.Tooltip(down, up, u); tip != d.lastTip {
		d.b.SetTooltip(tip)
		d.lastTip = tip
	}
}

func (d *titleDisplay) Close() {}

// defaultIconPNG encodes the app glyph as PNG (the format macOS/Linux trays accept).
// A 64px render keeps it crisp on HiDPI menu bars. Built once, on first use.
var defaultIconPNG = sync.OnceValue(func() []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, icon.Glyph(64))
	return buf.Bytes()
})
