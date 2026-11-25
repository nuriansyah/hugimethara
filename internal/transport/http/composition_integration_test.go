//go:build integration

package api_test

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// S14 acceptance: the PRD §6.6 order-overview scenario — two imported APIs
// composed in one DAG with a response mapping — runs end to end through the
// service registry, the egress-guarded explorer client and the composition
// executor.
func TestOrderOverviewCompositionEndToEnd(t *testing.T) {
	e := setup(t)
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations",
		signupBody("composition-co", "owner@composition.test"), false).Status)
	workspace := owner.do("POST", base+"/workspaces", map[string]any{
		"slug": "order-ops", "name": "Order ops",
	}, true)
	require.Equal(t, 201, workspace.Status, workspace.Body)
	workspaceID := workspace.Body["id"].(string)
	environmentID := developmentEnvironment(t, e, owner)
	allowLoopbackForTest(t, e, environmentID)

	var orderCalls, customerCalls atomic.Int64
	orders := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/orders/o-42" {
			http.Error(w, "unexpected order request", http.StatusBadRequest)
			return
		}
		orderCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"o-42","status":"paid","customer_id":"c-7"}`))
	}))
	defer orders.Close()
	customers := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/customers/c-7" {
			http.Error(w, "unexpected customer request", http.StatusBadRequest)
			return
		}
		customerCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c-7","name":"Ada"}`))
	}))
	defer customers.Close()

	importService := func(name, serverURL, path, operationID, param string) (string, string) {
		t.Helper()
		created := owner.do("POST", base+"/services", map[string]any{
			"workspace_id": workspaceID, "name": name, "protocol": "rest",
		}, true)
		require.Equal(t, 201, created.Status, created.Body)
		serviceID := created.Body["id"].(string)
		binding := owner.do("PUT", base+"/services/"+serviceID+"/environments/"+environmentID,
			map[string]any{"base_url": serverURL}, true)
		require.Equal(t, 200, binding.Status, binding.Body)
		spec := `{"openapi":"3.0.3","info":{"title":"` + name + `","version":"1.0.0"},"paths":{"` + path + `":{"get":{"operationId":"` + operationID + `","parameters":[{"name":"` + param + `","in":"path","required":true,"schema":{"type":"string"}}],"responses":{"200":{"description":"ok","content":{"application/json":{"schema":{"type":"object"}}}}}}}}}`
		imported := owner.do("POST", base+"/services/"+serviceID+"/import-openapi?dry_run=false",
			map[string]any{"spec": spec, "version_label": "1.0.0"}, true)
		require.Equal(t, 200, imported.Status, imported.Body)
		endpoints := owner.do("GET", base+"/services/"+serviceID+"/endpoints", nil, false)
		require.Equal(t, 200, endpoints.Status, endpoints.Body)
		require.Len(t, data(endpoints), 1)
		return serviceID, data(endpoints)[0]["id"].(string)
	}
	ordersService, ordersEndpoint := importService("Orders API", orders.URL, "/orders/{order_id}", "getOrder", "order_id")
	customersService, customersEndpoint := importService("Customers API", customers.URL, "/customers/{customer_id}", "getCustomer", "customer_id")

	mapping := owner.do("POST", base+"/mappings", map[string]any{
		"workspace_id": workspaceID,
		"name":         "Order overview",
		"draft": map[string]any{"rules": []any{
			map[string]any{"target": "order_id", "source": "$.steps.order.body.id"},
			map[string]any{"target": "order_status", "source": "$.steps.order.body.status"},
			map[string]any{"target": "customer", "source": "$.steps.customer.body.name"},
		}},
	}, true)
	require.Equal(t, 201, mapping.Status, mapping.Body)
	mappingID := mapping.Body["id"].(string)
	source := map[string]any{
		"request": map[string]any{"order_id": "o-42"},
		"steps": map[string]any{
			"order":    map[string]any{"status": 200, "body": map[string]any{"id": "o-42", "status": "paid", "customer_id": "c-7"}},
			"customer": map[string]any{"status": 200, "body": map[string]any{"id": "c-7", "name": "Ada"}},
		},
	}
	stored := owner.do("POST", base+"/mappings/"+mappingID+"/tests", map[string]any{
		"name": "Paid order overview", "input": source,
		"expected": map[string]any{"order_id": "o-42", "order_status": "paid", "customer": "Ada"},
	}, true)
	require.Equal(t, 201, stored.Status, stored.Body)
	publishedMapping := owner.do("POST", base+"/mappings/"+mappingID+"/publish", nil, true)
	require.Equal(t, 201, publishedMapping.Status, publishedMapping.Body)
	require.EqualValues(t, 1, publishedMapping.Body["version"])

	draft := map[string]any{
		"steps": []any{
			map[string]any{"id": "order", "call": map[string]any{
				"service_id": ordersService, "endpoint_id": ordersEndpoint,
				"input": map[string]any{"path_values": map[string]any{"order_id": "$.request.order_id"}},
			}},
			map[string]any{"id": "customer", "depends_on": []any{"order"}, "when": "steps.order.body.status == 'paid'",
				"call": map[string]any{
					"service_id": customersService, "endpoint_id": customersEndpoint,
					"input": map[string]any{"path_values": map[string]any{"customer_id": "$.steps.order.body.customer_id"}},
				}},
		},
		"response": map[string]any{"mapping_id": mappingID, "mapping_version": 1},
	}
	composition := owner.do("POST", base+"/api-compositions", map[string]any{
		"workspace_id": workspaceID, "name": "Order overview", "slug": "order-overview", "draft": draft,
	}, true)
	require.Equal(t, 201, composition.Status, composition.Body)
	compositionID := composition.Body["id"].(string)

	validated := owner.do("POST", base+"/api-compositions/"+compositionID+"/validate", nil, true)
	require.Equal(t, 200, validated.Status, validated.Body)
	require.Equal(t, true, validated.Body["valid"])
	published := owner.do("POST", base+"/api-compositions/"+compositionID+"/publish", nil, true)
	require.Equal(t, 201, published.Status, published.Body)
	require.EqualValues(t, 1, published.Body["version"])

	executed := owner.do("POST", base+"/api-compositions/"+compositionID+"/execute", map[string]any{
		"environment_id": environmentID, "request": map[string]any{"order_id": "o-42"},
	}, true)
	require.Equal(t, 200, executed.Status, executed.Body)
	require.Equal(t, "succeeded", executed.Body["status"], executed.Body)
	require.Equal(t, int64(1), orderCalls.Load())
	require.Equal(t, int64(1), customerCalls.Load())
	output := executed.Body["output"].(map[string]any)
	require.Equal(t, map[string]any{"order_id": "o-42", "order_status": "paid", "customer": "Ada"}, output)
	require.NotContains(t, output, "steps")
	steps := executed.Body["step_summary"].(map[string]any)
	require.EqualValues(t, 200, steps["order"].(map[string]any)["status"])
	require.EqualValues(t, 200, steps["customer"].(map[string]any)["status"])

	executionID := executed.Body["id"].(string)
	list := owner.do("GET", base+"/api-composition-executions?workspace_id="+workspaceID+"&composition_id="+compositionID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)
	require.Equal(t, executionID, data(list)[0]["id"])
	got := owner.do("GET", base+"/api-composition-executions/"+executionID, nil, false)
	require.Equal(t, 200, got.Status, got.Body)
	require.Equal(t, "succeeded", got.Body["status"])

	// A foreign tenant cannot see or run the composition or its executions.
	other := e.client()
	require.Equal(t, 201, other.do("POST", base+"/organizations",
		signupBody("other-composition", "owner@other-composition.test"), false).Status)
	require.Equal(t, 404, other.do("POST", base+"/api-compositions/"+compositionID+"/execute",
		map[string]any{"environment_id": environmentID}, true).Status)
	require.Equal(t, 404, other.do("GET", base+"/api-composition-executions/"+executionID, nil, false).Status)
	require.Equal(t, int64(1), orderCalls.Load(), "the refused execution made no upstream call")
}
