package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDraftChecksDAGAndReferences(t *testing.T) {
	serviceID := "11111111-1111-4111-8111-111111111111"
	endpointID := "22222222-2222-4222-8222-222222222222"
	valid := map[string]any{"steps": []any{
		map[string]any{"id": "order", "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
		map[string]any{"id": "customer", "depends_on": []any{"order"},
			"when": "steps.order.body.status != 'draft'",
			"call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
	}}
	require.NoError(t, ValidateDraft(valid))

	cases := []struct {
		name  string
		draft map[string]any
		want  string
	}{
		{"duplicate ids", map[string]any{"steps": []any{
			map[string]any{"id": "same", "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
			map[string]any{"id": "same", "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
		}}, "duplicated"},
		{"unknown dependency", map[string]any{"steps": []any{
			map[string]any{"id": "a", "depends_on": []any{"missing"}, "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
		}}, "unknown step"},
		{"cycle", map[string]any{"steps": []any{
			map[string]any{"id": "a", "depends_on": []any{"b"}, "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
			map[string]any{"id": "b", "depends_on": []any{"a"}, "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
		}}, "cycle"},
		{"fallback dependency required", map[string]any{"steps": []any{
			map[string]any{"id": "backup", "call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
			map[string]any{"id": "main", "on_error": "fallback_step", "fallback_step": "backup",
				"call": map[string]any{"service_id": serviceID, "endpoint_id": endpointID}},
		}}, "must depend"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateDraft(tc.draft)
			require.ErrorContains(t, err, tc.want)
		})
	}
}

func TestValidateDraftEnforcesHardLimits(t *testing.T) {
	err := ValidateDraft(map[string]any{
		"limits": map[string]any{"timeout_ms": 30001},
		"steps": []any{map[string]any{
			"id": "a",
			"call": map[string]any{
				"service_id":  "11111111-1111-4111-8111-111111111111",
				"endpoint_id": "22222222-2222-4222-8222-222222222222",
			},
		}},
	})
	require.ErrorContains(t, err, "timeout")
}

func TestValidateDraftAllowsAnEmptyEditableDraft(t *testing.T) {
	require.NoError(t, ValidateDraft(map[string]any{"steps": []any{}}))
	require.ErrorContains(t, ValidateDraft(map[string]any{}), "steps must be an array")
}
