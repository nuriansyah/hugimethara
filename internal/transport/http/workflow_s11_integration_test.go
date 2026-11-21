//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	workflowsvc "github.com/nuriansyah/hugimethara/internal/modules/workflow/services"
)

func newTriggerRunner(t *testing.T, e *env) *workflowsvc.TriggerRunner {
	t.Helper()
	x, err := workflowsvc.NewExpressions()
	require.NoError(t, err)
	return workflowsvc.NewTriggerRunner(e.pg.System, x, workflowsvc.TriggerConfig{
		Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
}

func userID(t *testing.T, e *env, email string) string {
	t.Helper()
	var id string
	require.NoError(t, e.pg.Owner.GetContext(context.Background(), &id, `SELECT id::text FROM users WHERE email = $1`, email))
	return id
}

func orgID(t *testing.T, c *client) uuid.UUID {
	t.Helper()
	r := c.do("GET", base+"/organizations/current", nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	return uuid.MustParse(r.Body["id"].(string))
}

// dueTimers makes every armed timer due now (instead of sleeping for it).
func dueTimers(t *testing.T, e *env) {
	t.Helper()
	_, err := e.pg.Owner.ExecContext(context.Background(),
		`UPDATE workflow_timers SET fire_at = now() - interval '1 second' WHERE fired_at IS NULL`)
	require.NoError(t, err)
}

func (f workflowFixture) get(t *testing.T, execID string) resp {
	t.Helper()
	r := f.owner.do("GET", base+"/workflow-executions/"+execID, nil, false)
	require.Equal(t, 200, r.Status, r.Body)
	return r
}

func node(id, typ string, cfg map[string]any) map[string]any {
	return map[string]any{"id": id, "type": typ, "config": cfg}
}

func edge(from, to, port string) map[string]any {
	e := map[string]any{"from": from, "to": to}
	if port != "" {
		e["port"] = port
	}
	return e
}

func shape(target string) map[string]any {
	return map[string]any{"rules": []any{map[string]any{"target": target, "source": "$"}}}
}

// financeApprovalDraft: payments of 1000 or more need a finance approval
// (expiring after an hour, then escalated); smaller ones are auto-approved.
func financeApprovalDraft(paymentURL, groupID string) map[string]any {
	return map[string]any{
		"schema_version": 1,
		"trigger":        map[string]any{"type": "trigger.manual"},
		"limits":         map[string]any{"max_duration_seconds": 7200},
		"nodes": []any{
			node("route", "logic.switch", map[string]any{"cases": []any{
				map[string]any{"port": "large", "condition": "input.amount >= 1000.0"}}}),
			node("approve", "human.approval", map[string]any{
				"title": "Approve payment", "instructions": "Check the invoice before approving.",
				"assignees": map[string]any{"groups": []any{groupID}}, "timeout_seconds": 3600}),
			node("pay", "action.http", map[string]any{"method": "POST", "url": paymentURL + "/payments",
				"body_expression": `{"amount": input.request.amount, "approved_by": input.decided_by}`}),
			node("refuse", "action.transform", shape("refused")),
			node("escalate", "action.transform", shape("escalated")),
			node("auto", "action.transform", shape("auto_approved")),
		},
		"edges": []any{
			edge("trigger", "route", ""),
			edge("route", "approve", "large"),
			edge("route", "auto", "default"),
			edge("approve", "pay", "approved"),
			edge("approve", "refuse", "rejected"),
			edge("approve", "escalate", "timeout"),
		},
	}
}

// S11 acceptance: the finance approval workflow end to end — assignee rule,
// segregation of duties, approve / reject / expiry branches, and the
// decision resuming the waiting execution.
func TestFinanceApprovalWorkflow(t *testing.T) {
	e := setup(t)
	var payments sync.Map
	var paid atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		payments.Store(r.Header.Get("Idempotency-Key"), body)
		paid.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"payment":"ok"}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wf-fin")
	eng := newEngine(t, e, "worker-fin")
	carol, carolID := addMember(t, e, "wf-fin", "carol@wf-fin.test")
	dave, _ := addMember(t, e, "wf-fin", "dave@wf-fin.test")
	ownerID := userID(t, e, "owner@wf-fin.test")

	g := f.owner.do("POST", base+"/groups", map[string]any{"name": "Finance approvers"}, true)
	require.Equal(t, 201, g.Status, g.Body)
	groupID := g.Body["id"].(string)
	for _, u := range []string{carolID, ownerID} {
		require.Equal(t, 204, f.owner.do("POST", base+"/groups/"+groupID+"/members", map[string]any{"user_id": u}, true).Status)
	}
	id := f.deploy(t, "payments", financeApprovalDraft(srv.URL, groupID))

	// A large payment waits for an approval; the execution stays running.
	execID := f.execute(t, id, map[string]any{"amount": 5000, "invoice": "INV-1"}, "").Body["id"].(string)
	pump(t, e, eng)
	got := f.get(t, execID)
	require.Equal(t, "running", got.Body["status"])
	require.Equal(t, "waiting", stepsByNode(got)["approve"]["status"])
	require.Equal(t, "skipped", stepsByNode(got)["auto"]["status"])

	inbox := carol.do("GET", base+"/approval-tasks", nil, false)
	require.Equal(t, 200, inbox.Status, inbox.Body)
	require.Len(t, data(inbox), 1)
	task := data(inbox)[0]
	taskID := task["id"].(string)
	require.Equal(t, "pending", task["status"])
	require.Equal(t, true, task["can_decide"])
	require.Equal(t, "payments", task["workflow_name"])
	require.Equal(t, map[string]any{"amount": 5000.0, "invoice": "INV-1"}, task["context"])
	require.NotEmpty(t, task["due_at"])

	// Not an assignee: invisible. The requester is an assignee but may not
	// approve their own request (segregation of duties); they may see it.
	require.Empty(t, data(dave.do("GET", base+"/approval-tasks", nil, false)))
	require.Equal(t, 404, dave.do("GET", base+"/approval-tasks/"+taskID, nil, false).Status)
	r := dave.do("POST", base+"/approval-tasks/"+taskID+"/decide", map[string]any{"decision": "approve"}, true)
	require.Equal(t, 404, r.Status, r.Body)
	r = f.owner.do("POST", base+"/approval-tasks/"+taskID+"/decide", map[string]any{"decision": "approve"}, true)
	require.Equal(t, 403, r.Status, r.Body)
	require.Equal(t, "self_approval_forbidden", r.Body["code"])
	r = carol.do("POST", base+"/approval-tasks/"+taskID+"/decide", map[string]any{"decision": "submit"}, true)
	require.Equal(t, 422, r.Status, r.Body)

	// Carol approves: the step resumes on the approved branch and pays.
	r = carol.do("POST", base+"/approval-tasks/"+taskID+"/decide",
		map[string]any{"decision": "approve", "comment": "Invoice checked"}, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "approved", r.Body["status"])
	require.Equal(t, carolID, r.Body["decided_by"])
	require.Equal(t, false, r.Body["can_decide"])
	pump(t, e, eng)
	got = f.get(t, execID)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	steps := stepsByNode(got)
	require.Equal(t, "approved", steps["approve"]["port"])
	require.Equal(t, "succeeded", steps["pay"]["status"])
	require.Equal(t, "skipped", steps["refuse"]["status"])
	require.Equal(t, "skipped", steps["escalate"]["status"])
	require.EqualValues(t, 1, paid.Load())
	payments.Range(func(_, v any) bool {
		require.Equal(t, map[string]any{"amount": 5000.0, "approved_by": carolID}, v)
		return true
	})
	r = carol.do("POST", base+"/approval-tasks/"+taskID+"/decide", map[string]any{"decision": "reject"}, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "task_not_pending", r.Body["code"])
	require.Empty(t, data(carol.do("GET", base+"/approval-tasks?status=pending", nil, false)))
	require.Len(t, data(carol.do("GET", base+"/approval-tasks?status=approved", nil, false)), 1)

	// Small payments never reach a human.
	small := f.execute(t, id, map[string]any{"amount": 50}, "").Body["id"].(string)
	pump(t, e, eng)
	got = f.get(t, small)
	require.Equal(t, "succeeded", got.Body["status"])
	require.Equal(t, map[string]any{"auto": map[string]any{"auto_approved": map[string]any{"amount": 50.0}}}, got.Body["output"])

	// Rejection takes the rejected branch; nothing is paid.
	rejected := f.execute(t, id, map[string]any{"amount": 2000}, "").Body["id"].(string)
	pump(t, e, eng)
	tasks := data(carol.do("GET", base+"/approval-tasks?execution_id="+rejected, nil, false))
	require.Len(t, tasks, 1)
	r = carol.do("POST", base+"/approval-tasks/"+tasks[0]["id"].(string)+"/decide",
		map[string]any{"decision": "reject", "comment": "Duplicate invoice"}, true)
	require.Equal(t, 200, r.Status, r.Body)
	pump(t, e, eng)
	got = f.get(t, rejected)
	require.Equal(t, "succeeded", got.Body["status"])
	require.Equal(t, "succeeded", stepsByNode(got)["refuse"]["status"])
	require.Equal(t, "skipped", stepsByNode(got)["pay"]["status"])
	require.EqualValues(t, 1, paid.Load())

	// Nobody acts within the timeout: the task expires and escalation runs.
	expiring := f.execute(t, id, map[string]any{"amount": 3000}, "").Body["id"].(string)
	pump(t, e, eng)
	dueTimers(t, e)
	require.NoError(t, eng.ReapOnce(context.Background()))
	pump(t, e, eng)
	got = f.get(t, expiring)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	require.Equal(t, "timeout", stepsByNode(got)["approve"]["port"])
	require.Equal(t, "succeeded", stepsByNode(got)["escalate"]["status"])
	tasks = data(f.owner.do("GET", base+"/approval-tasks?assigned_to=any&workspace_id="+f.workspaceID+
		"&execution_id="+expiring, nil, false))
	require.Len(t, tasks, 1)
	require.Equal(t, "expired", tasks[0]["status"])

	require.Contains(t, auditActions(t, f.owner, "?resource_type=workflow"), "workflow.task_decided:success")
}

// An expired task whose node has no timeout branch fails the execution with
// a reason code instead of silently finishing.
func TestExpiredTaskWithoutTimeoutBranchFails(t *testing.T) {
	e := setup(t)
	f := newWorkflowFixture(t, e, "wf-exp")
	eng := newEngine(t, e, "worker-exp")
	ownerID := userID(t, e, "owner@wf-exp.test")
	id := f.deploy(t, "collect", map[string]any{"schema_version": 1, "trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("form", "human.input", map[string]any{"title": "Invoice number",
			"assignees": map[string]any{"users": []any{ownerID}}, "timeout_seconds": 60,
			"fields": []any{map[string]any{"key": "invoice_no", "required": true}}})},
		"edges": []any{edge("trigger", "form", "")}})

	// The manual-input path: the form is validated, then submitted.
	ok := f.execute(t, id, map[string]any{}, "").Body["id"].(string)
	pump(t, e, eng)
	task := data(f.owner.do("GET", base+"/approval-tasks", nil, false))[0]
	require.Equal(t, "input", task["kind"])
	r := f.owner.do("POST", base+"/approval-tasks/"+task["id"].(string)+"/decide",
		map[string]any{"decision": "submit", "data": map[string]any{"invoice_no": 7}}, true)
	require.Equal(t, 422, r.Status, r.Body)
	r = f.owner.do("POST", base+"/approval-tasks/"+task["id"].(string)+"/decide",
		map[string]any{"decision": "submit", "data": map[string]any{"invoice_no": "INV-9"}}, true)
	require.Equal(t, 200, r.Status, r.Body)
	pump(t, e, eng)
	got := f.get(t, ok)
	require.Equal(t, "succeeded", got.Body["status"])
	require.Equal(t, map[string]any{"invoice_no": "INV-9"}, stepsByNode(got)["form"]["output"].(map[string]any)["data"])

	expired := f.execute(t, id, map[string]any{}, "").Body["id"].(string)
	pump(t, e, eng)
	dueTimers(t, e)
	require.NoError(t, eng.ReapOnce(context.Background()))
	got = f.get(t, expired)
	require.Equal(t, "failed", got.Body["status"])
	require.Equal(t, "task_expired", got.Body["error"].(map[string]any)["code"])
}

