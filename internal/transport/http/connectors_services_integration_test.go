//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectorLifecycleAndEnvironmentScopedConnectionTest(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("connector-co", "owner@connector.test"), false).Status)
	workspace := owner.do("POST", base+"/workspaces", map[string]any{
		"slug": "integrations", "name": "Integrations",
	}, true)
	require.Equal(t, 201, workspace.Status, workspace.Body)
	workspaceID := workspace.Body["id"].(string)
	environmentID := developmentEnvironment(t, e, owner)
	allowLoopbackForTest(t, e, environmentID)

	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		requestCount.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	definitions := owner.do("GET", base+"/connector-definitions", nil, false)
	require.Equal(t, 200, definitions.Status, definitions.Body)
	var definitionID string
	for _, row := range data(definitions) {
		if row["key"] == "http_rest" {
			definitionID = row["id"].(string)
		}
	}
	require.NotEmpty(t, definitionID)
	versions := owner.do("GET", base+"/connector-definitions/"+definitionID+"/versions", nil, false)
	require.Equal(t, 200, versions.Status, versions.Body)
	require.NotEmpty(t, data(versions))
	versionID := data(versions)[0]["id"].(string)
	secretValue := "do-not-return-connector-secret"
	credential := owner.do("POST", base+"/credentials", map[string]any{
		"name": "http bearer", "kind": "bearer", "value": secretValue,
	}, true)
	require.Equal(t, 201, credential.Status, credential.Body)

	created := owner.do("POST", base+"/connectors", map[string]any{
		"workspace_id": workspaceID, "environment_id": environmentID,
		"ref_key": "test-http", "connector_version_id": versionID,
		"config": map[string]any{"base_url": server.URL}, "auth_method": "bearer",
		"secret_id": credential.Body["id"],
	}, true)
	require.Equal(t, 201, created.Status, created.Body)
	connectorID := created.Body["id"].(string)
	require.Equal(t, "test-http", created.Body["name"])
	require.Equal(t, true, created.Body["credential_configured"])
	require.NotContains(t, created.Body, secretValue)

	got := owner.do("GET", base+"/connectors/"+connectorID, nil, false)
	require.Equal(t, 200, got.Status, got.Body)
	require.Equal(t, server.URL, got.Body["config"].(map[string]any)["base_url"])
	require.NotContains(t, got.Body, secretValue)

	tested := owner.do("POST", base+"/connectors/"+connectorID+"/test", nil, true)
	require.Equal(t, 200, tested.Status, tested.Body)
	require.Equal(t, true, tested.Body["ok"])
	require.NotContains(t, tested.Body, server.URL)
	requestsBeforeBlockedCall := requestCount.Load()
	_, err := e.pg.Owner.ExecContext(context.Background(),
		"UPDATE environments SET egress_allowlist = ARRAY[]::text[] WHERE id = $1", environmentID)
	require.NoError(t, err)
	blocked := owner.do("POST", base+"/connectors/"+connectorID+"/test", nil, true)
	require.Equal(t, 200, blocked.Status, blocked.Body)
	require.Equal(t, false, blocked.Body["ok"])
	require.Equal(t, "destination_blocked", blocked.Body["error_code"])
	require.Equal(t, requestsBeforeBlockedCall, requestCount.Load())
	health := owner.do("GET", base+"/connectors/"+connectorID+"/health", nil, false)
	require.Equal(t, 200, health.Status, health.Body)
	require.Len(t, data(health), 2)

	invalid := owner.do("PATCH", base+"/connectors/"+connectorID, map[string]any{
		"config": map[string]any{"base_url": server.URL, "api_token": "must-not-be-inline"},
	}, true)
	require.Equal(t, 422, invalid.Status, invalid.Body)
	require.NotContains(t, invalid.Body, "must-not-be-inline")

	disabled := owner.do("PATCH", base+"/connectors/"+connectorID, map[string]any{"status": "disabled"}, true)
	require.Equal(t, 200, disabled.Status, disabled.Body)
	require.Equal(t, "disabled", disabled.Body["status"])
	require.Equal(t, 204, owner.do("DELETE", base+"/connectors/"+connectorID, nil, true).Status)
	require.Equal(t, 404, owner.do("GET", base+"/connectors/"+connectorID, nil, false).Status)

	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations",
		signupBody("other-connector", "owner@other-connector.test"), false).Status)
	require.Equal(t, 404, other.do("GET", base+"/connectors/"+connectorID, nil, false).Status)
}

