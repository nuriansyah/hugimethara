package id

import (
	"testing"
	"time"
)

func TestNewIsVersion7AndTimeOrdered(t *testing.T) {
	a := New()
	time.Sleep(2 * time.Millisecond)
	b := New()
	if a.Version() != 7 || b.Version() != 7 {
		t.Fatalf("want v7, got %d and %d", a.Version(), b.Version())
	}
	if a.String() >= b.String() {
		t.Fatalf("ids are not time ordered: %s >= %s", a, b)
	}
}