// S11: cancel, delay timers and re-run from the failed step.
func TestCancelDelayAndRerunFromFailedStep(t *testing.T) {
	e := setup(t)
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusBadRequest) // not retryable: fails the execution
			_, _ = w.Write([]byte(`{"error":"ledger closed"}`))
			return
		}
		_, _ = w.Write([]byte(`{"posted":true}`))
	}))
	defer srv.Close()
	f := newWorkflowFixture(t, e, "wf-ctl")
	eng := newEngine(t, e, "worker-ctl")
	ctx := context.Background()

	delayed := f.deploy(t, "delayed", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"}, "limits": map[string]any{"max_duration_seconds": 7200},
		"nodes": []any{node("wait", "logic.delay", map[string]any{"seconds": 3600}),
			node("after", "action.transform", shape("waited"))},
		"edges": []any{edge("trigger", "wait", ""), edge("wait", "after", "")}})

	// Cancel a waiting execution: steps, timers and nothing left to fire.
	cancelled := f.execute(t, delayed, map[string]any{"n": 1}, "").Body["id"].(string)
	pump(t, e, eng)
	require.Equal(t, "waiting", stepsByNode(f.get(t, cancelled))["wait"]["status"])
	r := f.owner.do("POST", base+"/workflow-executions/"+cancelled+"/cancel", nil, true)
	require.Equal(t, 200, r.Status, r.Body)
	require.Equal(t, "cancelled", r.Body["status"])
	require.Equal(t, "cancelled", r.Body["error"].(map[string]any)["code"])
	require.Equal(t, "cancelled", stepsByNode(r)["wait"]["status"])
	r = f.owner.do("POST", base+"/workflow-executions/"+cancelled+"/cancel", nil, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "execution_finished", r.Body["code"])
	r = f.owner.do("POST", base+"/workflow-executions/"+cancelled+"/retry", nil, true)
	require.Equal(t, 409, r.Status)
	require.Equal(t, "execution_not_retryable", r.Body["code"])

	// The delay fires through its timer without holding a worker.
	waited := f.execute(t, delayed, map[string]any{"n": 2}, "").Body["id"].(string)
	pump(t, e, eng)
	require.NoError(t, eng.ReapOnce(ctx))
	require.Equal(t, "running", f.get(t, waited).Body["status"], "not due yet")
	dueTimers(t, e)
	require.NoError(t, eng.ReapOnce(ctx))
	pump(t, e, eng)
	got := f.get(t, waited)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	require.Equal(t, map[string]any{"waited": map[string]any{"n": 2.0}}, stepsByNode(got)["after"]["output"])
	var armed int
	require.NoError(t, e.pg.Owner.GetContext(ctx, &armed, `SELECT count(*) FROM workflow_timers WHERE fired_at IS NULL`))
	require.Zero(t, armed, "the cancelled execution's timer was disarmed")

	// Re-run from the failed step: succeeded steps are kept, not re-run.
	ledger := f.deploy(t, "ledger", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("prepare", "action.transform", shape("entry")),
			node("post", "action.http", map[string]any{"method": "POST", "url": srv.URL + "/ledger",
				"body_expression": "input"})},
		"edges": []any{edge("trigger", "prepare", ""), edge("prepare", "post", "")}})
	failed := f.execute(t, ledger, map[string]any{"amount": 10}, "").Body["id"].(string)
	pump(t, e, eng)
	got = f.get(t, failed)
	require.Equal(t, "failed", got.Body["status"])
	prepared := stepsByNode(got)["prepare"]["id"]

	r = f.owner.do("POST", base+"/workflow-executions/"+failed+"/retry", nil, true)
	require.Equal(t, 202, r.Status, r.Body)
	require.Equal(t, "running", r.Body["status"])
	require.EqualValues(t, 1, r.Body["rerun_count"])
	require.Nil(t, r.Body["error"])
	pump(t, e, eng)
	got = f.get(t, failed)
	require.Equal(t, "succeeded", got.Body["status"], got.Body)
	require.Equal(t, prepared, stepsByNode(got)["prepare"]["id"], "the succeeded step was kept")
	require.EqualValues(t, 2, got.Body["step_count"])
	require.EqualValues(t, 2, calls.Load())
	require.Contains(t, auditActions(t, f.owner, "?resource_type=workflow"), "workflow.execution_rerun:success")
	require.Contains(t, auditActions(t, f.owner, "?resource_type=workflow"), "workflow.execution_cancelled:success")
}

