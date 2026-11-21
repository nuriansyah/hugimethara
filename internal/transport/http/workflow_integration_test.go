//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
)

// newEngine returns a worker-side engine on the hugi_system pool, exactly as
// hugi-worker builds it.
func newEngine(t *testing.T, e *env, worker string) *workflowsvc.Engine {
	t.Helper()
	x, err := workflowsvc.NewExpressions()
	require.NoError(t, err)
	return workflowsvc.NewEngine(e.pg.System, workflowsvc.NewRegistry(x), workflowsvc.EngineConfig{
		WorkerID: worker, Lease: 2 * time.Second, RedispatchAge: time.Hour,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// pendingDispatches takes the unsent step dispatches out of the outbox (what
// the relay would publish to JetStream) and marks them sent.
func pendingDispatches(t *testing.T, e *env) []workflowsvc.StepMessage {
	t.Helper()
	ctx := context.Background()
	var rows []struct {
		ID      int64  `db:"id"`
		Payload []byte `db:"payload"`
	}
	require.NoError(t, e.pg.Owner.SelectContext(ctx, &rows,
		`SELECT id, payload FROM outbox WHERE sent_at IS NULL AND subject LIKE 'hugi.wf.step.%' ORDER BY id`))
	out := make([]workflowsvc.StepMessage, 0, len(rows))
	for _, r := range rows {
		_, err := e.pg.Owner.ExecContext(ctx, `UPDATE outbox SET sent_at = now() WHERE id = $1`, r.ID)
		require.NoError(t, err)
		var m workflowsvc.StepMessage
		require.NoError(t, json.Unmarshal(r.Payload, &m))
		out = append(out, m)
	}
	return out
}

// pump delivers dispatches to the engine until the queue is empty, like the
// JetStream consumer does.
func pump(t *testing.T, e *env, eng *workflowsvc.Engine) int {
	t.Helper()
	handled := 0
	for range 100 {
		msgs := pendingDispatches(t, e)
		if len(msgs) == 0 {
			return handled
		}
		for _, m := range msgs {
			require.NoError(t, eng.HandleStep(context.Background(), m))
			handled++
		}
	}
	t.Fatal("workflow did not quiesce")
	return handled
}

type workflowFixture struct {
	owner       *client
	workspaceID string
	envID       string
}

func newWorkflowFixture(t *testing.T, e *env, slug string) workflowFixture {
	t.Helper()
	owner := e.client()
	require.Equal(t, 201, owner.do("POST", base+"/organizations", signupBody(slug, "owner@"+slug+".test"), false).Status)
	ws := owner.do("POST", base+"/workspaces", map[string]any{"slug": "ops", "name": "Ops"}, true)
	require.Equal(t, 201, ws.Status, ws.Body)
	envID := developmentEnvironment(t, e, owner)
	allowLoopbackForTest(t, e, envID)
	return workflowFixture{owner: owner, workspaceID: ws.Body["id"].(string), envID: envID}
}

// deploy creates, publishes and deploys a workflow; it returns its id.
func (f workflowFixture) deploy(t *testing.T, slug string, draft map[string]any) string {
	t.Helper()
	created := f.owner.do("POST", base+"/workflows", map[string]any{
		"workspace_id": f.workspaceID, "name": slug, "slug": slug, "draft": draft}, true)
	require.Equal(t, 201, created.Status, created.Body)
	id := created.Body["id"].(string)
	published := f.owner.do("POST", base+"/workflows/"+id+"/publish", nil, true)
	require.Equal(t, 201, published.Status, published.Body)
	deployed := f.owner.do("PUT", base+"/workflows/"+id+"/deployments/"+f.envID, map[string]any{
		"version_id": published.Body["id"], "enabled": true}, true)
	require.Equal(t, 200, deployed.Status, deployed.Body)
	return id
}

func (f workflowFixture) execute(t *testing.T, workflowID string, input map[string]any, key string) resp {
	t.Helper()
	body := map[string]any{"environment_id": f.envID, "input": input}
	if key != "" {
		body["idempotency_key"] = key
	}
	r := f.owner.do("POST", base+"/workflows/"+workflowID+"/execute", body, true)
	require.Equal(t, 202, r.Status, r.Body)
	return r
}

func stepsByNode(r resp) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, s := range r.Body["steps"].([]any) {
		m := s.(map[string]any)
		out[m["node_id"].(string)] = m
	}
	return out
}

func conditionalParallelDraft(serverURL string) map[string]any {
	return map[string]any{
		"schema_version": 1,
		"trigger":        map[string]any{"type": "trigger.manual"},
		"nodes": []any{
			map[string]any{"id": "check", "type": "logic.if", "config": map[string]any{"condition": "input.amount > 100.0"}},
			map[string]any{"id": "fan", "type": "logic.parallel"},
			map[string]any{"id": "notify", "type": "action.http", "config": map[string]any{
				"method": "POST", "url": serverURL + "/notify", "body_expression": `{"amount": input.amount}`}},
			map[string]any{"id": "shape", "type": "action.transform", "config": map[string]any{"rules": []any{
				map[string]any{"target": "approved_amount", "source": "$.amount"}}}},
			map[string]any{"id": "join", "type": "logic.merge"},
			map[string]any{"id": "small", "type": "action.transform", "config": map[string]any{"rules": []any{
				map[string]any{"target": "auto_approved", "source": "$.amount"}}}},
		},
		"edges": []any{
			map[string]any{"from": "trigger", "to": "check"},
			map[string]any{"from": "check", "to": "fan", "port": "true"},
			map[string]any{"from": "check", "to": "small", "port": "false"},
			map[string]any{"from": "fan", "to": "notify"},
			map[string]any{"from": "fan", "to": "shape"},
			map[string]any{"from": "notify", "to": "join"},
			map[string]any{"from": "shape", "to": "join"},
		},
	}
}

// S10 acceptance: a conditional + parallel workflow executes, both branches
// of the parallel join in a merge, and the untaken branch is skipped.
func TestWorkflowConditionalParallelExecution(t *testing.T) {
	e := setup(t)
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"notified":true}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wf-flow")
	eng := newEngine(t, e, "worker-a")

	types := f.owner.do("GET", base+"/node-types", nil, false)
	require.Equal(t, 200, types.Status, types.Body)
	require.NotEmpty(t, data(types))

	id := f.deploy(t, "approvals", conditionalParallelDraft(srv.URL))

	started := f.execute(t, id, map[string]any{"amount": 250}, "order-1")
	execID := started.Body["id"].(string)
	require.Equal(t, "pending", started.Body["status"])
	pump(t, e, eng)

	got := f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, 200, got.Status, got.Body)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	steps := stepsByNode(got)
	require.Equal(t, "true", steps["check"]["port"])
	require.Equal(t, "skipped", steps["small"]["status"])
	for _, n := range []string{"check", "fan", "notify", "shape", "join"} {
		require.Equal(t, "succeeded", steps[n]["status"], n)
	}
	join := steps["join"]["output"].(map[string]any)
	require.Equal(t, map[string]any{"approved_amount": 250.0}, join["shape"])
	require.Equal(t, true, join["notify"].(map[string]any)["body"].(map[string]any)["notified"])
	require.EqualValues(t, 1, calls.Load())
	require.EqualValues(t, 6, got.Body["step_count"])

	// Idempotency-Key replays return the original execution and run nothing.
	again := f.execute(t, id, map[string]any{"amount": 250}, "order-1")
	require.Equal(t, execID, again.Body["id"])
	require.Zero(t, pump(t, e, eng))

	// The false branch: the whole parallel/merge subgraph is dead-path skipped.
	small := f.execute(t, id, map[string]any{"amount": 5}, "")
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+small.Body["id"].(string), nil, false)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	steps = stepsByNode(got)
	require.Equal(t, "succeeded", steps["small"]["status"])
	for _, n := range []string{"fan", "notify", "shape", "join"} {
		require.Equal(t, "skipped", steps[n]["status"], n)
	}
	require.Equal(t, map[string]any{"small": map[string]any{"auto_approved": 5.0}}, got.Body["output"])

	history := f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+id, nil, false)
	require.Equal(t, 200, history.Status, history.Body)
	require.Len(t, data(history), 2)

	// Tenant isolation: another organization cannot see the execution.
	other := newWorkflowFixture(t, e, "wf-other")
	require.Equal(t, 404, other.owner.do("GET", base+"/workflow-executions/"+execID, nil, false).Status)
}

