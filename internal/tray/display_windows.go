//go:build windows

package tray

import (
	"image"
	"sync"

	"github.com/codeseasy/speedtickr/internal/config"
	"github.com/codeseasy/speedtickr/internal/format"
	"github.com/codeseasy/speedtickr/internal/icon"
)

// On Windows the speed is shown as real text on the taskbar via a borderless
// layered window (see taskbarwindow_windows.go). The tray icon is kept only as the
// anchor for the right-click menu (units / interval / font / quit) and shows a
// static glyph — never the speed, so it doesn't churn temp icon files.
func newDisplay(cfg *config.Config, b backend) display {
	return &windowsDisplay{b: b, win: newTaskbarWindow(fontPct(cfg.FontSize))}
}

type windowsDisplay struct {
	b       backend
	win     *taskbarWindow
	lastTip string
}

func (d *windowsDisplay) Init() {
	d.b.SetIcon(defaultICO())
	d.b.SetTooltip("SpeedTickr")
	d.win.start()
}

func (d *windowsDisplay) Update(down, up float64, u format.Unit) {
	d.win.setText(format.Short(down, u), format.Short(up, u))
	if tip := format.Tooltip(down, up, u); tip != d.lastTip {
		d.b.SetTooltip(tip)
		d.lastTip = tip
	}
}

func (d *windowsDisplay) Close() { d.win.stop() }

// fontPct maps a font-size preset to a percent of the taskbar height.
func fontPct(s config.FontSize) int32 {
	switch s {
	case config.FontSmall:
		return 26
	case config.FontLarge:
		return 40
	default:
		return 33
	}
}

// addPlatformMenu adds the Windows-only "Font size" submenu and wires it to the
// taskbar window. On macOS/Linux this is a no-op (see display_other.go).
func (a *app) addPlatformMenu() {
	wd, ok := a.disp.(*windowsDisplay)
	if !ok {
		return
	}

	m := a.b.AddMenuItem("Font size", "Taskbar text size")
	sizes := []struct {
		label string
		size  config.FontSize
	}{
		{"Small", config.FontSmall},
		{"Medium", config.FontMedium},
		{"Large", config.FontLarge},
	}
	items := make([]menuItem, len(sizes))
	for i, s := range sizes {
		items[i] = m.AddSubMenuItemCheckbox(s.label, "", a.cfg.FontSize == s.size)
	}
	for i := range sizes {
		go func(sel menuItem, size config.FontSize) {
			for range sel.ClickedCh() {
				a.mu.Lock()
				a.cfg.FontSize = size
				a.mu.Unlock()
				for _, mi := range items {
					setChecked(mi, mi == sel)
				}
				wd.win.setScale(fontPct(size))
				a.save()
			}
		}(items[i], sizes[i].size)
	}
}

// defaultICO encodes the app glyph as a multi-size Windows .ico for the tray menu
// icon, so Windows can pick a crisp size for the notification area.
var defaultICO = sync.OnceValue(func() []byte {
	return icon.ICO([]*image.RGBA{icon.Glyph(16), icon.Glyph(24), icon.Glyph(32), icon.Glyph(48)})
})
