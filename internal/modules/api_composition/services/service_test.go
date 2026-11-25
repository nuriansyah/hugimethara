package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	compositionmodel "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model"
	compositiondto "github.com/nuriansyah/hugimethara/internal/modules/api_composition/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/api_composition/repository"
	mappingsvc "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/services"
	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
	serviceregdto "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model/dto"
)

type fakeInvoker struct {
	mu      sync.Mutex
	results map[string]serviceregmodel.ServiceTryResult
	errors  map[string]error
	calls   map[string]int
	inputs  map[string]serviceregdto.ServiceTryInput
}

func (f *fakeInvoker) TryEndpoint(_ context.Context, id uuid.UUID, input serviceregdto.ServiceTryInput) (serviceregmodel.ServiceTryResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := id.String()
	f.calls[key]++
	f.inputs[key] = input
	return f.results[key], f.errors[key]
}

func TestRunEvaluatesDependenciesConditionsAndInputReferences(t *testing.T) {
	const serviceID = "11111111-1111-4111-8111-111111111111"
	const orderEndpointID = "22222222-2222-4222-8222-222222222222"
	const customerEndpointID = "33333333-3333-4333-8333-333333333333"
	invoker := &fakeInvoker{
		results: map[string]serviceregmodel.ServiceTryResult{
			orderEndpointID:    {Status: 200, Body: map[string]any{"customer_id": "c-42", "status": "paid"}},
			customerEndpointID: {Status: 200, Body: map[string]any{"id": "c-42", "name": "Ada"}},
		},
		errors: map[string]error{},
		calls:  map[string]int{},
		inputs: map[string]serviceregdto.ServiceTryInput{},
	}
	svc := NewService(nil, nil, invoker)
	version := repository.VersionRow{Definition: []byte(`{
		"steps":[
			{"id":"order","call":{"service_id":"` + serviceID + `","endpoint_id":"` + orderEndpointID + `","input":{"path_values":{"id":"$.request.path.id"}}}},
			{"id":"customer","depends_on":["order"],"when":"steps.order.body.status != 'draft'",
			 "call":{"service_id":"` + serviceID + `","endpoint_id":"` + customerEndpointID + `","input":{"path_values":{"id":"$.steps.order.body.customer_id"}}}}
		]
	}`)}
	out, steps, err := svc.run(context.Background(), uuid.New(), version,
		compositiondto.ExecuteInput{EnvironmentID: uuid.New(), Request: map[string]any{"path": map[string]any{"id": "o-8"}}})
	require.NoError(t, err)
	require.Equal(t, "Ada", steps["customer"].Body.(map[string]any)["name"])
	require.Equal(t, "o-8", invoker.inputs[orderEndpointID].PathValues["id"])
	require.Equal(t, "c-42", invoker.inputs[customerEndpointID].PathValues["id"])
	result := out.(map[string]any)
	require.Equal(t, "paid", result["steps"].(map[string]any)["order"].(map[string]any)["body"].(map[string]any)["status"])
}

