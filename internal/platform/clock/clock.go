// Package clock abstracts wall-clock time so that time-dependent domain logic
// (timeouts, retries, schedules, expiry) is deterministic under test.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time. Implementations must be safe for
// concurrent use.
type Clock interface {
	Now() time.Time
}

// System is the real clock. It always returns UTC so persisted timestamps and
// comparisons never depend on the host time zone.
type System struct{}

// Now returns the current UTC time.
func (System) Now() time.Time { return time.Now().UTC() }

// Manual is a controllable clock for tests.
type Manual struct {
	mu  sync.Mutex
	now time.Time
}

// NewManual returns a Manual clock set to t.
func NewManual(t time.Time) *Manual { return &Manual{now: t.UTC()} }

// Now returns the clock's current time.
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Advance moves the clock forward by d.
func (m *Manual) Advance(d time.Duration) {
	m.mu.Lock()
	m.now = m.now.Add(d)
	m.mu.Unlock()
}

// Set moves the clock to t.
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	m.now = t.UTC()
	m.mu.Unlock()
}
