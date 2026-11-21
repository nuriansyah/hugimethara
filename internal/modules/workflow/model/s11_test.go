package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func TestCronNext(t *testing.T) {
	utc := time.UTC
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		require.NoError(t, err)
		return v
	}
	cases := []struct {
		expr, after, want string
		loc               *time.Location
	}{
		{"* * * * *", "2026-11-21T10:00:30Z", "2026-11-21T10:01:00Z", utc},
		{"*/15 * * * *", "2026-11-21T10:01:00Z", "2026-11-21T10:15:00Z", utc},
		{"0 9 * * 1-5", "2026-11-21T10:00:00Z", "2026-11-23T09:00:00Z", utc}, // Saturday → Monday
		{"30 8 1 * *", "2026-11-21T00:00:00Z", "2026-12-01T08:30:00Z", utc},
		{"0 0 29 feb *", "2026-03-01T00:00:00Z", "2028-02-29T00:00:00Z", utc},
		{"0 12 * jan,jul sun", "2026-11-21T00:00:00Z", "2027-01-03T12:00:00Z", utc},
		{"0 0 1 * 7", "2026-11-21T00:00:00Z", "2026-11-22T00:00:00Z", utc}, // dom OR dow: Sunday 22nd
		// 09:00 in Jakarta (UTC+7) is 02:00 UTC.
		{"0 9 * * *", "2026-11-21T03:00:00Z", "2026-11-22T02:00:00Z", mustLoc(t, "Asia/Jakarta")},
		// Half-hour zone: 09:00 in Kolkata (UTC+5:30) is 03:30 UTC.
		{"0 9 * * *", "2026-11-21T00:00:00Z", "2026-11-21T03:30:00Z", mustLoc(t, "Asia/Kolkata")},
	}
	for _, c := range cases {
		cr, err := ParseCron(c.expr)
		require.NoError(t, err, c.expr)
		require.Equal(t, at(c.want), cr.Next(at(c.after), c.loc).UTC(), c.expr)
	}
	never, err := ParseCron("0 0 30 2 *")
	require.NoError(t, err)
	require.True(t, never.Next(at("2026-01-01T00:00:00Z"), utc).IsZero())
	for _, bad := range []string{"", "* * * *", "60 * * * *", "* 24 * * *", "5-1 * * * *", "*/0 * * * *", "x * * * *"} {
		_, err := ParseCron(bad)
		require.Error(t, err, bad)
	}
}

func TestCronSkipsDSTGapWithoutLooping(t *testing.T) {
	ny := mustLoc(t, "America/New_York")
	c, err := ParseCron("30 2 * * *")
	require.NoError(t, err)
	// 2027-03-14 02:30 does not exist in New York; the next real 02:30 is the 15th.
	got := c.Next(time.Date(2027, 3, 14, 0, 0, 0, 0, ny), ny)
	require.Equal(t, 15, got.Day())
}

func approvalNode(cfg map[string]any) Node {
	base := map[string]any{"title": "Approve payment", "assignees": map[string]any{
		"groups": []any{"0196f0d4-0000-7000-8000-000000000001"}}}
	for k, v := range cfg {
		base[k] = v
	}
	return Node{ID: "approve", Type: TypeHumanApproval, Config: base}
}

