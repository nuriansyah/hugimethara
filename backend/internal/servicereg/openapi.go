package servicereg

import (
	"context"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/backend/internal/api/apigen"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
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

func parseOpenAPI(raw []byte) (*openapi3.T, []Endpoint, []byte, error) {
	if len(raw) == 0 || len(raw) > maxSpecBytes {
		return nil, nil, nil, invalidSpec("The OpenAPI document is empty or exceeds the size limit.")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(raw)
	if err != nil {
		return nil, nil, nil, translateOpenAPIError(err)
	}
	if doc.OpenAPI == "" || (!strings.HasPrefix(doc.OpenAPI, "3.0.") && !strings.HasPrefix(doc.OpenAPI, "3.1.")) {
		return nil, nil, nil, invalidSpec("Only OpenAPI 3.0 and 3.1 documents are supported.")
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, nil, nil, translateOpenAPIError(err)
	}
	endpoints, err := extractEndpoints(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	canonical, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, nil, invalidSpec("The OpenAPI document could not be normalized.")
	}
	return doc, endpoints, canonical, nil
}

func extractEndpoints(doc *openapi3.T) ([]Endpoint, error) {
	if doc.Paths == nil {
		return []Endpoint{}, nil
	}
	paths := doc.Paths.Keys()
	sort.Strings(paths)
	out := make([]Endpoint, 0)
	seenOperationIDs := map[string]struct{}{}
	for _, path := range paths {
		if err := validateEndpointPath(path); err != nil {
			return nil, invalidSpec("The OpenAPI document contains an unsafe endpoint path.")
		}
		item := doc.Paths.Value(path)
		if item == nil {
			continue
		}
		methods := item.Operations()
		methodNames := make([]string, 0, len(methods))
		for method := range methods {
			methodNames = append(methodNames, strings.ToUpper(method))
		}
		sort.Strings(methodNames)
		for _, method := range methodNames {
			switch method {
			case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			default:
				continue
			}
			operation := item.GetOperation(method)
			if operation == nil {
				continue
			}
			operationID := strings.TrimSpace(operation.OperationID)
			if operationID == "" {
				operationID = strings.ToLower(method) + "_" + strings.Trim(strings.ReplaceAll(path, "/", "_"), "_")
			}
			if len(operationID) < 1 || len(operationID) > 200 {
				return nil, invalidSpec("The OpenAPI document contains an invalid operationId.")
			}
			if _, duplicate := seenOperationIDs[operationID]; duplicate {
				return nil, invalidSpec("The OpenAPI document contains duplicate operationIds.")
			}
			seenOperationIDs[operationID] = struct{}{}
			endpoint := Endpoint{OperationID: operationID, Method: method, Path: path,
				Summary: operation.Summary, Deprecated: operation.Deprecated,
				Parameters: []map[string]any{}, ResponseSchemas: map[string]any{}, Security: []map[string]any{}}
			for _, ref := range append(append(openapi3.Parameters{}, item.Parameters...), operation.Parameters...) {
				if ref == nil || ref.Value == nil {
					return nil, invalidSpec("The OpenAPI document contains an unresolved parameter reference.")
				}
				param, err := objectMap(ref.Value)
				if err != nil {
					return nil, invalidSpec("The OpenAPI document contains an invalid parameter.")
				}
				endpoint.Parameters = append(endpoint.Parameters, param)
			}
			if operation.RequestBody != nil {
				if operation.RequestBody.Value == nil {
					return nil, invalidSpec("The OpenAPI document contains an unresolved request body reference.")
				}
				endpoint.RequestSchema = firstContentSchema(operation.RequestBody.Value.Content)
			}
			if operation.Responses != nil {
				statuses := operation.Responses.Keys()
				sort.Strings(statuses)
				for _, status := range statuses {
					ref := operation.Responses.Value(status)
					if ref == nil || ref.Value == nil {
						continue
					}
					schema := firstContentSchema(ref.Value.Content)
					if schema != nil {
						endpoint.ResponseSchemas[status] = schema
					}
				}
			}
			security := operation.Security
			if security == nil {
				security = &doc.Security
			}
			if len(*security) > 0 {
				var requirements []map[string]any
				data, err := json.Marshal(*security)
				if err != nil || json.Unmarshal(data, &requirements) != nil {
					return nil, invalidSpec("The OpenAPI document contains invalid security requirements.")
				}
				endpoint.Security = requirements
			}
			out = append(out, endpoint)
		}
	}
	return out, nil
}

func firstContentSchema(content openapi3.Content) map[string]any {
	if len(content) == 0 {
		return nil
	}
	keys := make([]string, 0, len(content))
	for key := range content {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i] == "application/json" {
			return true
		}
		if keys[j] == "application/json" {
			return false
		}
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		media := content[key]
		if media == nil || media.Schema == nil || media.Schema.Value == nil {
			continue
		}
		value, err := objectMap(media.Schema.Value)
		if err == nil {
			return value
		}
	}
	return nil
}

func objectMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	err = json.Unmarshal(data, &out)
	return out, err
}

