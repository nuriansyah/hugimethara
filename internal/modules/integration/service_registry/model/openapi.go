package model

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"

	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

type OpenAPIChange struct {
	OperationID string
	Method      string
	Path        string
	Kind        string
	Breaking    bool
}

type ImportResult struct {
	DryRun        bool
	SpecID        *uuid.UUID
	Changes       []OpenAPIChange
	EndpointCount int
}

func ObjectMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	err = json.Unmarshal(data, &out)
	return out, err
}

func ValidateEndpointPath(path string) error {
	if len(path) < 1 || len(path) > 1000 || path[0] != '/' || strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "\\%?#") {
		return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '}' {
			return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
		if path[i] != '{' {
			continue
		}
		end := strings.IndexByte(path[i+1:], '}')
		if end < 1 {
			return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
		end += i + 1
		name := path[i+1 : end]
		for _, r := range name {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' && r != '-' {
				return problem.Validation(problem.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
			}
		}
		i = end
	}
	return nil
}

func EndpointKey(method, path string) string { return strings.ToUpper(method) + "\x00" + path }

func BuildEndpointURL(baseURL, path string, pathValues, query map[string]string) (string, error) {
	if err := ValidateEndpointPath(path); err != nil {
		return "", problem.Validation(problem.FieldError{Field: "path", Code: "unsafe",
			Message: "The endpoint path is unsafe."})
	}
	normalizedBaseURL, err := ValidateBaseURL(baseURL)
	if err != nil {
		return "", problem.Validation(problem.FieldError{Field: "base_url", Code: "invalid",
			Message: "The service environment has an invalid base URL."})
	}
	base, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return "", problem.Validation(problem.FieldError{Field: "base_url", Code: "invalid",
			Message: "The service environment has an invalid base URL."})
	}
	used := map[string]struct{}{}
	var escaped, decoded strings.Builder
	for i := 0; i < len(path); {
		if path[i] != '{' {
			escaped.WriteByte(path[i])
			decoded.WriteByte(path[i])
			i++
			continue
		}
		end := strings.IndexByte(path[i+1:], '}') + i + 1
		name := path[i+1 : end]
		value, ok := pathValues[name]
		if !ok || value == "" || len(value) > 1000 || value == "." || value == ".." || strings.ContainsAny(value, "/\\%") {
			return "", problem.Validation(problem.FieldError{Field: "path_values", Code: "invalid",
				Message: "Every path parameter must have a safe non-empty value."})
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return "", problem.Validation(problem.FieldError{Field: "path_values", Code: "invalid",
					Message: "Every path parameter must have a safe non-empty value."})
			}
		}
		used[name] = struct{}{}
		escaped.WriteString(url.PathEscape(value))
		decoded.WriteString(value)
		i = end + 1
	}
	if len(used) != len(pathValues) {
		return "", problem.Validation(problem.FieldError{Field: "path_values", Code: "unexpected",
			Message: "Only path parameters declared by the endpoint are allowed."})
	}
	escapedEndpoint := escaped.String()
	decodedEndpoint := decoded.String()
	baseEscapedPath := strings.TrimRight(base.EscapedPath(), "/")
	base.Path = strings.TrimRight(base.Path, "/") + decodedEndpoint
	base.RawPath = baseEscapedPath + escapedEndpoint
	values := base.Query()
	queryBytes := 0
	for key, value := range query {
		if key == "" || strings.ContainsAny(key+value, "\r\n") || len(key) > 200 || len(value) > 2000 {
			return "", problem.Validation(problem.FieldError{Field: "query", Code: "invalid",
				Message: "One or more query parameters are invalid."})
		}
		queryBytes += len(key) + len(value)
		if queryBytes > 16<<10 {
			return "", problem.Validation(problem.FieldError{Field: "query", Code: "too_large",
				Message: "The query parameters exceed the size limit."})
		}
		values.Set(key, value)
	}
	base.RawQuery = values.Encode()
	base.Fragment = ""
	return base.String(), nil
}

func ValidRequestHeader(name, value string) bool {
	if name == "" || strings.ContainsAny(name+value, "\r\n") || len(name) > 200 || len(value) > 8192 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\t' {
			return false
		}
	}
	lower := strings.ToLower(name)
	switch lower {
	case "host", "content-length", "connection", "transfer-encoding", "upgrade", "proxy-authorization", "proxy-connection":
		return false
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') &&
			(r < '0' || r > '9') && !strings.ContainsRune("!#$%&'*+-.^_`|~", r) {
			return false
		}
	}
	return true
}

func FilteredResponseHeaders(headers map[string][]string) map[string]string {
	out := map[string]string{}
	total := 0
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		values := headers[name]
		lower := strings.ToLower(name)
		if sensitiveResponseHeader(lower) || len(values) == 0 || len(out) >= 32 {
			continue
		}
		value := strings.Join(values, ", ")
		if len(value) > 2048 {
			value = value[:2048]
		}
		if total+len(name)+len(value) > 16<<10 {
			continue
		}
		out[name] = value
		total += len(name) + len(value)
	}
	return out
}

func sensitiveResponseHeader(name string) bool {
	normalized := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, name)
	for _, marker := range []string{"auth", "cookie", "token", "secret", "apikey", "password", "credential", "session", "key"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	switch name {
	case "set-cookie", "proxy-authenticate", "proxy-authorization", "www-authenticate",
		"connection", "keep-alive", "transfer-encoding", "upgrade", "te", "trailer":
		return true
	}
	return false
}
