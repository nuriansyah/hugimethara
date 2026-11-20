package composition

import (
	"encoding/json"
	"fmt"
	"regexp"

	"cel.dev/cel-go/cel"
	"github.com/google/uuid"
)

const (
	maxSteps         = 25
	maxConcurrency   = 8
	maxTimeoutMs     = 30000
	maxStepTimeoutMs = 20000
	maxRetries       = 5
	maxWhenLength    = 1024
)

var compositionSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type definition struct {
	Steps    []step `json:"steps"`
	Response struct {
		MappingID      string `json:"mapping_id,omitempty"`
		MappingVersion int    `json:"mapping_version,omitempty"`
	} `json:"response"`
	Limits struct {
		TimeoutMs      int `json:"timeout_ms,omitempty"`
		MaxConcurrency int `json:"max_concurrency,omitempty"`
	} `json:"limits"`
}

type step struct {
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

func ValidateDraft(draft map[string]any) error {
	if _, ok := draft["steps"].([]any); !ok {
		return fmt.Errorf("composition steps must be an array")
	}
	raw, err := json.Marshal(draft)
	if err != nil {
		return fmt.Errorf("composition draft must be valid JSON")
	}
	var def definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return fmt.Errorf("composition draft has an invalid shape")
	}
	if len(def.Steps) > maxSteps {
		return fmt.Errorf("composition can contain at most %d steps", maxSteps)
	}
	if def.Limits.TimeoutMs < 0 || def.Limits.TimeoutMs > maxTimeoutMs {
		return fmt.Errorf("composition timeout must be between 0 and %d milliseconds", maxTimeoutMs)
	}
	if def.Limits.MaxConcurrency < 0 || def.Limits.MaxConcurrency > maxConcurrency {
		return fmt.Errorf("composition concurrency must be between 0 and %d", maxConcurrency)
	}
	if def.Limits.MaxConcurrency == 0 {
		def.Limits.MaxConcurrency = maxConcurrency
	}
	if def.Response.MappingID != "" {
		if _, err := uuid.Parse(def.Response.MappingID); err != nil || def.Response.MappingVersion < 1 {
			return fmt.Errorf("response mapping requires a valid mapping_id and positive mapping_version")
		}
	}
	steps := make(map[string]step, len(def.Steps))
	for _, current := range def.Steps {
		if !compositionSlug.MatchString(current.ID) {
			return fmt.Errorf("step ids must be lowercase letters, digits, or hyphens and begin with a letter or digit")
		}
		if _, exists := steps[current.ID]; exists {
			return fmt.Errorf("step id %q is duplicated", current.ID)
		}
		if _, err := uuid.Parse(current.Call.ServiceID); err != nil {
			return fmt.Errorf("step %q requires a valid service_id", current.ID)
		}
		if _, err := uuid.Parse(current.Call.EndpointID); err != nil {
			return fmt.Errorf("step %q requires a valid endpoint_id", current.ID)
		}
		if current.TimeoutMs < 0 || current.TimeoutMs > maxStepTimeoutMs {
			return fmt.Errorf("step %q timeout must be between 0 and %d milliseconds", current.ID, maxStepTimeoutMs)
		}
		if current.Retries < 0 || current.Retries > maxRetries {
			return fmt.Errorf("step %q retries must be between 0 and %d", current.ID, maxRetries)
		}
		switch current.OnError {
		case "", "fail", "null", "default", "fallback_step":
		default:
			return fmt.Errorf("step %q has an unsupported on_error policy", current.ID)
		}
		if current.OnError == "fallback_step" && current.FallbackStep == "" {
			return fmt.Errorf("step %q requires fallback_step when on_error is fallback_step", current.ID)
		}
		if current.When != "" {
			if len(current.When) > maxWhenLength {
				return fmt.Errorf("step %q when expression exceeds %d characters", current.ID, maxWhenLength)
			}
			if _, err := compileCondition(current.When); err != nil {
				return fmt.Errorf("step %q has invalid when expression: %w", current.ID, err)
			}
		}
		steps[current.ID] = current
	}
	for _, current := range def.Steps {
		seen := map[string]bool{}
		for _, dep := range current.DependsOn {
			if dep == current.ID {
				return fmt.Errorf("step %q cannot depend on itself", current.ID)
			}
			if _, ok := steps[dep]; !ok {
				return fmt.Errorf("step %q depends on unknown step %q", current.ID, dep)
			}
			if seen[dep] {
				return fmt.Errorf("step %q lists dependency %q more than once", current.ID, dep)
			}
			seen[dep] = true
		}
		if current.OnError == "fallback_step" {
			if _, ok := steps[current.FallbackStep]; !ok || current.FallbackStep == current.ID {
				return fmt.Errorf("step %q references an invalid fallback step", current.ID)
			}
			dependency := false
			for _, dep := range current.DependsOn {
				if dep == current.FallbackStep {
					dependency = true
					break
				}
			}
			if !dependency {
				return fmt.Errorf("step %q must depend on its fallback step", current.ID)
			}
		}
	}
	visited := map[string]bool{}
	active := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if active[id] {
			return fmt.Errorf("composition dependencies contain a cycle")
		}
		if visited[id] {
			return nil
		}
		active[id] = true
		for _, dep := range steps[id].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		delete(active, id)
		visited[id] = true
		return nil
	}
	for id := range steps {
		if err := visit(id); err != nil {
			return err
		}
	}
	return nil
}

func compileCondition(expression string) (*cel.Ast, error) {
	env, err := cel.NewEnv(cel.Variable("steps", cel.DynType), cel.Variable("request", cel.DynType))
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	return ast, nil
}
