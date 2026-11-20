package mapping

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"cel.dev/cel-go/cel"
)

const (
	maxRules       = 200
	maxIterations  = 10000
	maxOutputBytes = 1024 * 1024
)

type evaluator struct {
	iterations int
}

func ValidateDraft(draft map[string]any) error {
	rules, ok := draft["rules"].([]any)
	if !ok {
		return fmt.Errorf("draft.rules must be an array")
	}
	if len(rules) > maxRules {
		return fmt.Errorf("mapping has more than %d rules", maxRules)
	}
	count := 0
	var walk func([]any) error
	walk = func(items []any) error {
		for _, raw := range items {
			count++
			if count > maxRules {
				return fmt.Errorf("mapping has more than %d rules", maxRules)
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
	if len(raw) > maxOutputBytes {
		return nil, fmt.Errorf("mapping output exceeds 1 MiB")
	}
	return out, nil
}

func (e *evaluator) applyRules(rules []any, src any, root any) (map[string]any, error) {
	out := make(map[string]any)
	for _, raw := range rules {
		e.iterations++
		if e.iterations > maxIterations {
			return nil, fmt.Errorf("mapping exceeded %d iterations", maxIterations)
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
			items, err := jsonPath(src, foreach)
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
				if e.iterations > maxIterations {
					return nil, fmt.Errorf("mapping exceeded %d iterations", maxIterations)
				}
				row, err := e.applyRules(nested, item, src)
				if err != nil {
					return nil, err
				}
				converted = append(converted, row)
			}
			setTarget(out, target, converted)
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
			value, err = jsonPath(src, source)
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
		value, err = applyTransforms(value, rule)
		if err != nil {
			return nil, err
		}
		if rule["omit_if_null"] == true && value == nil {
			continue
		}
		setTarget(out, target, value)
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

func jsonPath(src any, path string) (any, error) {
	if path == "" || path == "$" {
		return src, nil
	}
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("JSONPath must start with $")
	}
	cur := src
	rest := strings.TrimPrefix(path, "$")
	for len(rest) > 0 {
		if rest[0] == '.' {
			rest = rest[1:]
			i := strings.IndexAny(rest, ".[")
			key := rest
			if i >= 0 {
				key, rest = rest[:i], rest[i:]
			} else {
				rest = ""
			}
			if key == "" {
				return nil, fmt.Errorf("invalid JSONPath %q", path)
			}
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, nil
			}
			cur, ok = m[key]
			if !ok {
				return nil, nil
			}
		} else if rest[0] == '[' {
			end := strings.Index(rest, "]")
			if end < 0 {
				return nil, fmt.Errorf("invalid JSONPath %q", path)
			}
			token := rest[1:end]
			rest = rest[end+1:]
			arr, ok := cur.([]any)
			if !ok {
				return nil, nil
			}
			if token == "*" {
				return arr, nil
			}
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(arr) {
				return nil, nil
			}
			cur = arr[index]
		} else {
			return nil, fmt.Errorf("invalid JSONPath %q", path)
		}
	}
	return cur, nil
}

// ResolveJSONPath evaluates the bounded JSONPath subset supported by mappings.
func ResolveJSONPath(src any, path string) (any, error) {
	return jsonPath(src, path)
}

func applyTransforms(value any, rule map[string]any) (any, error) {
	raw, _ := rule["transforms"].([]any)
	for _, item := range raw {
		op, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := op["op"].(string)
		switch name {
		case "trim":
			if s, ok := value.(string); ok {
				value = strings.TrimSpace(s)
			}
		case "upper":
			if s, ok := value.(string); ok {
				value = strings.ToUpper(s)
			}
		case "lower":
			if s, ok := value.(string); ok {
				value = strings.ToLower(s)
			}
		case "to_string":
			if value != nil {
				value = fmt.Sprint(value)
			}
		case "to_number":
			n, err := toNumber(value)
			if err != nil {
				return nil, fmt.Errorf("to_number failed: %w", err)
			}
			value = n
		case "to_bool":
			switch v := value.(type) {
			case bool:
				value = v
			case string:
				b, err := strconv.ParseBool(v)
				if err != nil {
					value = nil
				} else {
					value = b
				}
			default:
				value = nil
			}
		case "round":
			if n, ok := toFloat(value); ok {
				value = math.Round(n)
			}
		case "add":
			if n, ok := toFloat(value); ok {
				if v, ok := toFloat(op["value"]); ok {
					value = n + v
				}
			}
		case "multiply":
			if n, ok := toFloat(value); ok {
				if v, ok := toFloat(op["value"]); ok {
					value = n * v
				}
			}
		case "default":
			if value == nil {
				value = op["value"]
			}
		case "replace":
			if s, ok := value.(string); ok {
				from, _ := op["from"].(string)
				to, _ := op["to"].(string)
				value = strings.ReplaceAll(s, from, to)
			}
		case "substring":
			if s, ok := value.(string); ok {
				start, _ := op["start"].(float64)
				end, _ := op["end"].(float64)
				a, b := int(start), int(end)
				if a < 0 {
					a = 0
				}
				if b > len(s) {
					b = len(s)
				}
				if a <= b {
					value = s[a:b]
				}
			}
		case "split":
			if s, ok := value.(string); ok {
				sep, _ := op["separator"].(string)
				value = strings.Split(s, sep)
			}
		case "join":
			if arr, ok := value.([]any); ok {
				sep, _ := op["separator"].(string)
				parts := make([]string, len(arr))
				for i := range arr {
					parts[i] = fmt.Sprint(arr[i])
				}
				value = strings.Join(parts, sep)
			}
		case "date_format":
			if s, ok := value.(string); ok {
				layout, _ := op["layout"].(string)
				if layout == "" {
					layout = time.RFC3339
				}
				outLayout, _ := op["out_layout"].(string)
				if outLayout == "" {
					outLayout = "2006-01-02"
				}
				if t, err := time.Parse(layout, s); err == nil {
					value = t.Format(outLayout)
				}
			}
		case "null_as":
			if value == nil {
				value = op["value"]
			}
		case "omit_if_null":
			// handled after transformations
		}
	}
	return value, nil
}

func toNumber(value any) (any, error) {
	if n, ok := value.(float64); ok {
		return n, nil
	}
	if n, ok := value.(json.Number); ok {
		return n.Float64()
	}
	if s, ok := value.(string); ok {
		n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, err
		}
		return n, nil
	}
	return nil, fmt.Errorf("value is not numeric")
}

func toFloat(value any) (float64, bool) {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return 0, false
	}
	switch v.Kind() {
	case reflect.Float32, reflect.Float64:
		return v.Float(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(v.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(v.Uint()), true
	case reflect.String:
		n, err := strconv.ParseFloat(strings.TrimSpace(v.String()), 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func setTarget(out map[string]any, target string, value any) {
	target = strings.TrimPrefix(target, "$.")
	parts := strings.Split(target, ".")
	cur := out
	for i, part := range parts {
		part = strings.TrimSuffix(part, "[]")
		if i == len(parts)-1 {
			cur[part] = value
			return
		}
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			cur[part] = next
		}
		cur = next
	}
}
