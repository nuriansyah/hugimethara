package model

import (
	"net/http"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// ErrSlugTaken is returned when another organization already uses the slug.
var ErrSlugTaken = problem.NewProblem(http.StatusConflict, "slug_taken", "Address unavailable",
	"Another organization already uses this address. Choose a different one.")

func Diff(a, b Organization) map[string]any {
	ch := map[string]any{}
	if a.Name != b.Name {
		ch["name"] = map[string]any{"from": a.Name, "to": b.Name}
	}
	if a.Settings.SessionIdleTimeoutMinutes != b.Settings.SessionIdleTimeoutMinutes {
		ch["settings.session_idle_timeout_minutes"] = map[string]any{"from": a.Settings.SessionIdleTimeoutMinutes, "to": b.Settings.SessionIdleTimeoutMinutes}
	}
	if a.Settings.SessionAbsoluteTimeoutHours != b.Settings.SessionAbsoluteTimeoutHours {
		ch["settings.session_absolute_timeout_hours"] = map[string]any{"from": a.Settings.SessionAbsoluteTimeoutHours, "to": b.Settings.SessionAbsoluteTimeoutHours}
	}
	return ch
}

func Keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
