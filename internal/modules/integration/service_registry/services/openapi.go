package services

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
)

func parseOpenAPI(raw []byte) (*openapi3.T, []serviceregmodel.Endpoint, []byte, error) {
	if len(raw) == 0 || len(raw) > serviceregmodel.MaxSpecBytes {
		return nil, nil, nil, serviceregmodel.InvalidSpec("The OpenAPI document is empty or exceeds the size limit.")
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false
	doc, err := loader.LoadFromData(raw)
	if err != nil {
		return nil, nil, nil, serviceregmodel.TranslateOpenAPIError(err)
	}
	if doc.OpenAPI == "" || (!strings.HasPrefix(doc.OpenAPI, "3.0.") && !strings.HasPrefix(doc.OpenAPI, "3.1.")) {
		return nil, nil, nil, serviceregmodel.InvalidSpec("Only OpenAPI 3.0 and 3.1 documents are supported.")
	}
	if err := doc.Validate(context.Background()); err != nil {
		return nil, nil, nil, serviceregmodel.TranslateOpenAPIError(err)
	}
	endpoints, err := extractEndpoints(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	canonical, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, nil, serviceregmodel.InvalidSpec("The OpenAPI document could not be normalized.")
	}
	return doc, endpoints, canonical, nil
}

func extractEndpoints(doc *openapi3.T) ([]serviceregmodel.Endpoint, error) {
	if doc.Paths == nil {
		return []serviceregmodel.Endpoint{}, nil
	}
	paths := doc.Paths.Keys()
	sort.Strings(paths)
	out := make([]serviceregmodel.Endpoint, 0)
	seenOperationIDs := map[string]struct{}{}
	for _, path := range paths {
		if err := serviceregmodel.ValidateEndpointPath(path); err != nil {
			return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains an unsafe endpoint path.")
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
				return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains an invalid operationId.")
			}
			if _, duplicate := seenOperationIDs[operationID]; duplicate {
				return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains duplicate operationIds.")
			}
			seenOperationIDs[operationID] = struct{}{}
			endpoint := serviceregmodel.Endpoint{OperationID: operationID, Method: method, Path: path,
				Summary: operation.Summary, Deprecated: operation.Deprecated,
				Parameters: []map[string]any{}, ResponseSchemas: map[string]any{}, Security: []map[string]any{}}
			for _, ref := range append(append(openapi3.Parameters{}, item.Parameters...), operation.Parameters...) {
				if ref == nil || ref.Value == nil {
					return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains an unresolved parameter reference.")
				}
				param, err := serviceregmodel.ObjectMap(ref.Value)
				if err != nil {
					return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains an invalid parameter.")
				}
				endpoint.Parameters = append(endpoint.Parameters, param)
			}
			if operation.RequestBody != nil {
				if operation.RequestBody.Value == nil {
					return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains an unresolved request body reference.")
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
					return nil, serviceregmodel.InvalidSpec("The OpenAPI document contains invalid security requirements.")
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
		value, err := serviceregmodel.ObjectMap(media.Schema.Value)
		if err == nil {
			return value
		}
	}
	return nil
}
