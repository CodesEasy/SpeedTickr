// Package tray runs the system-tray application: it owns the systray lifecycle and
// menu, drives a meter, and pushes each reading to the platform display.
package tray

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/codeseasy/speedtickr/internal/autostart"
	"github.com/codeseasy/speedtickr/internal/config"
	"github.com/codeseasy/speedtickr/internal/format"
	"github.com/codeseasy/speedtickr/internal/meter"
)

// intervalChoices are the update rates offered in the menu.
var intervalChoices = []struct {
	label string
	d     time.Duration
}{
	{"0.5 s", 500 * time.Millisecond},
	{"1 s", time.Second},
	{"2 s", 2 * time.Second},
	{"5 s", 5 * time.Second},
}

// Run starts the tray and blocks until the user quits. It must be called from the
// main goroutine — the backend's event loop requires ownership of the main thread.
func Run(cfg *config.Config) error {
	b := newBackend()
	a := &app{cfg: cfg, b: b, disp: newDisplay(cfg, b)}
	b.Run(a.onReady, a.onExit)
	return nil
}

type app struct {
	cfg  *config.Config
	b    backend
	disp display

	mu       sync.Mutex
	cancel   context.CancelFunc // cancels the running meter
	meterEnd chan struct{}      // closed when the current sample forwarder stops

	restartMu sync.Mutex // serializes restartMeter so only one sample forwarder runs
}

func (a *app) onReady() {
	a.disp.Init()
	a.applyDefaults()
	a.buildMenu()
	a.restartMeter()
	go a.waitForSignal()
}

// applyDefaults runs once, on first launch: it enables start-at-login by default so
// the meter is always on. Afterwards the user's choice (via the menu) is respected.
func (a *app) applyDefaults() {
	if a.cfg.AutoStarted {
		return
	}
	if err := autostart.Enable(); err != nil {
		slog.Warn("could not enable start-at-login by default", "err", err)
	}
	a.cfg.AutoStarted = true
	a.save()
}

func (a *app) onExit() {
	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	a.disp.Close()
}

// buildMenu constructs the menu and starts one goroutine per item to handle clicks.
func (a *app) buildMenu() {
	mUnits := a.b.AddMenuItem("Units", "How the speed is shown")
	units := format.Units()
	unitItems := make([]menuItem, len(units))
	for i, u := range units {
		unitItems[i] = mUnits.AddSubMenuItemCheckbox(u.Label(), "", a.cfg.Unit == u)
	}

	mInterval := a.b.AddMenuItem("Update interval", "How often the speed refreshes")
	intervalItems := make([]menuItem, len(intervalChoices))
	for i, c := range intervalChoices {
		intervalItems[i] = mInterval.AddSubMenuItemCheckbox(c.label, "", a.cfg.Interval.D() == c.d)
	}

	a.addPlatformMenu() // Font size (Windows only)

	enabled, _ := autostart.IsEnabled()
	mStartup := a.b.AddMenuItemCheckbox("Start at login", "Launch SpeedTickr when you sign in", enabled)
	go handleStartup(mStartup)

	a.b.AddSeparator()
	mQuit := a.b.AddMenuItem("Quit", "Quit SpeedTickr")

	for i := range units {
		go a.handleUnit(unitItems[i], units[i], unitItems)
	}
	for i := range intervalItems {
		go a.handleInterval(intervalItems[i], intervalChoices[i].d, intervalItems)
	}
	go func() {
		<-mQuit.ClickedCh()
		a.b.Quit()
	}()
}

func (a *app) handleUnit(item menuItem, u format.Unit, all []menuItem) {
	for range item.ClickedCh() {
		a.mu.Lock()
		a.cfg.Unit = u
		a.mu.Unlock()
		for _, mi := range all {
			setChecked(mi, mi == item)
		}
		a.save()
	}
}

func (a *app) handleInterval(item menuItem, d time.Duration, all []menuItem) {
	for range item.ClickedCh() {
		a.mu.Lock()
		a.cfg.Interval = config.Duration(d)
		a.mu.Unlock()
		for _, mi := range all {
			setChecked(mi, mi == item)
		}
		a.save()
		a.restartMeter()
	}
}

// restartMeter cancels any running meter and starts a fresh one with the current
// settings, forwarding its samples to the display. It waits for the previous
// forwarder to stop first so only one goroutine ever touches the display.
func (a *app) restartMeter() {
	a.restartMu.Lock()
	defer a.restartMu.Unlock()

	a.mu.Lock()
	if a.cancel != nil {
		a.cancel()
	}
	prevEnd := a.meterEnd
	a.mu.Unlock()

	if prevEnd != nil {
		<-prevEnd
	}

	ctx, cancel := context.WithCancel(context.Background())
	end := make(chan struct{})

	a.mu.Lock()
	a.cancel = cancel
	a.meterEnd = end
	interval := a.cfg.Interval.D()
	iface := a.cfg.Interface
	a.mu.Unlock()

	samples := meter.New(interval, iface).Run(ctx)
	go func() {
		defer close(end)
		for s := range samples {
			a.disp.Update(s.Down, s.Up, a.unit())
		}
	}()
}

func (a *app) unit() format.Unit {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg.Unit
}

func (a *app) save() {
	a.mu.Lock()
	snapshot := *a.cfg
	a.mu.Unlock()
	if err := snapshot.Save(); err != nil {
		slog.Warn("could not save config", "err", err)
	}
}

func (a *app) waitForSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch
	a.b.Quit()
}

// handleStartup toggles launch-at-login using the OS's native mechanism, keeping the
// checkmark in sync only when the change actually succeeds.
func handleStartup(item menuItem) {
	for range item.ClickedCh() {
		var err error
		if item.Checked() {
			if err = autostart.Disable(); err == nil {
				item.Uncheck()
			}
		} else if err = autostart.Enable(); err == nil {
			item.Check()
		}
		if err != nil {
			slog.Warn("could not change start-at-login", "err", err)
		}
	}
}

func setChecked(item menuItem, checked bool) {
	if checked {
		item.Check()
	} else {
		item.Uncheck()
	}
}
