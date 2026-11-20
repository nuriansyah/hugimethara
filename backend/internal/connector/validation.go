package connector

import (
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strings"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/logging"
)

func validateConfig(schemaJSON, configJSON []byte) error {
	var schema, value any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("connector: stored config schema is invalid: %w", err)
	}
	if err := json.Unmarshal(configJSON, &value); err != nil {
		return httpx.Validation(httpx.FieldError{Field: "config", Code: "invalid_json", Message: "Configuration must be valid JSON."})
	}
	if err := validateSchemaValue(schema, value, "config"); err != nil {
		return httpx.Validation(httpx.FieldError{Field: "config", Code: "schema_invalid", Message: err.Error()})
	}
	if key := sensitiveConfigKey(value, "config"); key != "" {
		return httpx.Validation(httpx.FieldError{Field: key, Code: "use_secret_reference", Message: "Credentials must be stored as a credential reference, not in connector configuration."})
	}
	return nil
}

func validateSchemaValue(schema, value any, path string) error {
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
			if err := validateSchemaValue(spec, child, path+"."+name); err != nil {
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
				if err := validateSchemaValue(itemSchema, child, fmt.Sprintf("%s[%d]", path, i)); err != nil {
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

func containsJSONString(items []byte, target string) bool {
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

func sensitiveConfigKey(value any, path string) string {
	switch object := value.(type) {
	case map[string]any:
		for key, child := range object {
			if logging.IsSensitiveKey(key) {
				return path + "." + key
			}
			if found := sensitiveConfigKey(child, path+"."+key); found != "" {
				return found
			}
		}
	case []any:
		for i, child := range object {
			if found := sensitiveConfigKey(child, fmt.Sprintf("%s[%d]", path, i)); found != "" {
				return found
			}
		}
	}
	return ""
}

func sanitizeConfig(config []byte) json.RawMessage {
	var value any
	if json.Unmarshal(config, &value) != nil {
		return json.RawMessage(`{}`)
	}
	value = redactConfig(value)
	sanitized, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return sanitized
}

func redactConfig(value any) any {
	switch object := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(object))
		for key, child := range object {
			if hideConfigEntry(key, child) {
				continue
			}
			result[key] = redactConfig(child)
		}
		return result
	case []any:
		result := make([]any, len(object))
		for i, child := range object {
			result[i] = redactConfig(child)
		}
		return result
	default:
		return value
	}
}

func hideConfigEntry(key string, value any) bool {
	if logging.IsSensitiveKey(key) || strings.Contains(strings.ToLower(key), "webhook_url") {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	u, err := url.Parse(text)
	return err == nil && (u.User != nil || u.RawQuery != "")
}
