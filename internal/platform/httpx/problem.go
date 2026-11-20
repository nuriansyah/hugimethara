// Package httpx contains HTTP primitives shared by every domain: RFC 9457
// problem responses, JSON helpers, pagination, and middleware.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/nuriansyah/hugimethara/internal/platform/logger"
)

const problemTypeBase = "https://docs.hugimethara.dev/errors/"

// FieldError describes one invalid input field.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem is an RFC 9457 problem details body, extended with a stable code,
// the request id and field errors. It also implements error so services can
// return it directly.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Code      string       `json:"code"`
	Detail    string       `json:"detail,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
	// Extensions are extra top-level members (RFC 9457 §3.2), e.g. choices
	// the client must pick from.
	Extensions map[string]any `json:"-"`
	// RetryAfter, when set, is sent as the Retry-After header.
	RetryAfter time.Duration `json:"-"`
}

// problemConvertible lets domain errors render as problems without depending
// on this package's concrete error type.
type problemConvertible interface{ Problem() *Problem }

func (p *Problem) Error() string { return p.Code + ": " + p.Detail }

// NewProblem builds a problem whose type URI is derived from code.
func NewProblem(status int, code, title, detail string) *Problem {
	return &Problem{Type: problemTypeBase + code, Title: title, Status: status, Code: code, Detail: detail}
}

func BadRequest(detail string) *Problem {
	return NewProblem(http.StatusBadRequest, "bad_request", "Bad request", detail)
}

func NotFound(detail string) *Problem {
	return NewProblem(http.StatusNotFound, "not_found", "Not found", detail)
}

func Validation(errs ...FieldError) *Problem {
	p := NewProblem(http.StatusUnprocessableEntity, "validation_failed", "Validation failed",
		"One or more fields are invalid.")
	p.Errors = errs
	return p
}

// WriteError renders err as a problem. Unknown errors become an opaque 500 and
// are logged with the request id; their message is never sent to the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var p *Problem
	var pc problemConvertible
	if errors.As(err, &pc) {
		p = pc.Problem()
	} else if !errors.As(err, &p) {
		slog.ErrorContext(r.Context(), "unhandled error", "err", err, "path", r.URL.Path)
		p = NewProblem(http.StatusInternalServerError, "internal_error", "Internal server error",
			"An unexpected error occurred. Quote the request id when contacting support.")
	}
	out := *p
	out.RequestID = logging.RequestID(r.Context())
	if out.RetryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(out.RetryAfter.Seconds())+1))
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(out.Status)
	_ = json.NewEncoder(w).Encode(out.body())
}

// body merges extensions into the serialized problem.
func (p Problem) body() any {
	if len(p.Extensions) == 0 {
		return p
	}
	type plain Problem
	b, _ := json.Marshal(plain(p))
	m := map[string]any{}
	_ = json.Unmarshal(b, &m)
	for k, v := range p.Extensions {
		if _, exists := m[k]; !exists {
			m[k] = v
		}
	}
	return m
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
