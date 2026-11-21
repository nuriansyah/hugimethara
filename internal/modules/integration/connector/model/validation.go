package model

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
)

func ValidateSchemaValue(schema, value any, path string) error {
	rule, ok := schema.(map[string]any)
	if !ok {
		return fmt.Errorf("%s has an unsupported schema", path)
	}
	typeName, _ := rule["type"].(string)
	switch typeName {
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be an object", path)
		}
		required := stringSlice(rule["required"])
		for _, name := range required {
			if _, exists := object[name]; !exists {
				return fmt.Errorf("%s.%s is required", path, name)
			}
		}
		properties, _ := rule["properties"].(map[string]any)
		allowExtras, explicitlySet := rule["additionalProperties"].(bool)
		for name, child := range object {
			spec, exists := properties[name]
			if !exists {
				if explicitlySet && !allowExtras {
					return fmt.Errorf("%s.%s is not allowed", path, name)
				}
				continue
			}
			if err := ValidateSchemaValue(spec, child, path+"."+name); err != nil {
				return err
			}
		}
	case "array":
		array, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s must be an array", path)
		}
		if itemSchema, exists := rule["items"]; exists {
			for i, child := range array {
				if err := ValidateSchemaValue(itemSchema, child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s must be a string", path)
		}
		if format, _ := rule["format"].(string); format == "uri" {
			u, err := url.ParseRequestURI(text)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("%s must be an absolute URI", path)
			}
		}
	case "integer":
		number, ok := value.(float64)
		if !ok || number != float64(int64(number)) {
			return fmt.Errorf("%s must be an integer", path)
		}
		if err := checkNumericBounds(number, rule, path); err != nil {
			return err
		}
	case "number":
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s must be a number", path)
		}
		if err := checkNumericBounds(number, rule, path); err != nil {
			return err
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s must be a boolean", path)
		}
	case "":
		return fmt.Errorf("%s has no supported schema type", path)
	default:
		return fmt.Errorf("%s uses unsupported schema type %q", path, typeName)
	}
	if enum, ok := rule["enum"].([]any); ok {
		for _, allowed := range enum {
			if reflect.DeepEqual(allowed, value) {
				return nil
			}
		}
		return fmt.Errorf("%s is not an allowed value", path)
	}
	return nil
}

func checkNumericBounds(number float64, rule map[string]any, path string) error {
	if min, ok := rule["minimum"].(float64); ok && number < min {
		return fmt.Errorf("%s must be at least %v", path, min)
	}
	if max, ok := rule["maximum"].(float64); ok && number > max {
		return fmt.Errorf("%s must be at most %v", path, max)
	}
	return nil
}

func stringSlice(value any) []string {
	items, _ := value.([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		if text, ok := item.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func ContainsJSONString(items []byte, target string) bool {
	var values []string
	if json.Unmarshal(items, &values) != nil {
		return false
	}
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