func TestServiceOpenAPIImportDiffAndGuardedExplorerCall(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("service-co", "owner@service.test"), false).Status)
	workspace := owner.do("POST", base+"/workspaces", map[string]any{
		"slug": "apis", "name": "APIs",
	}, true)
	require.Equal(t, 201, workspace.Status, workspace.Body)
	workspaceID := workspace.Body["id"].(string)
	environmentID := developmentEnvironment(t, e, owner)
	allowLoopbackForTest(t, e, environmentID)

	var requestCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/order-42" || r.URL.Query().Get("view") != "compact" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		requestCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "session=should-not-be-returned")
		_, _ = w.Write([]byte(`{"id":"order-42","state":"ready"}`))
	}))
	defer server.Close()

	created := owner.do("POST", base+"/services", map[string]any{
		"workspace_id": workspaceID, "name": "Orders API", "protocol": "rest",
	}, true)
	require.Equal(t, 201, created.Status, created.Body)
	serviceID := created.Body["id"].(string)

	binding := owner.do("PUT", base+"/services/"+serviceID+"/environments/"+environmentID, map[string]any{
		"base_url": server.URL,
	}, true)
	require.Equal(t, 200, binding.Status, binding.Body)

	spec := `{
	  "openapi":"3.0.3",
	  "info":{"title":"Orders API","version":"1.0.0"},
	  "paths":{
	    "/orders/{order_id}":{
	      "get":{
	        "operationId":"getOrder",
	        "summary":"Read an order",
	        "parameters":[
	          {"name":"order_id","in":"path","required":true,"schema":{"type":"string"}},
	          {"name":"view","in":"query","schema":{"type":"string"}}
	        ],
	        "responses":{"200":{"description":"Order","content":{"application/json":{"schema":{"type":"object"}}}}}
	      }
	    }
	  }
	}`
	preview := owner.do("POST", base+"/services/"+serviceID+"/import-openapi?dry_run=true",
		map[string]any{"spec": spec}, true)
	require.Equal(t, 200, preview.Status, preview.Body)
	require.Equal(t, true, preview.Body["dry_run"])
	require.EqualValues(t, 1, preview.Body["endpoint_count"])
	changes := preview.Body["changes"].([]any)
	require.Len(t, changes, 1)
	require.Equal(t, "added", changes[0].(map[string]any)["kind"])

	imported := owner.do("POST", base+"/services/"+serviceID+"/import-openapi?dry_run=false",
		map[string]any{"spec": spec, "version_label": "1.0.0"}, true)
	require.Equal(t, 200, imported.Status, imported.Body)
	require.Equal(t, false, imported.Body["dry_run"])

	endpoints := owner.do("GET", base+"/services/"+serviceID+"/endpoints", nil, false)
	require.Equal(t, 200, endpoints.Status, endpoints.Body)
	require.Len(t, data(endpoints), 1)
	endpointID := data(endpoints)[0]["id"].(string)

	result := owner.do("POST", base+"/service-endpoints/"+endpointID+"/try", map[string]any{
		"environment_id": environmentID,
		"path_values":    map[string]string{"order_id": "order-42"},
		"query":          map[string]string{"view": "compact"},
	}, true)
	require.Equal(t, 200, result.Status, result.Body)
	require.EqualValues(t, 200, result.Body["status"])
	require.Equal(t, "ready", result.Body["body"].(map[string]any)["state"])
	require.NotContains(t, result.Body["headers"], "set-cookie")

	requestsBeforeBlockedCall := requestCount.Load()
	_, err := e.pg.Owner.ExecContext(context.Background(),
		"UPDATE environments SET egress_allowlist = ARRAY[]::text[] WHERE id = $1", environmentID)
	require.NoError(t, err)
	blocked := owner.do("POST", base+"/service-endpoints/"+endpointID+"/try", map[string]any{
		"environment_id": environmentID,
		"path_values":    map[string]string{"order_id": "order-42"},
		"query":          map[string]string{"view": "compact"},
	}, true)
	require.Equal(t, 422, blocked.Status, blocked.Body)
	require.Equal(t, requestsBeforeBlockedCall, requestCount.Load())

	unsafe := owner.do("POST", base+"/service-endpoints/"+endpointID+"/try", map[string]any{
		"environment_id": environmentID,
		"path_values":    map[string]string{"order_id": "../private"},
	}, true)
	require.Equal(t, 422, unsafe.Status, unsafe.Body)

	list := owner.do("GET", base+"/services?workspace_id="+workspaceID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)
	require.Equal(t, int64(1), int64(data(list)[0]["endpoint_count"].(float64)))
}

