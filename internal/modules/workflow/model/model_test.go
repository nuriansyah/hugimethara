package model

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// conditionalParallel is the S10 acceptance graph: an if whose true branch
// fans out to two parallel HTTP-free nodes that join in a merge, while the
// false branch is a dead path.
func conditionalParallel() Definition {
	return Definition{
		SchemaVersion: 1,
		Trigger:       Trigger{Type: TypeTriggerManual},
		Nodes: []Node{
			{ID: "check", Type: TypeLogicIf, Config: map[string]any{"condition": "input.amount > 100"}},
			{ID: "fan", Type: TypeLogicParallel},
			{ID: "a", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
			{ID: "b", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
			{ID: "join", Type: TypeLogicMerge},
			{ID: "small", Type: TypeActionTransform, Config: map[string]any{"rules": []any{}}},
		},
		Edges: []Edge{
			{From: TriggerNodeID, To: "check"},
			{From: "check", To: "fan", Port: PortTrue},
			{From: "check", To: "small", Port: PortFalse},
			{From: "fan", To: "a"}, {From: "fan", To: "b"},
			{From: "a", To: "join"}, {From: "b", To: "join"},
		},
	}
}

func codes(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, is := range issues {
		out[i] = is.Code
	}
	return out
}

func TestValidateAcceptsConditionalParallelGraph(t *testing.T) {
	require.Empty(t, Validate(conditionalParallel(), func(string) error { return nil }))
}

func TestValidateReportsEveryStructuralIssue(t *testing.T) {
	d := conditionalParallel()
	d.Nodes = append(d.Nodes,
		Node{ID: "orphan", Type: TypeLogicParallel},
		Node{ID: "conn", Type: TypeActionConnector},
	)
	d.Edges = append(d.Edges, Edge{From: "a", To: "b"}) // b now has two inputs without a merge
	issues := Validate(d, nil)
	require.Contains(t, codes(issues), "node_type_unavailable")

	d.Nodes = d.Nodes[:len(d.Nodes)-1]
	issues = Validate(d, nil)
	require.ElementsMatch(t, []string{"join_requires_merge", "unreachable"}, codes(issues))
}

func TestValidateRejectsCyclesPortsAndBadExpressions(t *testing.T) {
	d := conditionalParallel()
	d.Edges = append(d.Edges, Edge{From: "join", To: "fan"})
	require.Contains(t, codes(Validate(d, nil)), "join_requires_merge")

	d = conditionalParallel()
	d.Nodes[4] = Node{ID: "join", Type: TypeLogicMerge}
	d.Edges = append(d.Edges, Edge{From: "join", To: "check"})
	require.Contains(t, codes(Validate(d, nil)), "join_requires_merge")

	d = Definition{SchemaVersion: 1, Trigger: Trigger{Type: TypeTriggerManual},
		Nodes: []Node{{ID: "x", Type: TypeLogicParallel}, {ID: "y", Type: TypeLogicMerge}},
		Edges: []Edge{{From: TriggerNodeID, To: "y"}, {From: "x", To: "y"}, {From: "y", To: "x"}}}
	require.Contains(t, codes(Validate(d, nil)), "cycle")

	d = conditionalParallel()
	d.Edges[1].Port = "maybe"
	d.Edges[3].Port = "true"
	require.Equal(t, []string{"invalid_port", "invalid_port"}, codes(Validate(d, nil)))

	d = conditionalParallel()
	issues := Validate(d, func(expr string) error { return errors.New("undeclared reference") })
	require.Equal(t, []string{"invalid_expression"}, codes(issues))
	require.Equal(t, "nodes[0].config.condition", issues[0].Path)
}

func TestValidateRejectsUnsafeHTTPConfig(t *testing.T) {
	d := Definition{SchemaVersion: 1, Trigger: Trigger{Type: TypeTriggerManual},
		Nodes: []Node{{ID: "call", Type: TypeActionHTTP, Config: map[string]any{
			"method": "TRACE", "url": "https://user:pw@example.com", "headers": map[string]any{"X\r\nInjected": "1"},
		}}},
		Edges: []Edge{{From: TriggerNodeID, To: "call"}}}
	require.ElementsMatch(t, []string{"invalid_method", "invalid_url", "invalid_headers"}, codes(Validate(d, nil)))
}

func TestStepStateMachineIsExhaustive(t *testing.T) {
	statuses := []StepStatus{StepPending, StepQueued, StepRunning, StepWaiting, StepSucceeded, StepFailed, StepSkipped,
		StepCancelled, StepRetryScheduled}
	events := []StepEvent{EvDispatch, EvClaim, EvSucceed, EvFail, EvScheduleRetry, EvRetryDue, EvLeaseExpired,
		EvSkip, EvCancel, EvSuspend, EvResume}
	valid := 0
	for _, s := range statuses {
		for _, ev := range events {
			to, err := StepTransition(s, ev)
			if s.Terminal() {
				require.Error(t, err, "terminal %s must not accept %s", s, ev)
				continue
			}
			if err == nil {
				valid++
				require.NotEqual(t, s, to)
			} else {
				require.Equal(t, s, to)
			}
		}
	}
	require.Equal(t, len(stepTable), valid)

	to, err := StepTransition(StepRunning, EvLeaseExpired)
	require.NoError(t, err)
	require.Equal(t, StepQueued, to, "a crashed worker's step goes back to the queue")
	_, err = StepTransition(StepSucceeded, EvCancel)
	require.Error(t, err)
}

func TestExecutionStateMachine(t *testing.T) {
	to, err := ExecutionTransition(ExecPending, ExStart)
	require.NoError(t, err)
	require.Equal(t, ExecRunning, to)
	to, err = ExecutionTransition(ExecRunning, ExTimeout)
	require.NoError(t, err)
	require.Equal(t, ExecTimedOut, to)
	for _, s := range []ExecutionStatus{ExecSucceeded, ExecFailed, ExecCancelled, ExecTimedOut} {
		for _, ev := range []ExecutionEvent{ExStart, ExComplete, ExFail, ExCancel, ExTimeout} {
			_, err := ExecutionTransition(s, ev)
			require.Error(t, err)
		}
	}
}

func TestAdvanceTakesTrueBranchAndSkipsDeadPath(t *testing.T) {
	d := conditionalParallel()
	plan := Advance(d, map[string]NodeState{})
	require.Equal(t, []string{"check"}, plan.Ready)

	states := map[string]NodeState{"check": {Status: StepSucceeded, Port: PortTrue}}
	plan = Advance(d, states)
	require.Equal(t, []string{"fan"}, plan.Ready)
	require.Equal(t, []string{"small"}, plan.Skipped)

	states["fan"] = NodeState{Status: StepSucceeded}
	states["small"] = NodeState{Status: StepSkipped}
	require.Equal(t, []string{"a", "b"}, Advance(d, states).Ready)

	states["a"] = NodeState{Status: StepSucceeded}
	states["b"] = NodeState{Status: StepRunning}
	require.Empty(t, Advance(d, states).Ready, "merge waits for every active branch")
	states["b"] = NodeState{Status: StepSucceeded}
	require.Equal(t, []string{"join"}, Advance(d, states).Ready)
}

func TestAdvanceFalseBranchSkipsMergeWithoutDeadlock(t *testing.T) {
	d := conditionalParallel()
	plan := Advance(d, map[string]NodeState{"check": {Status: StepSucceeded, Port: PortFalse}})
	require.Equal(t, []string{"small"}, plan.Ready)
	require.Equal(t, []string{"a", "b", "fan", "join"}, plan.Skipped)
}

func TestNodeInputForMergeAndEntry(t *testing.T) {
	d := conditionalParallel()
	states := map[string]NodeState{"a": {Status: StepSucceeded}, "b": {Status: StepSucceeded}}
	in := NodeInput(d, "join", states, map[string]any{"a": 1, "b": 2}, nil)
	require.Equal(t, map[string]any{"a": 1, "b": 2}, in)
	require.Equal(t, map[string]any{"amount": 5}, NodeInput(d, "check", nil, nil, map[string]any{"amount": 5}))
}

func TestLimitsAndRetryBackoff(t *testing.T) {
	require.Equal(t, DefaultMaxSteps, Limits{}.EffectiveMaxSteps())
	require.Equal(t, MaxStepsCap, Limits{MaxSteps: 1e6}.EffectiveMaxSteps())
	require.Equal(t, MaxDurationCap, Limits{MaxDurationSeconds: 1e9}.EffectiveMaxDuration())
	n := Node{Retry: &RetryPolicy{MaxAttempts: 50, InitialMs: 100}}
	require.Equal(t, MaxRetryAttempts, n.MaxAttempts())
	require.Equal(t, 100*time.Millisecond, n.RetryDelay(2))
	require.Equal(t, 400*time.Millisecond, n.RetryDelay(4))
	require.Equal(t, 1, Node{}.MaxAttempts())
}

func TestChecksumIsStable(t *testing.T) {
	a, _, err := Checksum(conditionalParallel())
	require.NoError(t, err)
	b, _, err := Checksum(conditionalParallel())
	require.NoError(t, err)
	require.Equal(t, a, b)
	require.Len(t, a, 64)
}

// The shipped examples must stay publishable.
func TestExampleDefinitionsAreValid(t *testing.T) {
	for _, f := range []string{"order-approval.json", "finance-approval.json"} {
		raw, err := os.ReadFile("../../../../examples/workflows/" + f)
		require.NoError(t, err)
		d, err := ParseDefinition(raw)
		require.NoError(t, err, f)
		require.Empty(t, Validate(d, nil), f)
	}
}
