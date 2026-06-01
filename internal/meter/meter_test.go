package meter

import (
	"context"
	"testing"
	"time"

	psnet "github.com/shirou/gopsutil/v4/net"
)

func TestRate(t *testing.T) {
	tests := []struct {
		name      string
		cur, prev uint64
		dt        float64
		want      float64
	}{
		{"steady", 2000, 1000, 1, 1000},
		{"half second", 1500, 1000, 0.5, 1000},
		{"counter reset", 500, 1000, 1, 0},
		{"no change", 1000, 1000, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rate(tt.cur, tt.prev, tt.dt); got != tt.want {
				t.Errorf("rate(%d, %d, %v) = %v, want %v", tt.cur, tt.prev, tt.dt, got, tt.want)
			}
		})
	}
}

func stubCounters(t *testing.T) {
	counterFunc = func() ([]psnet.IOCountersStat, error) {
		return []psnet.IOCountersStat{
			{Name: "lo", BytesRecv: 100, BytesSent: 100},
			{Name: "eth0", BytesRecv: 10, BytesSent: 1},
			{Name: "wlan0", BytesRecv: 20, BytesSent: 2},
		}, nil
	}
	prev := primaryFunc
	t.Cleanup(func() {
		counterFunc = func() ([]psnet.IOCountersStat, error) { return psnet.IOCounters(true) }
		primaryFunc = prev
	})
}

func TestTotalsCountsPrimaryInterface(t *testing.T) {
	stubCounters(t)
	primaryFunc = func() string { return "wlan0" } // the internet-facing one

	m := &Meter{} // auto mode
	if down, up := m.totals(); down != 20 || up != 2 {
		t.Errorf("primary interface: got down=%d up=%d, want 20/2 (wlan0 only)", down, up)
	}
}

func TestTotalsConfiguredInterfaceWins(t *testing.T) {
	stubCounters(t)
	primaryFunc = func() string { return "wlan0" }

	m := &Meter{iface: "eth0"} // explicit choice overrides auto
	if down, up := m.totals(); down != 10 || up != 1 {
		t.Errorf("configured interface: got down=%d up=%d, want 10/1 (eth0)", down, up)
	}
}

func TestTotalsFallsBackToSumNonLoopback(t *testing.T) {
	stubCounters(t)
	primaryFunc = func() string { return "" } // primary undeterminable

	m := &Meter{loopback: map[string]bool{"lo": true}}
	if down, up := m.totals(); down != 30 || up != 3 {
		t.Errorf("fallback sum: got down=%d up=%d, want 30/3 (eth0+wlan0)", down, up)
	}
}

func TestRunBaselineThenRate(t *testing.T) {
	recv := uint64(0)
	counterFunc = func() ([]psnet.IOCountersStat, error) {
		recv += 1000 // +1000 bytes received between every reading
		return []psnet.IOCountersStat{{Name: "eth0", BytesRecv: recv}}, nil
	}
	t.Cleanup(func() {
		counterFunc = func() ([]psnet.IOCountersStat, error) { return psnet.IOCounters(true) }
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	m := New(20*time.Millisecond, "eth0")
	samples := m.Run(ctx)

	s := <-samples // first tick after baseline
	if s.Down <= 0 {
		t.Fatalf("expected positive download rate, got %v", s.Down)
	}
}
