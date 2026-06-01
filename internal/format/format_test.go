package format

import "testing"

func TestFull(t *testing.T) {
	tests := []struct {
		bps  float64
		unit Unit
		want string
	}{
		{0, Bps, "0 bit/s"},
		{125, Bps, "1000 bit/s"},              // 125 B/s = 1000 bit/s
		{125_000, Kbps, "1000 Kbit/s"},        // = 1,000,000 bit/s
		{125_000, Mbps, "1.0 Mbit/s"},         // = 1 Mbit/s
		{500, Kbps, "4.0 Kbit/s"},             // 500 B/s = 4000 bit/s
		{125_000_000, Gbps, "1.0 Gbit/s"},     // = 1 Gbit/s
		{125_000_000_000, Tbps, "1.0 Tbit/s"}, // = 1 Tbit/s
	}
	for _, tt := range tests {
		if got := Full(tt.bps, tt.unit); got != tt.want {
			t.Errorf("Full(%v, %v) = %q, want %q", tt.bps, tt.unit, got, tt.want)
		}
	}
}

func TestShort(t *testing.T) {
	tests := []struct {
		bps  float64
		unit Unit
		want string
	}{
		{0, Kbps, "0Kbps"},
		{125, Bps, "1000bps"},
		{125_000, Kbps, "1000Kbps"}, // full unit shown, not just "K"
		{125_000, Mbps, "1.0Mbps"},
		{125_000_000, Gbps, "1.0Gbps"},
		{125_000_000_000, Tbps, "1.0Tbps"},
	}
	for _, tt := range tests {
		if got := Short(tt.bps, tt.unit); got != tt.want {
			t.Errorf("Short(%v, %v) = %q, want %q", tt.bps, tt.unit, got, tt.want)
		}
	}
}

func TestShortRounding(t *testing.T) {
	// num() picks precision from the rounded value: <10 -> one decimal,
	// >=10 -> none, and a rate that rounds to zero -> "0".
	tests := []struct {
		bps  float64
		unit Unit
		want string
	}{
		{1_242_500, Mbps, "9.9Mbps"}, // 9.94 -> "9.9"
		{1_245_000, Mbps, "10Mbps"},  // 9.96 rounds to 10 -> no decimal
		{1_250_000, Mbps, "10Mbps"},  // exactly 10
		{5_000, Mbps, "0Mbps"},       // 0.04 -> rounds to zero -> "0"
	}
	for _, tt := range tests {
		if got := Short(tt.bps, tt.unit); got != tt.want {
			t.Errorf("Short(%v, %v) = %q, want %q", tt.bps, tt.unit, got, tt.want)
		}
	}
}

func TestUnitTextRoundTrip(t *testing.T) {
	for _, u := range Units() {
		b, err := u.MarshalText()
		if err != nil {
			t.Fatal(err)
		}
		var got Unit
		if err := got.UnmarshalText(b); err != nil {
			t.Fatalf("%v: %v", u, err)
		}
		if got != u {
			t.Errorf("round trip %v -> %q -> %v", u, b, got)
		}
	}
	var u Unit
	if err := u.UnmarshalText([]byte("nonsense")); err == nil {
		t.Error("expected error for unknown unit")
	}
}
