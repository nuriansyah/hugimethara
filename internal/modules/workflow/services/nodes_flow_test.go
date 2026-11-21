package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
)

func TestSwitchNodeRoutesToFirstMatchingCaseOrDefault(t *testing.T) {
	r, _ := newRegistry(t)
	n := model.Node{ID: "route", Type: model.TypeLogicSwitch, Config: map[string]any{"cases": []any{
		map[string]any{"port": "large", "condition": "input.amount >= 1000.0"},
		map[string]any{"port": "medium", "condition": "input.amount >= 100.0"},
	}}}
	for amount, port := range map[float64]string{5000: "large", 250: "medium", 5: model.PortDefault} {
		res, err := run(t, r, n, NodeContext{Input: map[string]any{"amount": amount}})
		require.NoError(t, err)
		require.Equal(t, port, res.Port, amount)
	}
}

func TestLoopNodeRunsBodyPerItemWithLimits(t *testing.T) {
	r, _ := newRegistry(t)
	n := model.Node{ID: "each", Type: model.TypeLogicLoop, Config: map[string]any{
		"items": "input.lines", "concurrency": 3.0, "max_iterations": 3.0,
		"body": map[string]any{"type": model.TypeActionTransform, "config": map[string]any{"rules": []any{
			map[string]any{"target": "sku", "source": "$.sku"}}}}}}
	in := map[string]any{"lines": []any{map[string]any{"sku": "a"}, map[string]any{"sku": "b"}}}
	res, err := run(t, r, n, NodeContext{Input: in})
	require.NoError(t, err)
	require.Equal(t, []any{map[string]any{"sku": "a"}, map[string]any{"sku": "b"}}, res.Output, "order preserved")

	in["lines"] = []any{1.0, 2.0, 3.0, 4.0}
	_, err = run(t, r, n, NodeContext{Input: in})
	var se *model.StepError
	require.ErrorAs(t, err, &se)
	require.Equal(t, model.ReasonLoopLimit, se.Code)
}

func TestLoopNodeGivesEachItemItsOwnIdempotencyKey(t *testing.T) {
	seen := make(chan string, 10)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		seen <- req.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	policy, err := egress.ParsePolicy([]string{"127.0.0.0/8"})
	require.NoError(t, err)
	r, _ := newRegistry(t)
	exec := uuid.New()
	n := model.Node{ID: "each", Type: model.TypeLogicLoop, Config: map[string]any{
		"body": map[string]any{"type": model.TypeActionHTTP, "config": map[string]any{"method": "GET", "url": srv.URL}}}}
	res, err := run(t, r, n, NodeContext{Input: []any{"x", "y"}, ExecutionID: exec, Attempt: 2, Egress: policy})
	require.NoError(t, err)
	require.Len(t, res.Output, 2)
	close(seen)
	var got []string
	for k := range seen {
		got = append(got, k)
	}
	require.ElementsMatch(t, []string{exec.String() + ":each:0:2", exec.String() + ":each:1:2"}, got)
}

func TestDelayAndHumanNodesSuspend(t *testing.T) {
	r, _ := newRegistry(t)
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

	res, err := run(t, r, model.Node{ID: "wait", Type: model.TypeLogicDelay, Config: map[string]any{"seconds": 90.0}},
		NodeContext{Now: now, Input: "x"})
	require.NoError(t, err)
	require.Equal(t, now.Add(90*time.Second), *res.Suspend.WakeAt)
	require.Nil(t, res.Suspend.Task)

	// An "until" in the past continues at once with the input.
	res, err = run(t, r, model.Node{ID: "wait", Type: model.TypeLogicDelay, Config: map[string]any{
		"until": `"2026-11-21T09:00:00Z"`}}, NodeContext{Now: now, Input: "x"})
	require.NoError(t, err)
	require.Nil(t, res.Suspend)
	require.Equal(t, "x", res.Output)

	res, err = run(t, r, model.Node{ID: "approve", Type: model.TypeHumanApproval, Config: map[string]any{
		"title": "Pay?", "timeout_seconds": 60.0,
		"assignees": map[string]any{"users": []any{uuid.NewString()}}}}, NodeContext{Now: now})
	require.NoError(t, err)
	require.Equal(t, "approval", res.Suspend.Task.Kind)
	require.Equal(t, now.Add(time.Minute), *res.Suspend.WakeAt)
}
