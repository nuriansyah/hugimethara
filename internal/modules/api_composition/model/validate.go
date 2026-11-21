package model

import "regexp"

const (
	MaxSteps         = 25
	MaxConcurrency   = 8
	MaxTimeoutMs     = 30000
	MaxStepTimeoutMs = 20000
	MaxRetries       = 5
	MaxWhenLength    = 1024
)

var CompositionSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type Definition struct {
	Steps    []Step `json:"steps"`
	Response struct {
		MappingID      string `json:"mapping_id,omitempty"`
		MappingVersion int    `json:"mapping_version,omitempty"`
	} `json:"response"`
	Limits struct {
		TimeoutMs      int `json:"timeout_ms,omitempty"`
		MaxConcurrency int `json:"max_concurrency,omitempty"`
	} `json:"limits"`
}

type Step struct {
	ID           string   `json:"id"`
	DependsOn    []string `json:"depends_on,omitempty"`
	When         string   `json:"when,omitempty"`
	OnError      string   `json:"on_error,omitempty"`
	Default      any      `json:"default,omitempty"`
	FallbackStep string   `json:"fallback_step,omitempty"`
	TimeoutMs    int      `json:"timeout_ms,omitempty"`
	Retries      int      `json:"retries,omitempty"`
	Call         struct {
		ServiceID  string         `json:"service_id"`
		EndpointID string         `json:"endpoint_id"`
		Input      map[string]any `json:"input,omitempty"`
	} `json:"call"`
}