// S11 limits: a per-workflow concurrency limit and the loop iteration limit
// refuse work with explicit reason codes.
func TestConcurrencyAndLoopLimits(t *testing.T) {
	e := setup(t)
	f := newWorkflowFixture(t, e, "wf-lim")
	eng := newEngine(t, e, "worker-lim")

	single := f.deploy(t, "single", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"limits":  map[string]any{"max_duration_seconds": 7200, "max_concurrent_executions": 1},
		"nodes":   []any{node("wait", "logic.delay", map[string]any{"seconds": 3600})},
		"edges":   []any{edge("trigger", "wait", "")}})
	first := f.execute(t, single, map[string]any{}, "batch-1").Body["id"].(string)
	pump(t, e, eng)
	r := f.owner.do("POST", base+"/workflows/"+single+"/execute", map[string]any{"environment_id": f.envID}, true)
	require.Equal(t, 429, r.Status, r.Body)
	require.Equal(t, "concurrency_limit_exceeded", r.Body["code"])
	// An idempotent replay of the running execution is still answered.
	require.Equal(t, first, f.execute(t, single, map[string]any{}, "batch-1").Body["id"])
	require.Equal(t, 200, f.owner.do("POST", base+"/workflow-executions/"+first+"/cancel", nil, true).Status)
	f.execute(t, single, map[string]any{}, "")

	loop := f.deploy(t, "loop", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.manual"},
		"nodes": []any{node("each", "logic.loop", map[string]any{"items": "input.lines", "max_iterations": 2,
			"body": map[string]any{"type": "action.transform", "config": shape("line")}})},
		"edges": []any{edge("trigger", "each", "")}})
	ok := f.execute(t, loop, map[string]any{"lines": []any{1, 2}}, "").Body["id"].(string)
	tooMany := f.execute(t, loop, map[string]any{"lines": []any{1, 2, 3}}, "").Body["id"].(string)
	pump(t, e, eng)
	got := f.get(t, ok)
	require.Equal(t, "succeeded", got.Body["status"])
	require.Equal(t, []any{map[string]any{"line": 1.0}, map[string]any{"line": 2.0}}, stepsByNode(got)["each"]["output"])
	got = f.get(t, tooMany)
	require.Equal(t, "failed", got.Body["status"])
	require.Equal(t, "loop_limit_exceeded", got.Body["error"].(map[string]any)["code"])
}

