package model

import (
	"encoding/json"
	"fmt"
	"time"

	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

const (
	Resource          = "api_composition"
	MaxAggregateBytes = 5 << 20
	DefaultDeadline   = 30 * time.Second
	DefaultStepLimit  = 20 * time.Second
)

var ErrCompositionNotFound = apperrors.NotFound("API composition not found.")

var ErrExecutionNotFound = apperrors.NotFound("API composition execution not found.")

type EndpointResponseError struct {
	Status int `json:"-"`
}

func (e EndpointResponseError) Error() string { return "endpoint returned an unsuccessful status" }

func RawJSONValue(raw json.RawMessage) (any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("composition: decode JSON value: %w", err)
	}
	return value, nil
}

func StringMap(value any) map[string]string {
	input, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, raw := range input {
		switch v := raw.(type) {
		case string:
			out[key] = v
		case nil:
			continue
		default:
			out[key] = fmt.Sprint(v)
		}
	}
	return out
}

func Validation(field, code, message string) error {
	return apperrors.Validation(apperrors.FieldError{Field: field, Code: code, Message: message})
}

func NullableJSON(raw []byte) any {
	if len(raw) == 0 {
		return nil
	}
	return json.RawMessage(raw)
}

func PtrString(value string) *string { return &value }
