package model

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// ResourceType is the authz resource type.
const ResourceType = "organization"

// Organization is a tenant.
type Organization struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	Status    string
	Settings  Settings
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Settings are org-wide, typed configuration values stored as JSONB.
type Settings struct {
	SessionIdleTimeoutMinutes   int `json:"session_idle_timeout_minutes"`
	SessionAbsoluteTimeoutHours int `json:"session_absolute_timeout_hours"`
}

// Bounds and defaults for settings (ADR-0006).
const (
	DefaultIdleMinutes   = 480
	MinIdleMinutes       = 5
	MaxIdleMinutes       = 1440
	DefaultAbsoluteHours = 168
	MinAbsoluteHours     = 1
	MaxAbsoluteHours     = 720
)

// DefaultEnvironments are created with every organization.
var DefaultEnvironments = []struct {
	Slug, Name   string
	IsProduction bool
}{
	{"development", "Development", false},
	{"staging", "Staging", false},
	{"production", "Production", true},
}

// DefaultSettings returns the settings a new organization starts with.
func DefaultSettings() Settings {
	return Settings{SessionIdleTimeoutMinutes: DefaultIdleMinutes, SessionAbsoluteTimeoutHours: DefaultAbsoluteHours}
}

// ParseSettings decodes stored settings, filling defaults for missing values.
func ParseSettings(raw []byte) Settings {
	s := DefaultSettings()
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &s)
	}
	if s.SessionIdleTimeoutMinutes == 0 {
		s.SessionIdleTimeoutMinutes = DefaultIdleMinutes
	}
	if s.SessionAbsoluteTimeoutHours == 0 {
		s.SessionAbsoluteTimeoutHours = DefaultAbsoluteHours
	}
	return s
}

// IdleTimeout and AbsoluteTimeout return the session lifetimes.
func (s Settings) IdleTimeout() time.Duration {
	return time.Duration(s.SessionIdleTimeoutMinutes) * time.Minute
}

func (s Settings) AbsoluteTimeout() time.Duration {
	return time.Duration(s.SessionAbsoluteTimeoutHours) * time.Hour
}

// Validate checks bounds and returns field errors with the given prefix.
func (s Settings) Validate(prefix string) []problem.FieldError {
	var errs []problem.FieldError
	if s.SessionIdleTimeoutMinutes < MinIdleMinutes || s.SessionIdleTimeoutMinutes > MaxIdleMinutes {
		errs = append(errs, problem.FieldError{Field: prefix + "session_idle_timeout_minutes", Code: "out_of_range",
			Message: "Idle timeout must be between 5 and 1440 minutes."})
	}
	if s.SessionAbsoluteTimeoutHours < MinAbsoluteHours || s.SessionAbsoluteTimeoutHours > MaxAbsoluteHours {
		errs = append(errs, problem.FieldError{Field: prefix + "session_absolute_timeout_hours", Code: "out_of_range",
			Message: "Absolute session lifetime must be between 1 and 720 hours."})
	}
	if len(errs) == 0 && s.IdleTimeout() > s.AbsoluteTimeout() {
		errs = append(errs, problem.FieldError{Field: prefix + "session_idle_timeout_minutes", Code: "exceeds_absolute",
			Message: "Idle timeout cannot be longer than the absolute session lifetime."})
	}
	return errs
}

var (
	slugRe     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)
	reserved   = map[string]bool{"api": true, "admin": true, "www": true, "app": true, "hugimethara": true, "system": true, "login": true, "signup": true}
	nonSlugish = regexp.MustCompile(`[^a-z0-9]+`)
)

// ValidateSlug returns a field error if slug is not acceptable.
func ValidateSlug(field, slug string) *problem.FieldError {
	if !slugRe.MatchString(slug) || strings.Contains(slug, "--") {
		return &problem.FieldError{Field: field, Code: "invalid_slug",
			Message: "Use 3–40 lowercase letters, digits or single hyphens, starting and ending with a letter or digit."}
	}
	if reserved[slug] {
		return &problem.FieldError{Field: field, Code: "reserved", Message: "This address is reserved. Choose another."}
	}
	return nil
}

// ValidateName returns a field error if name is not acceptable.
func ValidateName(field, name string) *problem.FieldError {
	n := len([]rune(strings.TrimSpace(name)))
	if n < 2 || n > 100 {
		return &problem.FieldError{Field: field, Code: "invalid_length", Message: "Name must be 2–100 characters."}
	}
	return nil
}

// SuggestSlug derives a slug candidate from a display name.
func SuggestSlug(name string) string {
	s := strings.Trim(nonSlugish.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if len(s) > 40 {
		s = strings.TrimRight(s[:40], "-")
	}
	return s
}