// S10 acceptance: a worker that dies mid-step loses its lease; the reaper
// re-queues the step and another worker finishes the execution.
func TestWorkflowRecoversFromWorkerCrash(t *testing.T) {
	e := setup(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wf-crash")
	id := f.deploy(t, "crash", conditionalParallelDraft(srv.URL))
	execID := f.execute(t, id, map[string]any{"amount": 500}, "").Body["id"].(string)

	// worker-dead claims the first step, then "crashes": its message is gone
	// and it never completes or heartbeats.
	msgs := pendingDispatches(t, e)
	require.Len(t, msgs, 1)
	ctx := context.Background()
	res, err := e.pg.System.ExecContext(ctx, `UPDATE workflow_execution_steps
		SET status = 'running', worker_id = 'worker-dead', lease_until = now() + interval '1 hour', started_at = now()
		WHERE id = $1 AND status = 'queued'`, msgs[0].StepID)
	require.NoError(t, err)
	n, _ := res.RowsAffected()
	require.EqualValues(t, 1, n)

	eng := newEngine(t, e, "worker-b")
	require.Zero(t, pump(t, e, eng), "nothing to do while the dead worker holds the lease")
	require.NoError(t, eng.ReapOnce(ctx))
	require.Empty(t, pendingDispatches(t, e), "an unexpired lease is not stolen")

	// The lease expires.
	_, err = e.pg.Owner.ExecContext(ctx, `UPDATE workflow_execution_steps SET lease_until = now() - interval '1 second'
		WHERE id = $1`, msgs[0].StepID)
	require.NoError(t, err)
	require.NoError(t, eng.ReapOnce(ctx))
	require.Greater(t, pump(t, e, eng), 0)

	got := f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	require.Equal(t, "succeeded", stepsByNode(got)["check"]["status"])

	// A late completion by the dead worker is fenced out.
	var owner string
	require.NoError(t, e.pg.Owner.GetContext(ctx, &owner,
		`SELECT coalesce(worker_id, '') FROM workflow_execution_steps WHERE id = $1`, msgs[0].StepID))
	require.Empty(t, owner)
}

// S10 acceptance: failures are inspectable — the failed step keeps its input,
// the upstream response and a reason code; retries are bounded by policy.
func TestWorkflowFailuresAreInspectableAndRetriesAreBounded(t *testing.T) {
	e := setup(t)
	var flaky atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/flaky" {
			flaky.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":"down"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad customer id"}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wf-fail")
	eng := newEngine(t, e, "worker-c")
	ctx := context.Background()

	// An invalid draft is refused with every issue listed.
	bad := f.owner.do("POST", base+"/workflows", map[string]any{"workspace_id": f.workspaceID, "name": "Bad",
		"slug": "bad", "draft": map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
			"nodes": []any{map[string]any{"id": "x", "type": "logic.if", "config": map[string]any{"condition": "input.("}},
				map[string]any{"id": "y", "type": "action.connector"}},
			"edges": []any{map[string]any{"from": "trigger", "to": "x"}}}}, true)
	require.Equal(t, 201, bad.Status, bad.Body)
	v := f.owner.do("POST", base+"/workflows/"+bad.Body["id"].(string)+"/validate", nil, true)
	require.Equal(t, 200, v.Status, v.Body)
	require.Equal(t, false, v.Body["valid"])
	pub := f.owner.do("POST", base+"/workflows/"+bad.Body["id"].(string)+"/publish", nil, true)
	require.Equal(t, 422, pub.Status, pub.Body)
	require.Equal(t, "invalid_workflow", pub.Body["code"])
	var codes []string
	for _, x := range pub.Body["errors"].([]any) {
		codes = append(codes, x.(map[string]any)["code"].(string))
	}
	require.Contains(t, codes, "invalid_expression")
	require.Contains(t, codes, "node_type_unavailable")

	// Permanent failure (4xx): the execution fails at once with a reason code.
	permanent := f.deploy(t, "permanent", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{map[string]any{"id": "lookup", "type": "action.http",
			"config": map[string]any{"method": "GET", "url": srv.URL + "/customers/42"}}},
		"edges": []any{map[string]any{"from": "trigger", "to": "lookup"}}})
	execID := f.execute(t, permanent, map[string]any{"customer": 42}, "").Body["id"].(string)
	pump(t, e, eng)
	got := f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "failed", got.Body["status"])
	execErr := got.Body["error"].(map[string]any)
	require.Equal(t, "node_failed", execErr["code"])
	require.Equal(t, "lookup", execErr["node_id"])
	step := stepsByNode(got)["lookup"]
	require.Equal(t, "failed", step["status"])
	require.Equal(t, map[string]any{"customer": 42.0}, step["input"])
	require.EqualValues(t, 400, step["output"].(map[string]any)["status"])
	require.Contains(t, step["error"].(map[string]any)["message"], "400")

	// Retryable failure (5xx) with max_attempts 2: one retry, then failed.
	retrying := f.deploy(t, "retrying", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{map[string]any{"id": "sync", "type": "action.http",
			"config": map[string]any{"method": "GET", "url": srv.URL + "/flaky"},
			"retry":  map[string]any{"max_attempts": 2, "backoff": "fixed", "initial_ms": 1}}},
		"edges": []any{map[string]any{"from": "trigger", "to": "sync"}}})
	execID = f.execute(t, retrying, map[string]any{}, "").Body["id"].(string)
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "running", got.Body["status"])
	require.Equal(t, "retry_scheduled", stepsByNode(got)["sync"]["status"])

	time.Sleep(20 * time.Millisecond)
	require.NoError(t, eng.ReapOnce(ctx))
	pump(t, e, eng)
	got = f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, "failed", got.Body["status"], got.Body)
	step = stepsByNode(got)["sync"]
	require.EqualValues(t, 2, step["attempt"])
	require.Equal(t, true, step["error"].(map[string]any)["retryable"])
	require.EqualValues(t, 2, flaky.Load())

	// Not deployed in an environment → explicit conflict, not a silent no-op.
	prod := uuid.NewString()
	r := f.owner.do("POST", base+"/workflows/"+retrying+"/execute", map[string]any{"environment_id": prod}, true)
	require.Equal(t, 409, r.Status, r.Body)
	require.Equal(t, "workflow_not_deployed", r.Body["code"])
}
