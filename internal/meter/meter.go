// Package meter samples network interface byte counters and reports throughput.
//
// It reads cumulative counters from gopsutil, diffs successive readings over the
// real elapsed time, and emits a Sample per tick. It knows nothing about how the
// result is displayed — that keeps it small and testable.
package meter

import (
	"context"
	"log/slog"
	stdnet "net"
	"sync"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"
)

// Sample is the throughput observed over one interval, in bytes per second.
type Sample struct {
	Down float64 // bytes/sec received
	Up   float64 // bytes/sec sent
}

// counterFunc reads per-interface cumulative counters. It is a variable so tests
// can substitute a deterministic source.
var counterFunc = func() ([]psnet.IOCountersStat, error) {
	return psnet.IOCounters(true)
}

// Meter periodically computes network throughput.
type Meter struct {
	interval time.Duration
	iface    string          // specific interface name, or "" for all non-loopback
	loopback map[string]bool // interface names to exclude when iface == ""

	warnMissing sync.Once // logs a configured-but-absent interface at most once
}

// New returns a Meter that polls every interval. If iface is non-empty only that
// interface is measured; otherwise the meter measures whichever interface currently
// reaches the internet, which avoids double-counting when a VPN/tunnel or virtual
// switch carries the same traffic as the physical NIC.
func New(interval time.Duration, iface string) *Meter {
	return &Meter{
		interval: interval,
		iface:    iface,
		loopback: loopbackNames(),
	}
}

// Run samples until ctx is cancelled, sending a Sample on the returned channel
// after every interval. The channel is closed when ctx is done. The first
// interval establishes a baseline and reports zero.
func (m *Meter) Run(ctx context.Context) <-chan Sample {
	out := make(chan Sample)

	go func() {
		defer close(out)

		prevDown, prevUp := m.totals()
		last := time.Now()

		ticker := time.NewTicker(m.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				down, up := m.totals()
				dt := now.Sub(last).Seconds()
				last = now

				var s Sample
				if dt > 0 {
					s.Down = rate(down, prevDown, dt)
					s.Up = rate(up, prevUp, dt)
				}
				prevDown, prevUp = down, up

				select {
				case out <- s:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return out
}

// totals returns the received/sent byte counters for the interface that matters: a
// specific one if configured, otherwise the interface that currently reaches the
// internet. Counting that single interface — rather than every interface — avoids
// double-counting when a VPN/tunnel or virtual switch mirrors the physical NIC. If
// the internet-facing interface can't be determined it falls back to summing all
// non-loopback interfaces. On counter read error it returns 0,0 (a counter-reset
// guard turns that into a zero rate next tick).
func (m *Meter) totals() (down, up uint64) {
	counters, err := counterFunc()
	if err != nil {
		return 0, 0
	}
	name := m.iface
	if name == "" {
		name = primaryFunc()
	}
	matched := false
	for _, c := range counters {
		if name != "" {
			if c.Name == name {
				down += c.BytesRecv
				up += c.BytesSent
				matched = true
			}
			continue
		}
		if m.loopback[c.Name] { // fallback: sum every non-loopback interface
			continue
		}
		down += c.BytesRecv
		up += c.BytesSent
	}
	// A configured interface that matches no counter (renamed/removed) would read a
	// flat zero forever; warn once so it isn't mistaken for an idle link.
	if m.iface != "" && !matched {
		m.warnMissing.Do(func() {
			slog.Warn("configured network interface not found; speed will read zero", "interface", m.iface)
		})
	}
	return down, up
}

// primaryFunc resolves the internet-facing interface name; it is a variable so tests
// can substitute a deterministic value.
var primaryFunc = primaryInterface

// primaryInterface returns the name of the interface used to reach the internet,
// found by asking the OS which local address it would use for an external host. No
// packets are sent (UDP dial just selects a route). Returns "" if undeterminable.
func primaryInterface() string {
	conn, err := stdnet.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*stdnet.UDPAddr)
	if !ok {
		return ""
	}
	ifaces, err := stdnet.Interfaces()
	if err != nil {
		return ""
	}
	for _, ifi := range ifaces {
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if n, ok := a.(*stdnet.IPNet); ok && n.IP.Equal(local.IP) {
				return ifi.Name
			}
		}
	}
	return ""
}

// rate converts a counter delta into bytes/sec, guarding against counter resets
// (interface restart, wraparound) which would otherwise produce a huge spike.
func rate(cur, prev uint64, dt float64) float64 {
	if cur < prev {
		return 0
	}
	return float64(cur-prev) / dt
}

// loopbackNames returns the set of loopback interface names so they can be
// excluded from "all interfaces" totals.
func loopbackNames() map[string]bool {
	set := make(map[string]bool)
	ifaces, err := stdnet.Interfaces()
	if err != nil {
		return set
	}
	for _, i := range ifaces {
		if i.Flags&stdnet.FlagLoopback != 0 {
			set[i.Name] = true
		}
	}
	return set
}