func validateEndpointPath(path string) error {
	if len(path) < 1 || len(path) > 1000 || path[0] != '/' || strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "\\%?#") {
		return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "." || segment == ".." {
			return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '}' {
			return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
		if path[i] != '{' {
			continue
		}
		end := strings.IndexByte(path[i+1:], '}')
		if end < 1 {
			return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
		}
		end += i + 1
		name := path[i+1 : end]
		for _, r := range name {
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' && r != '-' {
				return httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe", Message: "Endpoint path is unsafe."})
			}
		}
		i = end
	}
	return nil
}

func Diff(current []Endpoint, previous []diffEndpointRow) ImportResult {
	old := make(map[string]diffEndpointRow, len(previous))
	for _, endpoint := range previous {
		old[endpointKey(endpoint.Method, endpoint.Path)] = endpoint
	}
	newEndpoints := make(map[string]Endpoint, len(current))
	for _, endpoint := range current {
		newEndpoints[endpointKey(endpoint.Method, endpoint.Path)] = endpoint
	}
	changes := make([]OpenAPIChange, 0)
	for key, endpoint := range newEndpoints {
		before, exists := old[key]
		if !exists {
			changes = append(changes, OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "added"})
			continue
		}
		if !sameEndpoint(endpoint, before) {
			changes = append(changes, OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "changed"})
		}
	}
	for key, endpoint := range old {
		if _, exists := newEndpoints[key]; !exists {
			changes = append(changes, OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "removed", Breaking: true})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Method < changes[j].Method
	})
	return ImportResult{Changes: changes, EndpointCount: len(current)}
}

func sameEndpoint(current Endpoint, previous diffEndpointRow) bool {
	eqJSON := func(value any, old json.RawMessage) bool {
		a, errA := json.Marshal(value)
		var decoded any
		errB := json.Unmarshal(old, &decoded)
		b, errC := json.Marshal(decoded)
		return errA == nil && errB == nil && errC == nil && string(a) == string(b)
	}
	return current.OperationID == previous.OperationID &&
		current.Summary == previous.Summary && current.Deprecated == previous.Deprecated &&
		eqJSON(current.Parameters, previous.Parameters) &&
		eqJSON(current.RequestSchema, previous.RequestSchema) &&
		eqJSON(current.ResponseSchemas, previous.ResponseSchemas) &&
		eqJSON(current.Security, previous.Security)
}

func endpointKey(method, path string) string { return strings.ToUpper(method) + "\x00" + path }

func buildEndpointURL(baseURL, path string, pathValues, query map[string]string) (string, error) {
	if err := validateEndpointPath(path); err != nil {
		return "", httpx.Validation(httpx.FieldError{Field: "path", Code: "unsafe",
			Message: "The endpoint path is unsafe."})
	}
	normalizedBaseURL, err := validateBaseURL(baseURL)
	if err != nil {
		return "", httpx.Validation(httpx.FieldError{Field: "base_url", Code: "invalid",
			Message: "The service environment has an invalid base URL."})
	}
	base, err := url.Parse(normalizedBaseURL)
	if err != nil {
		return "", httpx.Validation(httpx.FieldError{Field: "base_url", Code: "invalid",
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
			return "", httpx.Validation(httpx.FieldError{Field: "path_values", Code: "invalid",
				Message: "Every path parameter must have a safe non-empty value."})
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return "", httpx.Validation(httpx.FieldError{Field: "path_values", Code: "invalid",
					Message: "Every path parameter must have a safe non-empty value."})
			}
		}
		used[name] = struct{}{}
		escaped.WriteString(url.PathEscape(value))
		decoded.WriteString(value)
		i = end + 1
	}
	if len(used) != len(pathValues) {
		return "", httpx.Validation(httpx.FieldError{Field: "path_values", Code: "unexpected",
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
			return "", httpx.Validation(httpx.FieldError{Field: "query", Code: "invalid",
				Message: "One or more query parameters are invalid."})
		}
		queryBytes += len(key) + len(value)
		if queryBytes > 16<<10 {
			return "", httpx.Validation(httpx.FieldError{Field: "query", Code: "too_large",
				Message: "The query parameters exceed the size limit."})
		}
		values.Set(key, value)
	}
	base.RawQuery = values.Encode()
	base.Fragment = ""
	return base.String(), nil
}

func validRequestHeader(name, value string) bool {
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

func filteredResponseHeaders(headers map[string][]string) map[string]string {
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

func apiChanges(changes []OpenAPIChange) []apigen.OpenAPIChange {
	out := make([]apigen.OpenAPIChange, len(changes))
	for i, change := range changes {
		kind := apigen.OpenAPIChangeKind(change.Kind)
		breaking := change.Breaking
		out[i] = apigen.OpenAPIChange{OperationId: change.OperationID, Method: change.Method,
			Path: change.Path, Kind: kind, Breaking: &breaking}
	}
	return out
}
