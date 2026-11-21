package model

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

const (
	Resource           = "service"
	MaxSpecBytes       = 2 << 20
	MaxExplorerBytes   = 1 << 20
	DefaultHTTPTimeout = 20 * time.Second
)

var ErrServiceNotFound = problem.NotFound("Service not found.")

type RegisteredService struct {
	ID            uuid.UUID
	WorkspaceID   uuid.UUID
	Name          string
	Owner         string
	Description   string
	Protocol      string
	EndpointCount int64
	Environments  []Environment
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Environment struct {
	EnvironmentID       uuid.UUID
	BaseURL             string
	ConnectorInstanceID *uuid.UUID
}

type Endpoint struct {
	ID              uuid.UUID
	ServiceID       uuid.UUID
	OperationID     string
	Method          string
	Path            string
	Summary         string
	Parameters      []map[string]any
	RequestSchema   map[string]any
	ResponseSchemas map[string]any
	Security        []map[string]any
	Deprecated      bool
}

type ServiceTryResult struct {
	Status     int
	Headers    map[string]string
	Body       any
	DurationMs int
}

func UnmarshalJSON(data []byte, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func NullableUUID(v *uuid.UUID) any {
	if v == nil || *v == uuid.Nil {
		return nil
	}
	return *v
}

func ValidateBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" ||
		strings.ContainsAny(u.Host+u.Path, "\r\n\\") {
		return "", problem.Validation(problem.FieldError{Field: "base_url", Code: "invalid",
			Message: "Base URL must be an HTTP(S) URL without credentials, query, or fragment."})
	}
	for _, segment := range strings.Split(u.Path, "/") {
		if segment == "." || segment == ".." {
			return "", problem.Validation(problem.FieldError{Field: "base_url", Code: "invalid",
				Message: "Base URL path must not contain traversal segments."})
		}
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func ValidateSourceURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.Fragment != "" || strings.ContainsAny(u.Host, "\r\n") {
		return nil, problem.Validation(problem.FieldError{Field: "url", Code: "invalid",
			Message: "The OpenAPI source must be an HTTP(S) URL without credentials or a fragment."})
	}
	return u, nil
}

func FormatFromMediaType(contentType string, data []byte) string {
	if strings.Contains(strings.ToLower(contentType), "yaml") {
		return "yaml"
	}
	if !json.Valid(data) {
		return "yaml"
	}
	return "json"
}

func InvalidSpec(message string) error {
	return problem.NewProblem(http.StatusUnprocessableEntity, "invalid_openapi", "Invalid OpenAPI document", message)
}

func TranslateOpenAPIError(err error) error {
	if err == nil {
		return nil
	}
	return InvalidSpec("The document is not a valid OpenAPI 3.0 or 3.1 document.")
}
