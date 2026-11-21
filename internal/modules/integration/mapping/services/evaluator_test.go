package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	mappingmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/mapping/model"
)

func TestApplyPRDExample(t *testing.T) {
	draft := map[string]any{"rules": []any{
		map[string]any{"target": "name", "source": "$.customer_name", "transforms": []any{
			map[string]any{"op": "trim"},
		}},
		map[string]any{"target": "amount", "source": "$.total_amount", "transforms": []any{
			map[string]any{"op": "to_number"},
		}, "default": float64(0)},
		map[string]any{"target": "items[]", "foreach": "$.lines", "rules": []any{
			map[string]any{"target": "sku", "source": "$.item.sku"},
		}},
		map[string]any{"target": "tier", "expr": "src.total_amount > 1000000 ? 'gold' : 'std'", "when": "has(src.total_amount)"},
	}}

	out, err := Apply(draft, map[string]any{
		"customer_name": "  Ada Lovelace  ",
		"total_amount":  float64(25),
		"lines": []any{
			map[string]any{"item": map[string]any{"sku": "A"}},
			map[string]any{"item": map[string]any{"sku": "B"}},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "Ada Lovelace", out.(map[string]any)["name"])
	require.Equal(t, float64(25), out.(map[string]any)["amount"])
	require.Equal(t, "std", out.(map[string]any)["tier"])
	require.Equal(t, []any{
		map[string]any{"sku": "A"},
		map[string]any{"sku": "B"},
	}, out.(map[string]any)["items"])
}

func TestApplyLimitsAndValidation(t *testing.T) {
	rules := make([]any, mappingmodel.MaxRules+1)
	for i := range rules {
		rules[i] = map[string]any{"target": "x", "source": "$.x"}
	}
	require.Error(t, ValidateDraft(map[string]any{"rules": rules}))
	require.Error(t, ValidateDraft(map[string]any{"rules": []any{
		map[string]any{"target": "x", "expr": "missing("},
	}}))
}

func TestJSONPathFieldAndIndex(t *testing.T) {
	input := map[string]any{"orders": []any{
		map[string]any{"id": "a"},
		map[string]any{"id": "b"},
	}}
	value, err := mappingmodel.JsonPath(input, "$.orders[1].id")
	require.NoError(t, err)
	require.Equal(t, "b", value)
}