func developmentEnvironment(t *testing.T, e *env, owner *client) string {
	t.Helper()
	environments := owner.do("GET", base+"/environments", nil, false)
	require.Equal(t, 200, environments.Status, environments.Body)
	for _, environment := range data(environments) {
		if environment["slug"] == "development" {
			return environment["id"].(string)
		}
	}
	t.Fatal("development environment was not created")
	return ""
}

func allowLoopbackForTest(t *testing.T, e *env, environmentID string) {
	t.Helper()
	_, err := e.pg.Owner.ExecContext(context.Background(),
		"UPDATE environments SET egress_allowlist = ARRAY['127.0.0.1/32']::text[] WHERE id = $1", environmentID)
	require.NoError(t, err)
}

func TestMappingLifecyclePublishGateAndTenantIsolation(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("mapping-co", "owner@mapping.test"), false).Status)
	workspace := owner.do("POST", base+"/workspaces", map[string]any{
		"slug": "api-studio", "name": "API Studio",
	}, true)
	require.Equal(t, 201, workspace.Status, workspace.Body)
	workspaceID := workspace.Body["id"].(string)

	created := owner.do("POST", base+"/mappings", map[string]any{
		"workspace_id": workspaceID,
		"name":         "Order overview",
		"draft": map[string]any{"rules": []any{
			map[string]any{"target": "name", "source": "$.customer_name", "transforms": []any{
				map[string]any{"op": "trim"},
			}},
			map[string]any{"target": "amount", "source": "$.total_amount", "transforms": []any{
				map[string]any{"op": "to_number"},
			}},
			map[string]any{"target": "tier", "expr": "src.total_amount > 100 ? 'gold' : 'std'", "when": "has(src.total_amount)"},
		}},
	}, true)
	require.Equal(t, 201, created.Status, created.Body)
	mappingID := created.Body["id"].(string)

	tested := owner.do("POST", base+"/mappings/"+mappingID+"/test", map[string]any{
		"input": map[string]any{"customer_name": "  Ada Lovelace  ", "total_amount": 125},
	}, true)
	require.Equal(t, 200, tested.Status, tested.Body)
	require.Equal(t, "Ada Lovelace", tested.Body["output"].(map[string]any)["name"])
	require.Equal(t, "gold", tested.Body["output"].(map[string]any)["tier"])

	// Publishing requires at least one passing stored test.
	publishWithoutTests := owner.do("POST", base+"/mappings/"+mappingID+"/publish", nil, true)
	require.Equal(t, 422, publishWithoutTests.Status, publishWithoutTests.Body)

	stored := owner.do("POST", base+"/mappings/"+mappingID+"/tests", map[string]any{
		"name":     "Gold customer",
		"input":    map[string]any{"customer_name": "  Ada Lovelace  ", "total_amount": 125},
		"expected": map[string]any{"name": "Ada Lovelace", "amount": float64(125), "tier": "gold"},
	}, true)
	require.Equal(t, 201, stored.Status, stored.Body)
	testID := stored.Body["id"].(string)

	storedRun := owner.do("POST", base+"/mappings/"+mappingID+"/test", map[string]any{
		"input": map[string]any{"customer_name": "Ada Lovelace", "total_amount": 125},
	}, true)
	require.Equal(t, 200, storedRun.Status, storedRun.Body)
	require.Equal(t, float64(1), storedRun.Body["stored_tests_passed"])
	require.Empty(t, storedRun.Body["stored_tests_failed"])

	published := owner.do("POST", base+"/mappings/"+mappingID+"/publish", nil, true)
	require.Equal(t, 201, published.Status, published.Body)
	require.Equal(t, float64(1), published.Body["version"])

	versions := owner.do("GET", base+"/mappings/"+mappingID+"/versions", nil, false)
	require.Equal(t, 200, versions.Status, versions.Body)
	require.Len(t, data(versions), 1)

	updatedTest := owner.do("PATCH", base+"/mappings/"+mappingID+"/tests/"+testID, map[string]any{
		"name": "Gold customer updated",
	}, true)
	require.Equal(t, 200, updatedTest.Status, updatedTest.Body)

	list := owner.do("GET", base+"/mappings?workspace_id="+workspaceID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)

	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations",
		signupBody("other-mapping", "owner@other-mapping.test"), false).Status)
	require.Equal(t, 404, other.do("GET", base+"/mappings/"+mappingID, nil, false).Status)
	require.Equal(t, 404, other.do("GET", base+"/mappings/"+mappingID+"/versions", nil, false).Status)
}
