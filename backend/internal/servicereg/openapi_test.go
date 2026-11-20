package servicereg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseOpenAPISupports30And31WithLocalReferences(t *testing.T) {
	for _, version := range []string{"3.0.3", "3.1.0"} {
		t.Run(version, func(t *testing.T) {
			spec := `openapi: ` + version + `
info:
  title: Demo API
  version: v1
paths:
  /widgets/{widget_id}:
    get:
      operationId: getWidget
      summary: Get a widget
      parameters:
        - name: widget_id
          in: path
          required: true
          schema:
            type: string
      responses:
        "200":
          description: A widget
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Widget"
components:
  schemas:
    Widget:
      type: object
      properties:
        id:
          type: string
`
			doc, endpoints, canonical, err := parseOpenAPI([]byte(spec))
			require.NoError(t, err)
			require.Equal(t, version, doc.OpenAPI)
			require.Len(t, endpoints, 1)
			require.Equal(t, "GET", endpoints[0].Method)
			require.Equal(t, "/widgets/{widget_id}", endpoints[0].Path)
			require.NotEmpty(t, endpoints[0].ResponseSchemas["200"])
			require.True(t, json.Valid(canonical))
		})
	}
}

func TestParseOpenAPIRejectsExternalReferences(t *testing.T) {
	spec := `openapi: 3.0.3
info:
  title: Demo API
  version: v1
paths:
  /widgets:
    $ref: https://example.invalid/openapi.yaml#/paths/~1widgets
`
	_, _, _, err := parseOpenAPI([]byte(spec))
	require.Error(t, err)
	require.NotContains(t, strings.ToLower(err.Error()), "example.invalid")
}

func TestDiffReportsAddedChangedAndRemovedOperations(t *testing.T) {
	before := []diffEndpointRow{
		{OperationID: "getOld", Method: "GET", Path: "/old", Summary: "Old",
			Parameters: json.RawMessage(`[]`), RequestSchema: json.RawMessage(`null`),
			ResponseSchemas: json.RawMessage(`{}`), Security: json.RawMessage(`[]`)},
		{OperationID: "getWidget", Method: "GET", Path: "/widgets", Summary: "Before",
			Parameters: json.RawMessage(`[]`), RequestSchema: json.RawMessage(`null`),
			ResponseSchemas: json.RawMessage(`{}`), Security: json.RawMessage(`[]`)},
	}
	current := []Endpoint{
		{OperationID: "getWidget", Method: "GET", Path: "/widgets", Summary: "After",
			Parameters: []map[string]any{}, ResponseSchemas: map[string]any{}, Security: []map[string]any{}},
		{OperationID: "createWidget", Method: "POST", Path: "/widgets",
			Parameters: []map[string]any{}, ResponseSchemas: map[string]any{}, Security: []map[string]any{}},
	}
	result := Diff(current, before)
	require.Len(t, result.Changes, 3)
	kinds := map[string]string{}
	breaking := map[string]bool{}
	for _, change := range result.Changes {
		kinds[change.Method+" "+change.Path] = change.Kind
		breaking[change.Method+" "+change.Path] = change.Breaking
	}
	require.Equal(t, "changed", kinds["GET /widgets"])
	require.Equal(t, "removed", kinds["GET /old"])
	require.True(t, breaking["GET /old"])
	require.Equal(t, "added", kinds["POST /widgets"])
	require.Equal(t, 2, result.EndpointCount)
}

func TestEndpointPathValidationAndExpansionPreventTraversal(t *testing.T) {
	require.NoError(t, validateEndpointPath("/v1/widgets/{widget_id}"))
	require.NoError(t, validateEndpointPath("/v1/widgets/{widget_id}.json"))
	for _, path := range []string{"//attacker.example/x", "/../admin", `/v1\admin`, "/v1/%2e%2e/admin", "/bad?query=1"} {
		require.Error(t, validateEndpointPath(path), path)
	}
	_, err := buildEndpointURL("https://api.example/base", "/widgets/{id}",
		map[string]string{"id": "../admin"}, nil)
	require.Error(t, err)
	_, err = buildEndpointURL("https://api.example/base", "/widgets/{id}",
		map[string]string{"id": "a/b"}, nil)
	require.Error(t, err)

	target, err := buildEndpointURL("https://api.example/base", "/widgets/{id}",
		map[string]string{"id": "item-7"}, map[string]string{"q": "a b"})
	require.NoError(t, err)
	require.Equal(t, "https://api.example/base/widgets/item-7?q=a+b", target)

	target, err = buildEndpointURL("https://api.example/base", "/widgets/{id}.json",
		map[string]string{"id": "item-7"}, nil)
	require.NoError(t, err)
	require.Equal(t, "https://api.example/base/widgets/item-7.json", target)
}

func TestParseOpenAPIRejectsUnsafeEndpointPaths(t *testing.T) {
	spec := `openapi: 3.0.3
info:
  title: Demo API
  version: v1
paths:
  /../admin:
    get:
      operationId: unsafe
      responses:
        "200":
          description: ok
`
	_, _, _, err := parseOpenAPI([]byte(spec))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unsafe endpoint path")
}

func TestExplorerFiltersSensitiveResponseHeaders(t *testing.T) {
	filtered := filteredResponseHeaders(map[string][]string{
		"Content-Type":       {"application/json"},
		"Set-Cookie":         {"session=secret"},
		"Authorization":      {"Bearer secret"},
		"X-Request-Id":       {"request-1"},
		"X-Api-Key":          {"secret"},
		"Proxy-Authenticate": {"Basic"},
	})
	require.Equal(t, "application/json", filtered["Content-Type"])
	require.Equal(t, "request-1", filtered["X-Request-Id"])
	require.NotContains(t, filtered, "Set-Cookie")
	require.NotContains(t, filtered, "Authorization")
	require.NotContains(t, filtered, "X-Api-Key")
	require.NotContains(t, filtered, "Proxy-Authenticate")
}
