package services

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	connectormodel "github.com/nuriansyah/hugimethara/internal/modules/integration/connector/model"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
)

func validateConfig(schemaJSON, configJSON []byte) error {
	var schema, value any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return fmt.Errorf("connector: stored config schema is invalid: %w", err)
	}
	if err := json.Unmarshal(configJSON, &value); err != nil {
		return problem.Validation(problem.FieldError{Field: "config", Code: "invalid_json", Message: "Configuration must be valid JSON."})
	}
	if err := connectormodel.ValidateSchemaValue(schema, value, "config"); err != nil {
		return problem.Validation(problem.FieldError{Field: "config", Code: "schema_invalid", Message: err.Error()})
	}
	if key := sensitiveConfigKey(value, "config"); key != "" {
		return problem.Validation(problem.FieldError{Field: key, Code: "use_secret_reference", Message: "Credentials must be stored as a credential reference, not in connector configuration."})
	}
	return nil
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
