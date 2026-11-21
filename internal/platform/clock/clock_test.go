package clock

import (
	"testing"
	"time"
)

func TestManualClock(t *testing.T) {
	start := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	c := NewManual(start)
	c.Advance(time.Minute)
	if got := c.Now(); !got.Equal(start.Add(time.Minute)) {
		t.Fatalf("Now() = %v", got)
	}
	if (System{}).Now().Location() != time.UTC {
		t.Fatal("system clock must be UTC")
	}
}
