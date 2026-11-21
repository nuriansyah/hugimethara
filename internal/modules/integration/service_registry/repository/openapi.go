package repository

import (
	"encoding/json"
	"sort"

	serviceregmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/service_registry/model"
)

func Diff(current []serviceregmodel.Endpoint, previous []DiffEndpointRow) serviceregmodel.ImportResult {
	old := make(map[string]DiffEndpointRow, len(previous))
	for _, endpoint := range previous {
		old[serviceregmodel.EndpointKey(endpoint.Method, endpoint.Path)] = endpoint
	}
	newEndpoints := make(map[string]serviceregmodel.Endpoint, len(current))
	for _, endpoint := range current {
		newEndpoints[serviceregmodel.EndpointKey(endpoint.Method, endpoint.Path)] = endpoint
	}
	changes := make([]serviceregmodel.OpenAPIChange, 0)
	for key, endpoint := range newEndpoints {
		before, exists := old[key]
		if !exists {
			changes = append(changes, serviceregmodel.OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "added"})
			continue
		}
		if !sameEndpoint(endpoint, before) {
			changes = append(changes, serviceregmodel.OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "changed"})
		}
	}
	for key, endpoint := range old {
		if _, exists := newEndpoints[key]; !exists {
			changes = append(changes, serviceregmodel.OpenAPIChange{OperationID: endpoint.OperationID, Method: endpoint.Method,
				Path: endpoint.Path, Kind: "removed", Breaking: true})
		}
	}
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Method < changes[j].Method
	})
	return serviceregmodel.ImportResult{Changes: changes, EndpointCount: len(current)}
}

func sameEndpoint(current serviceregmodel.Endpoint, previous DiffEndpointRow) bool {
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