func TestRunAppliesOnErrorDefaultAndStopsOnFailure(t *testing.T) {
	const serviceID = "11111111-1111-4111-8111-111111111111"
	const endpointID = "22222222-2222-4222-8222-222222222222"
	invoker := &fakeInvoker{results: map[string]serviceregmodel.ServiceTryResult{}, errors: map[string]error{
		endpointID: errors.New("private upstream error"),
	}, calls: map[string]int{}}
	invoker.inputs = map[string]serviceregdto.ServiceTryInput{}
	svc := NewService(nil, nil, invoker)
	version := repository.VersionRow{Definition: []byte(`{
		"steps":[{"id":"optional","on_error":"default","default":{"available":false},
		"call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `"}}]
	}`)}
	out, steps, err := svc.run(context.Background(), uuid.New(), version, compositiondto.ExecuteInput{EnvironmentID: uuid.New()})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"available": false}, steps["optional"].Body)
	require.NotContains(t, out.(map[string]any)["steps"].(map[string]any)["optional"].(map[string]any)["error"], "private")

	version.Definition = []byte(`{
		"steps":[{"id":"required","call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `"}}]
	}`)
	_, _, err = svc.run(context.Background(), uuid.New(), version, compositiondto.ExecuteInput{EnvironmentID: uuid.New()})
	require.ErrorContains(t, err, "required")
}

func TestRunSkipsDescendantsWhenAConditionalStepIsSkipped(t *testing.T) {
	const serviceID = "11111111-1111-4111-8111-111111111111"
	const endpointID = "22222222-2222-4222-8222-222222222222"
	invoker := &fakeInvoker{results: map[string]serviceregmodel.ServiceTryResult{
		endpointID: {Status: 200, Body: map[string]any{"value": "should-not-run"}},
	}, errors: map[string]error{}, calls: map[string]int{}, inputs: map[string]serviceregdto.ServiceTryInput{}}
	svc := NewService(nil, nil, invoker)
	version := repository.VersionRow{Definition: []byte(`{
		"steps":[
			{"id":"optional","when":"request.enabled == true",
			 "call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `"}},
			{"id":"dependent","depends_on":["optional"],
			 "call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `"}}
		]
	}`)}
	_, steps, err := svc.run(context.Background(), uuid.New(), version,
		compositiondto.ExecuteInput{EnvironmentID: uuid.New(), Request: map[string]any{"enabled": false}})
	require.NoError(t, err)
	require.True(t, steps["optional"].Skipped)
	require.True(t, steps["dependent"].Skipped)
	require.Empty(t, invoker.calls)
}

func TestRunRejectsMalformedJSONPathInput(t *testing.T) {
	const serviceID = "11111111-1111-4111-8111-111111111111"
	const endpointID = "22222222-2222-4222-8222-222222222222"
	invoker := &fakeInvoker{results: map[string]serviceregmodel.ServiceTryResult{},
		errors: map[string]error{}, calls: map[string]int{}, inputs: map[string]serviceregdto.ServiceTryInput{}}
	svc := NewService(nil, nil, invoker)
	version := repository.VersionRow{Definition: []byte(`{
		"steps":[{"id":"invalid","call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `",
		 "input":{"body":{"id":"$.request["}}}}]
	}`)}
	_, _, err := svc.run(context.Background(), uuid.New(), version,
		compositiondto.ExecuteInput{EnvironmentID: uuid.New(), Request: map[string]any{}})
	require.ErrorContains(t, err, "step input reference is invalid")
	require.Empty(t, invoker.calls)
}

func TestClientErrorsDoNotOpenTheServiceCircuitBreaker(t *testing.T) {
	const serviceID = "11111111-1111-4111-8111-111111111111"
	const endpointID = "22222222-2222-4222-8222-222222222222"
	environmentID := uuid.New()
	orgID := uuid.New()
	invoker := &fakeInvoker{results: map[string]serviceregmodel.ServiceTryResult{
		endpointID: {Status: 404, Body: map[string]any{"message": "not found"}},
	}, errors: map[string]error{}, calls: map[string]int{}, inputs: map[string]serviceregdto.ServiceTryInput{}}
	svc := NewService(nil, nil, invoker)
	version := repository.VersionRow{Definition: []byte(`{
		"steps":[{"id":"lookup","call":{"service_id":"` + serviceID + `","endpoint_id":"` + endpointID + `"}}]
	}`)}
	_, steps, err := svc.run(context.Background(), orgID, version, compositiondto.ExecuteInput{EnvironmentID: environmentID})
	require.Error(t, err)
	require.Equal(t, 404, steps["lookup"].Status)
	require.Empty(t, svc.breaker)
}

func TestStoredJSONDecodingErrorsAreReturned(t *testing.T) {
	_, err := (repository.CompositionRow{Draft: []byte(`{`)}).Model()
	require.ErrorContains(t, err, "decode draft")

	_, err = (repository.VersionRow{Definition: []byte(`{`)}).Model()
	require.ErrorContains(t, err, "decode version")

	_, err = (repository.ExecutionRow{Output: []byte(`{`)}).Model()
	require.ErrorContains(t, err, "decode JSON value")
}

func TestCircuitBreakerResetsAfterQuietPeriod(t *testing.T) {
	svc := NewService(nil, nil, nil)
	now := time.Now()
	for range 4 {
		svc.breakerFailure("service", now)
	}

	require.True(t, svc.breakerAllow("service", now.Add(31*time.Second)))
	require.Empty(t, svc.breaker)
}

func TestResponseMappingAddressesStepBodies(t *testing.T) {
	results := map[string]compositionmodel.StepResult{
		"order":    {Status: 200, Body: map[string]any{"id": "o-42", "status": "paid"}},
		"customer": {Status: 200, Body: map[string]any{"name": "Ada"}},
	}
	out, err := mappingsvc.Apply(map[string]any{"rules": []any{
		map[string]any{"target": "order_id", "source": "$.steps.order.body.id"},
		map[string]any{"target": "order_status", "source": "$.steps.order.body.status"},
		map[string]any{"target": "customer", "source": "$.steps.customer.body.name"},
	}}, map[string]any{"request": map[string]any{"order_id": "o-42"}, "steps": stepValues(results)})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"order_id": "o-42", "order_status": "paid", "customer": "Ada"}, out)
}
