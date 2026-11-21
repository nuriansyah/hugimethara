package services

import (
	"encoding/json"
	"fmt"

	"cel.dev/cel-go/cel"

	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
)

type evaluator struct {
	iterations int
}

func ValidateDraft(draft map[string]any) error {
	rules, ok := draft["rules"].([]any)
	if !ok {
		return fmt.Errorf("draft.rules must be an array")
	}
	if len(rules) > mappingmodel.MaxRules {
		return fmt.Errorf("mapping has more than %d rules", mappingmodel.MaxRules)
	}
	count := 0
	var walk func([]any) error
	walk = func(items []any) error {
		for _, raw := range items {
			count++
			if count > mappingmodel.MaxRules {
				return fmt.Errorf("mapping has more than %d rules", mappingmodel.MaxRules)
			}
			rule, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("mapping rule must be an object")
			}
			if rule["target"] == nil && rule["foreach"] == nil {
				return fmt.Errorf("mapping rule requires target or foreach")
			}
			for _, key := range []string{"expr", "when"} {
				if text, exists := rule[key].(string); exists && text != "" {
					if _, err := compileCEL(text); err != nil {
						return fmt.Errorf("invalid CEL in %s: %w", key, err)
					}
				}
			}
			if nested, ok := rule["rules"].([]any); ok {
				if err := walk(nested); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(rules)
}

func Apply(draft map[string]any, input any) (any, error) {
	if err := ValidateDraft(draft); err != nil {
		return nil, err
	}
	e := &evaluator{}
	rules, _ := draft["rules"].([]any)
	out, err := e.applyRules(rules, input, nil)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	if len(raw) > mappingmodel.MaxOutputBytes {
		return nil, fmt.Errorf("mapping output exceeds 1 MiB")
	}
	return out, nil
}

func (e *evaluator) applyRules(rules []any, src any, root any) (map[string]any, error) {
	out := make(map[string]any)
	for _, raw := range rules {
		e.iterations++
		if e.iterations > mappingmodel.MaxIterations {
			return nil, fmt.Errorf("mapping exceeded %d iterations", mappingmodel.MaxIterations)
		}
		rule, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("mapping rule must be an object")
		}
		if text, ok := rule["when"].(string); ok && text != "" {
			ok, err := evalCELBool(text, src)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
		}
		if foreach, ok := rule["foreach"].(string); ok && foreach != "" {
			items, err := mappingmodel.JsonPath(src, foreach)
			if err != nil {
				return nil, err
			}
			arr, ok := items.([]any)
			if !ok {
				continue
			}
			target, _ := rule["target"].(string)
			nested, _ := rule["rules"].([]any)
			converted := make([]any, 0, len(arr))
			for _, item := range arr {
				e.iterations++
				if e.iterations > mappingmodel.MaxIterations {
					return nil, fmt.Errorf("mapping exceeded %d iterations", mappingmodel.MaxIterations)
				}
				row, err := e.applyRules(nested, item, src)
				if err != nil {
					return nil, err
				}
				converted = append(converted, row)
			}
			mappingmodel.SetTarget(out, target, converted)
			continue
		}
		target, _ := rule["target"].(string)
		if target == "" {
			continue
		}
		var value any
		var err error
		if expr, ok := rule["expr"].(string); ok && expr != "" {
			value, err = evalCEL(expr, src)
		} else if source, ok := rule["source"].(string); ok && source != "" {
			value, err = mappingmodel.JsonPath(src, source)
		} else if def, exists := rule["default"]; exists {
			value = def
		} else {
			value = nil
		}
		if err != nil {
			return nil, err
		}
		if value == nil {
			if def, exists := rule["default"]; exists {
				value = def
			}
		}
		value, err = mappingmodel.ApplyTransforms(value, rule)
		if err != nil {
			return nil, err
		}
		if rule["omit_if_null"] == true && value == nil {
			continue
		}
		mappingmodel.SetTarget(out, target, value)
	}
	return out, nil
}

func compileCEL(expression string) (*cel.Ast, error) {
	env, err := cel.NewEnv(cel.Variable("src", cel.DynType))
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	return ast, nil
}

func evalCEL(expression string, src any) (any, error) {
	env, err := cel.NewEnv(cel.Variable("src", cel.DynType))
	if err != nil {
		return nil, err
	}
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}
	program, err := env.Program(ast)
	if err != nil {
		return nil, err
	}
	out, _, err := program.Eval(map[string]any{"src": src})
	if err != nil {
		return nil, err
	}
	value := out.Value()
	if value == nil {
		return nil, nil
	}
	return value, nil
}

func evalCELBool(expression string, src any) (bool, error) {
	value, err := evalCEL(expression, src)
	if err != nil {
		return false, err
	}
	ok, _ := value.(bool)
	return ok, nil
}
