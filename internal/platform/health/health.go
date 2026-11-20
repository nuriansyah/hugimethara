// Package health serves /healthz (liveness) and /readyz (readiness).
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// Check verifies one dependency. It must honour ctx cancellation.
type Check func(ctx context.Context) error

// Handler aggregates named readiness checks.
type Handler struct {
	checks  map[string]Check
	timeout time.Duration
}

func New(timeout time.Duration) *Handler {
	if timeout == 0 {
		timeout = 2 * time.Second
	}
	return &Handler{checks: map[string]Check{}, timeout: timeout}
}

// Add registers a readiness check.
func (h *Handler) Add(name string, c Check) { h.checks[name] = c }

// CheckResult is the outcome of one check.
type CheckResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Report is the aggregate readiness outcome.
type Report struct {
	Status string                 `json:"status"`
	Checks map[string]CheckResult `json:"checks,omitempty"`
}

// OK reports whether every check passed.
func (r Report) OK() bool { return r.Status == "ok" }

// Live always succeeds while the process can serve HTTP.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	write(w, http.StatusOK, Report{Status: "ok"})
}

// Ready serves the readiness report: 200 when all checks pass, else 503.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	rep := h.Evaluate(r.Context())
	status := http.StatusOK
	if !rep.OK() {
		status = http.StatusServiceUnavailable
	}
	write(w, status, rep)
}

// Evaluate runs all checks concurrently within the handler timeout. Error
// text is the short, credential-free description produced by each check.
func (h *Handler) Evaluate(ctx context.Context) Report {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	rep := Report{Status: "ok", Checks: make(map[string]CheckResult, len(h.checks))}
	for name, c := range h.checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := CheckResult{Status: "ok"}
			if err := c(ctx); err != nil {
				res = CheckResult{Status: "fail", Error: err.Error()}
			}
			mu.Lock()
			rep.Checks[name] = res
			if res.Status != "ok" {
				rep.Status = "fail"
			}
			mu.Unlock()
		}()
	}
	wg.Wait()
	return rep
}

func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