// S11: schedule triggers fire once per activation and advance; event
// triggers start workflows from platform events, idempotently and without
// self-triggering or unbounded chains.
func TestScheduleAndEventTriggers(t *testing.T) {
	e := setup(t)
	f := newWorkflowFixture(t, e, "wf-trg")
	eng := newEngine(t, e, "worker-trg")
	runner := newTriggerRunner(t, e)
	ctx := context.Background()

	scheduled := f.deploy(t, "nightly", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.schedule",
			"config": map[string]any{"cron": "0 2 * * *", "timezone": "Asia/Jakarta", "input": map[string]any{"report": "daily"}}},
		"nodes": []any{node("build", "action.transform", shape("report"))},
		"edges": []any{edge("trigger", "build", "")}})
	list := f.owner.do("GET", base+"/workflow-schedules?workspace_id="+f.workspaceID, nil, false)
	require.Equal(t, 200, list.Status, list.Body)
	require.Len(t, data(list), 1)
	sched := data(list)[0]
	require.Equal(t, "0 2 * * *", sched["cron"])
	require.Equal(t, "Asia/Jakarta", sched["timezone"])
	next, err := time.Parse(time.RFC3339, sched["next_fire_at"].(string))
	require.NoError(t, err)
	require.Equal(t, 19, next.UTC().Hour(), "02:00 in Jakarta is 19:00 UTC")

	n, err := runner.FireDueSchedules(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "not due yet")
	_, err = e.pg.Owner.ExecContext(ctx, `UPDATE workflow_triggers SET next_fire_at = now() - interval '1 minute'
		WHERE workflow_id = $1`, scheduled)
	require.NoError(t, err)
	n, err = runner.FireDueSchedules(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	n, err = runner.FireDueSchedules(ctx)
	require.NoError(t, err)
	require.Zero(t, n, "the schedule advanced; the activation does not fire twice")
	pump(t, e, eng)
	runs := data(f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+scheduled, nil, false))
	require.Len(t, runs, 1)
	require.Equal(t, "trigger.schedule", runs[0]["trigger_type"])
	require.Equal(t, "succeeded", runs[0]["status"])
	require.Equal(t, map[string]any{"report": "daily"}, runs[0]["input"])
	sched = data(f.owner.do("GET", base+"/workflow-schedules?workspace_id="+f.workspaceID, nil, false))[0]
	next, err = time.Parse(time.RFC3339, sched["next_fire_at"].(string))
	require.NoError(t, err)
	require.True(t, next.After(time.Now()))

	// Disabling the deployment stops the schedule.
	versions := data(f.owner.do("GET", base+"/workflows/"+scheduled+"/versions", nil, false))
	r := f.owner.do("PUT", base+"/workflows/"+scheduled+"/deployments/"+f.envID,
		map[string]any{"version_id": versions[0]["id"], "enabled": false}, true)
	require.Equal(t, 200, r.Status, r.Body)
	sched = data(f.owner.do("GET", base+"/workflow-schedules?workspace_id="+f.workspaceID, nil, false))[0]
	require.Equal(t, false, sched["enabled"])
	require.Nil(t, sched["next_fire_at"])

	// Event trigger: start an incident workflow when any workflow fails.
	onFailure := f.deploy(t, "on-failure", map[string]any{"schema_version": 1,
		"trigger": map[string]any{"type": "trigger.event", "config": map[string]any{
			"event": "workflow.execution_finished", "condition": `input.data.status == "failed"`}},
		"nodes": []any{node("incident", "action.transform", shape("incident"))},
		"edges": []any{edge("trigger", "incident", "")}})
	org := orgID(t, f.owner)
	event := func(id, workflowID, status string, depth int) workflowsvc.Event {
		raw, _ := json.Marshal(map[string]any{"workflow_id": workflowID, "status": status, "trigger_depth": depth})
		return workflowsvc.Event{OrgID: org, Name: "workflow.execution_finished", ID: id, Data: raw}
	}
	count := func() int {
		return len(data(f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+onFailure, nil, false)))
	}
	require.NoError(t, runner.HandleEvent(ctx, event("outbox-1", scheduled, "succeeded", 0)))
	require.Zero(t, count(), "condition filters successful runs")
	require.NoError(t, runner.HandleEvent(ctx, event("outbox-2", scheduled, "failed", 0)))
	require.NoError(t, runner.HandleEvent(ctx, event("outbox-2", scheduled, "failed", 0)))
	require.Equal(t, 1, count(), "redelivery of one event starts one execution")
	require.NoError(t, runner.HandleEvent(ctx, event("outbox-3", onFailure, "failed", 0)))
	require.Equal(t, 1, count(), "a workflow is not started by its own events")
	require.NoError(t, runner.HandleEvent(ctx, event("outbox-4", scheduled, "failed", 5)))
	require.Equal(t, 1, count(), "event chains are bounded")
	pump(t, e, eng)
	runs = data(f.owner.do("GET", base+"/workflow-executions?workspace_id="+f.workspaceID+"&workflow_id="+onFailure, nil, false))
	require.Equal(t, "trigger.event", runs[0]["trigger_type"])
	require.Equal(t, "succeeded", runs[0]["status"])
	require.Equal(t, "workflow.execution_finished", runs[0]["input"].(map[string]any)["event"])

	// Other tenants' events never reach this organization's triggers.
	other := newWorkflowFixture(t, e, "wf-trg-other")
	ev := event("outbox-5", scheduled, "failed", 0)
	ev.OrgID = orgID(t, other.owner)
	require.NoError(t, runner.HandleEvent(ctx, ev))
	require.Equal(t, 1, count())
}
