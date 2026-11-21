package services

import (
	"encoding/json"
	"fmt"

	"cel.dev/cel-go/cel"
	"github.com/google/uuid"

	compositionmodel "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
)

func ValidateDraft(draft map[string]any) error {
	if _, ok := draft["steps"].([]any); !ok {
		return fmt.Errorf("composition steps must be an array")
	}
	raw, err := json.Marshal(draft)
	if err != nil {
		return fmt.Errorf("composition draft must be valid JSON")
	}
	var def compositionmodel.Definition
	if err := json.Unmarshal(raw, &def); err != nil {
		return fmt.Errorf("composition draft has an invalid shape")
	}
	if len(def.Steps) > compositionmodel.MaxSteps {
		return fmt.Errorf("composition can contain at most %d steps", compositionmodel.MaxSteps)
	}
	if def.Limits.TimeoutMs < 0 || def.Limits.TimeoutMs > compositionmodel.MaxTimeoutMs {
		return fmt.Errorf("composition timeout must be between 0 and %d milliseconds", compositionmodel.MaxTimeoutMs)
	}
	if def.Limits.MaxConcurrency < 0 || def.Limits.MaxConcurrency > compositionmodel.MaxConcurrency {
		return fmt.Errorf("composition concurrency must be between 0 and %d", compositionmodel.MaxConcurrency)
	}
	if def.Limits.MaxConcurrency == 0 {
		def.Limits.MaxConcurrency = compositionmodel.MaxConcurrency
	}
	if def.Response.MappingID != "" {
		if _, err := uuid.Parse(def.Response.MappingID); err != nil || def.Response.MappingVersion < 1 {
			return fmt.Errorf("response mapping requires a valid mapping_id and positive mapping_version")
		}
	}
	steps := make(map[string]compositionmodel.Step, len(def.Steps))
	for _, current := range def.Steps {
		if !compositionmodel.CompositionSlug.MatchString(current.ID) {
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
		if current.TimeoutMs < 0 || current.TimeoutMs > compositionmodel.MaxStepTimeoutMs {
			return fmt.Errorf("step %q timeout must be between 0 and %d milliseconds", current.ID, compositionmodel.MaxStepTimeoutMs)
		}
		if current.Retries < 0 || current.Retries > compositionmodel.MaxRetries {
			return fmt.Errorf("step %q retries must be between 0 and %d", current.ID, compositionmodel.MaxRetries)
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
			if len(current.When) > compositionmodel.MaxWhenLength {
				return fmt.Errorf("step %q when expression exceeds %d characters", current.ID, compositionmodel.MaxWhenLength)
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