func financeDefinition() Definition {
	return Definition{SchemaVersion: 1, Trigger: Trigger{Type: TypeTriggerManual},
		Limits: Limits{MaxDurationSeconds: 7200},
		Nodes: []Node{
			{ID: "route", Type: TypeLogicSwitch, Config: map[string]any{"cases": []any{
				map[string]any{"port": "large", "condition": "input.amount >= 1000.0"}}}},
			approvalNode(map[string]any{"timeout_seconds": 3600.0}),
			{ID: "pay", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
			{ID: "auto", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
			{ID: "escalate", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
		},
		Edges: []Edge{
			{From: TriggerNodeID, To: "route"},
			{From: "route", To: "approve", Port: "large"},
			{From: "route", To: "auto", Port: PortDefault},
			{From: "approve", To: "pay", Port: PortApproved},
			{From: "approve", To: "escalate", Port: PortTimeout},
		}}
}

func TestValidateS11Nodes(t *testing.T) {
	require.Empty(t, Validate(financeDefinition(), nil))

	d := financeDefinition()
	d.Edges[1].Port = "huge" // not a case of the switch
	require.Equal(t, []string{"invalid_port"}, codes(Validate(d, nil)))

	d = financeDefinition()
	d.Nodes[1] = approvalNode(nil) // no timeout, but a timeout edge
	require.Equal(t, []string{"timeout_not_configured"}, codes(Validate(d, nil)))

	d = financeDefinition()
	d.Limits.MaxDurationSeconds = 600 // approval waits up to 1h
	require.Equal(t, []string{"timeout_exceeds_execution_limit"}, codes(Validate(d, nil)))

	d = financeDefinition()
	d.Nodes[1] = approvalNode(map[string]any{"assignees": map[string]any{"users": []any{"not-an-id"}}})
	d.Edges = d.Edges[:4]
	require.Equal(t, []string{"invalid_task"}, codes(Validate(d, nil)))

	d = financeDefinition()
	d.Nodes[0].Config = map[string]any{"cases": []any{map[string]any{"port": "default", "condition": "true"}}}
	require.Contains(t, codes(Validate(d, nil)), "invalid_cases")
}

func TestValidateLoopDelayAndTriggers(t *testing.T) {
	base := func(n Node) Definition {
		return Definition{SchemaVersion: 1, Trigger: Trigger{Type: TypeTriggerManual}, Nodes: []Node{n},
			Edges: []Edge{{From: TriggerNodeID, To: n.ID}}}
	}
	loop := Node{ID: "each", Type: TypeLogicLoop, Config: map[string]any{"body": map[string]any{
		"type": TypeActionHTTP, "config": map[string]any{"method": "GET", "url": "https://example.com/x"}}}}
	require.Empty(t, Validate(base(loop), nil))
	loop.Config["body"] = map[string]any{"type": TypeHumanApproval}
	require.Equal(t, []string{"invalid_loop"}, codes(Validate(base(loop), nil)))
	loop.Config["body"] = map[string]any{"type": TypeActionHTTP, "config": map[string]any{"method": "TRACE", "url": "x"}}
	require.ElementsMatch(t, []string{"invalid_method", "invalid_url"}, codes(Validate(base(loop), nil)))

	delay := Node{ID: "wait", Type: TypeLogicDelay, Config: map[string]any{"seconds": 30.0}}
	require.Empty(t, Validate(base(delay), nil))
	delay.Config["seconds"] = 7200.0 // default max duration is 1h
	require.Equal(t, []string{"timeout_exceeds_execution_limit"}, codes(Validate(base(delay), nil)))
	require.Equal(t, []string{"invalid_delay"}, codes(Validate(base(Node{ID: "w", Type: TypeLogicDelay}), nil)))

	d := base(delay)
	d.Nodes[0].Config["seconds"] = 1.0
	d.Trigger = Trigger{Type: TypeTriggerSchedule, Config: map[string]any{"cron": "0 9 * * 1-5", "timezone": "Asia/Jakarta"}}
	require.Empty(t, Validate(d, nil))
	d.Trigger.Config = map[string]any{"cron": "every day", "timezone": "Mars/Base"}
	require.Equal(t, []string{"invalid_schedule"}, codes(Validate(d, nil)))
	d.Trigger = Trigger{Type: TypeTriggerEvent, Config: map[string]any{"event": "workflow.execution_finished"}}
	require.Empty(t, Validate(d, nil))
	d.Trigger.Config = map[string]any{"event": "user.deleted"}
	require.Equal(t, []string{"invalid_event"}, codes(Validate(d, nil)))
	d.Trigger = Trigger{Type: TypeTriggerWebhook}
	require.Equal(t, []string{"trigger_unavailable"}, codes(Validate(d, nil)))

	d = base(delay)
	d.Limits.MaxConcurrentExecutions = -1
	require.Contains(t, codes(Validate(d, nil)), "limit_out_of_range")
}

func TestHumanInputFormValidation(t *testing.T) {
	n := Node{ID: "collect", Type: TypeHumanInput, Config: map[string]any{
		"title": "Invoice details", "assignees": map[string]any{"users": []any{"0196f0d4-0000-7000-8000-000000000002"}},
		"fields": []any{
			map[string]any{"key": "invoice_no", "label": "Invoice number", "required": true},
			map[string]any{"key": "amount", "type": "number", "required": true},
			map[string]any{"key": "urgent", "type": "boolean"},
		}}}
	task, err := ParseHumanTask(n)
	require.NoError(t, err)
	require.Equal(t, "input", task.Kind)
	require.Len(t, task.Fields, 3)

	out, issues := ValidateFormData(task.Fields, map[string]any{"invoice_no": "INV-7", "amount": 12.5, "urgent": true})
	require.Empty(t, issues)
	require.Equal(t, map[string]any{"invoice_no": "INV-7", "amount": 12.5, "urgent": true}, out)

	_, issues = ValidateFormData(task.Fields, map[string]any{"amount": "12", "extra": 1})
	require.ElementsMatch(t, []string{"required", "invalid_type", "unknown_field"}, codes(issues))
}

func TestRerunTransitionsAndWaitingState(t *testing.T) {
	for _, from := range []ExecutionStatus{ExecFailed, ExecTimedOut} {
		to, err := ExecutionTransition(from, ExRerun)
		require.NoError(t, err)
		require.Equal(t, ExecRunning, to)
	}
	for _, from := range []ExecutionStatus{ExecPending, ExecRunning, ExecSucceeded, ExecCancelled} {
		_, err := ExecutionTransition(from, ExRerun)
		require.Error(t, err, from)
	}
	to, err := StepTransition(StepRunning, EvSuspend)
	require.NoError(t, err)
	require.Equal(t, StepWaiting, to)
	to, err = StepTransition(StepWaiting, EvResume)
	require.NoError(t, err)
	require.Equal(t, StepSucceeded, to)
	require.False(t, StepWaiting.Terminal())

	// A waiting approval keeps successors unresolved: nothing is ready or skipped.
	d := financeDefinition()
	plan := Advance(d, map[string]NodeState{"route": {Status: StepSucceeded, Port: "large"},
		"approve": {Status: StepWaiting}})
	require.Empty(t, plan.Ready)
	require.Equal(t, []string{"auto"}, plan.Skipped)
	plan = Advance(d, map[string]NodeState{"route": {Status: StepSucceeded, Port: "large"},
		"auto": {Status: StepSkipped}, "approve": {Status: StepSucceeded, Port: PortApproved}})
	require.Equal(t, []string{"pay"}, plan.Ready)
	require.Equal(t, []string{"escalate"}, plan.Skipped)
}
