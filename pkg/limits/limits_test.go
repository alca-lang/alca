package limits_test

import (
	"testing"

	"github.com/alca-lang/alca/pkg/limits"
)

func TestMeterAccountsLiveAndCumulative(t *testing.T) {
	meter := &limits.Meter{}
	meter.Reset()
	meter.SetLimits(1000, 0)

	meter.Charge(400)
	if got := meter.Live(); got != 400 {
		t.Fatalf("live = %d, want 400", got)
	}
	if got := meter.Allocated(); got != 400 {
		t.Fatalf("allocated = %d, want 400", got)
	}
	if meter.Exceeded() {
		t.Fatal("meter must not report an exceeded limit yet")
	}

	meter.Refund(100)
	if got := meter.Live(); got != 300 {
		t.Fatalf("live after refund = %d, want 300", got)
	}
	if got := meter.Allocated(); got != 400 {
		t.Fatalf("allocated after refund = %d, want 400", got)
	}

	meter.Charge(800)
	if !meter.Exceeded() {
		t.Fatal("meter must report an exceeded live limit")
	}
	if got := meter.Peak(); got != 1100 {
		t.Fatalf("peak = %d, want 1100", got)
	}
}

func TestMeterAllocationLimit(t *testing.T) {
	meter := &limits.Meter{}
	meter.Reset()
	meter.SetLimits(0, 100)

	meter.ChargeTransient(60)
	if meter.Exceeded() {
		t.Fatal("meter must not report an exceeded allocation limit yet")
	}
	meter.ChargeTransient(60)
	if !meter.Exceeded() {
		t.Fatal("meter must report an exceeded allocation limit")
	}
	if msg := meter.Message(); msg == "" {
		t.Fatal("an exceeded meter must describe the limit")
	}
}

func TestMeterResetClearsState(t *testing.T) {
	meter := &limits.Meter{}
	meter.Reset()
	meter.SetLimits(10, 0)
	meter.Charge(100)
	if !meter.Exceeded() {
		t.Fatal("meter must report an exceeded limit")
	}
	meter.Reset()
	if meter.Exceeded() {
		t.Fatal("a reset meter must not report an exceeded limit")
	}
	if got := meter.Live(); got != 0 {
		t.Fatalf("live after reset = %d, want 0", got)
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		raw  string
		want int64
	}{
		{"0", 0},
		{"512", 512},
		{"64kb", 64 * 1024},
		{"256mb", 256 * 1024 * 1024},
		{"2gb", 2 * 1024 * 1024 * 1024},
		{"1B", 1},
	}
	for _, tc := range cases {
		got, err := limits.ParseSize(tc.raw)
		if err != nil {
			t.Fatalf("ParseSize(%q): %v", tc.raw, err)
		}
		if got != tc.want {
			t.Fatalf("ParseSize(%q) = %d, want %d", tc.raw, got, tc.want)
		}
	}
	if _, err := limits.ParseSize("nonsense"); err == nil {
		t.Fatal("ParseSize must reject an invalid size")
	}
}

func TestTickReportsPeriodically(t *testing.T) {
	meter := &limits.Meter{}
	checks := 0
	for i := 0; i < limits.CheckInterval*3; i++ {
		if meter.Tick() {
			checks++
		}
	}
	if checks != 3 {
		t.Fatalf("checks = %d, want 3", checks)
	}
}
