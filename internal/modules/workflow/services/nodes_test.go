package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

func newRegistry(t *testing.T) (Registry, *Expressions) {
	t.Helper()
	x, err := NewExpressions()
	require.NoError(t, err)
	return NewRegistry(x), x
}

func run(t *testing.T, r Registry, n model.Node, c NodeContext) (NodeResult, error) {
	t.Helper()
	c.Node = n
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return r[n.Type].Execute(ctx, c)
}

func TestExpressionsCompileEvaluateAndRejectNonBoolConditions(t *testing.T) {
	x, err := NewExpressions()
	require.NoError(t, err)
	require.Error(t, x.Check("input.amount >"))
	v, err := x.Eval(`{"total": input.amount * 2.0, "who": trigger.user}`,
		map[string]any{"input": map[string]any{"amount": 21.0}, "trigger": map[string]any{"user": "ana"}})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"total": 42.0, "who": "ana"}, v)
	_, err = x.EvalBool("1 + 1", map[string]any{})
	require.Error(t, err)
}

func TestIfNodeChoosesPortAndPassesInputThrough(t *testing.T) {
	r, _ := newRegistry(t)
	n := model.Node{ID: "check", Type: model.TypeLogicIf, Config: map[string]any{"condition": "input.amount > 100.0"}}
	res, err := run(t, r, n, NodeContext{Input: map[string]any{"amount": 250.0}})
	require.NoError(t, err)
	require.Equal(t, model.PortTrue, res.Port)
	require.Equal(t, map[string]any{"amount": 250.0}, res.Output)
	res, err = run(t, r, n, NodeContext{Input: map[string]any{"amount": 5.0}})
	require.NoError(t, err)
	require.Equal(t, model.PortFalse, res.Port)
}

func TestFilterNodeUsesItemAndUpstreamOutputs(t *testing.T) {
	r, _ := newRegistry(t)
	n := model.Node{ID: "f", Type: model.TypeLogicFilter, Config: map[string]any{
		"items": "nodes.load.output.orders", "condition": "item.total >= trigger.min"}}
	res, err := run(t, r, n, NodeContext{
		Trigger: map[string]any{"min": 10.0},
		Outputs: map[string]any{"load": map[string]any{"orders": []any{
			map[string]any{"id": "a", "total": 5.0}, map[string]any{"id": "b", "total": 15.0}}}},
	})
	require.NoError(t, err)
	require.Equal(t, []any{map[string]any{"id": "b", "total": 15.0}}, res.Output)

	_, err = run(t, r, model.Node{ID: "g", Type: model.TypeLogicFilter,
		Config: map[string]any{"condition": "true"}}, NodeContext{Input: "not a list"})
	require.ErrorContains(t, err, "not an array")
}

func TestTransformNodeAppliesMappingRules(t *testing.T) {
	r, _ := newRegistry(t)
	n := model.Node{ID: "t", Type: model.TypeActionTransform, Config: map[string]any{"rules": []any{
		map[string]any{"target": "customer.name", "source": "$.name"},
	}}}
	res, err := run(t, r, n, NodeContext{Input: map[string]any{"name": "Ana"}})
	require.NoError(t, err)
	require.Equal(t, map[string]any{"customer": map[string]any{"name": "Ana"}}, res.Output)
}

func TestHTTPNodeHonoursEgressPolicyStatusClassesAndIdempotencyKey(t *testing.T) {
	var gotKey string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotKey = req.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(req.Body).Decode(&gotBody)
		switch req.URL.Path {
		case "/flaky":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer srv.Close()
	r, _ := newRegistry(t)
	node := func(path string) model.Node {
		return model.Node{ID: "call", Type: model.TypeActionHTTP, Config: map[string]any{
			"method": "POST", "url": srv.URL + path, "body_expression": `{"amount": input.amount}`}}
	}
	_, err := run(t, r, node("/"), NodeContext{Input: map[string]any{"amount": 1.0}})
	var se *model.StepError
	require.ErrorAs(t, err, &se)
	require.Equal(t, model.ReasonEgressBlocked, se.Code, "loopback is denied unless allowlisted")

	loopback, err := egress.ParsePolicy([]string{"127.0.0.0/8", "::1"})
	require.NoError(t, err)
	res, err := run(t, r, node("/"), NodeContext{Input: map[string]any{"amount": 7.0}, Egress: loopback,
		IdempotencyKey: "exec:call::1"})
	require.NoError(t, err)
	require.Equal(t, "exec:call::1", gotKey)
	require.Equal(t, map[string]any{"amount": 7.0}, gotBody)
	require.Equal(t, map[string]any{"ok": true}, res.Output.(map[string]any)["body"])

	_, err = run(t, r, node("/flaky"), NodeContext{Input: map[string]any{"amount": 1.0}, Egress: loopback})
	require.ErrorAs(t, err, &se)
	require.True(t, se.Retryable, "5xx is retryable: %+v", se)
	_, err = run(t, r, node("/missing"), NodeContext{Input: map[string]any{"amount": 1.0}, Egress: loopback})
	require.ErrorAs(t, err, &se)
	require.False(t, se.Retryable, "4xx is a permanent failure")
}
