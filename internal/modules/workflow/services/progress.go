package services

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	"github.com/nuriansyah/hugimethara/internal/platform/id"
)

// StepSubject is the JetStream subject a step dispatch is published on; the
// 4th token carries the tenant for per-org observability (bus.Streams).
func StepSubject(orgID uuid.UUID) string { return "hugi.wf.step." + orgID.String() }

// StepMessage is the dispatch payload. It carries ids only: the worker reads
// everything else from Postgres, the source of truth.
type StepMessage struct {
	StepID uuid.UUID `json:"step_id"`
	OrgID  uuid.UUID `json:"org_id"`
}

// dispatch enqueues a step through the transactional outbox, so the message
// exists if and only if the transaction that queued the step commits.
func dispatch(ctx context.Context, tx *sqlx.Tx, orgID, stepID uuid.UUID) error {
	_, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: orgID, Subject: StepSubject(orgID),
		Payload: StepMessage{StepID: stepID, OrgID: orgID}})
	return err
}

// snapshot is the progression view of an execution's steps.
type snapshot struct {
	states  map[string]model.NodeState
	outputs map[string]any
}

func loadSnapshot(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) (snapshot, error) {
	rows, err := repository.StepStates(ctx, tx, executionID)
	if err != nil {
		return snapshot{}, err
	}
	s := snapshot{states: make(map[string]model.NodeState, len(rows)), outputs: map[string]any{}}
	for _, r := range rows {
		s.states[r.NodeID] = model.NodeState{Status: model.StepStatus(r.Status), Port: r.Port}
		if model.StepStatus(r.Status) == model.StepSucceeded && len(r.Output) > 0 {
			var v any
			if err := json.Unmarshal(r.Output, &v); err != nil {
				return snapshot{}, err
			}
			s.outputs[r.NodeID] = v
		}
	}
	return s, nil
}

// progressResult tells the caller what happened to the execution.
type progressResult struct {
	finished model.ExecutionStatus // empty while running
	output   any
	err      *model.StepError
}

// progress applies model.Advance: it inserts skipped steps, queues ready ones
// (with their computed input) and dispatches them, and finishes the
// execution when nothing is left to run. The caller must hold the execution
// row lock so concurrent completions serialize.
func progress(ctx context.Context, tx *sqlx.Tx, ids id.Generator, exec repository.LockedExecution,
	def model.Definition) (progressResult, error) {
	snap, err := loadSnapshot(ctx, tx, exec.ID)
	if err != nil {
		return progressResult{}, err
	}
	var trigger any
	if len(exec.Input) > 0 {
		if err := json.Unmarshal(exec.Input, &trigger); err != nil {
			return progressResult{}, err
		}
	}
	plan := model.Advance(def, snap.states)
	if exec.StepCount+len(plan.Ready)+len(plan.Skipped) > exec.MaxSteps {
		se := &model.StepError{Code: model.ReasonMaxStepsExceeded,
			Message: "The execution would exceed its step limit; raise limits.max_steps or simplify the graph."}
		return progressResult{finished: model.ExecFailed, err: se}, finish(ctx, tx, exec, model.ExecFailed, nil, se)
	}
	created := 0
	for _, nodeID := range plan.Skipped {
		n, _ := def.Node(nodeID)
		ok, err := repository.InsertStep(ctx, tx, repository.NewStep{ID: ids.New(), OrgID: exec.OrgID,
			ExecutionID: exec.ID, NodeID: nodeID, NodeType: n.Type, Status: model.StepSkipped})
		if err != nil {
			return progressResult{}, err
		}
		if ok {
			created++
		}
		snap.states[nodeID] = model.NodeState{Status: model.StepSkipped}
	}
	for _, nodeID := range plan.Ready {
		n, _ := def.Node(nodeID)
		input, err := json.Marshal(model.NodeInput(def, nodeID, snap.states, snap.outputs, trigger))
		if err != nil {
			return progressResult{}, err
		}
		if len(input) > model.MaxInlinePayload {
			se := &model.StepError{Code: model.ReasonPayloadTooLarge, NodeID: nodeID,
				Message: "The node input exceeds the inline payload limit (64 KiB)."}
			return progressResult{finished: model.ExecFailed, err: se}, finish(ctx, tx, exec, model.ExecFailed, nil, se)
		}
		stepID := ids.New()
		ok, err := repository.InsertStep(ctx, tx, repository.NewStep{ID: stepID, OrgID: exec.OrgID,
			ExecutionID: exec.ID, NodeID: nodeID, NodeType: n.Type, Status: model.StepQueued, Input: input})
		if err != nil {
			return progressResult{}, err
		}
		if !ok {
			continue // another completion already queued it
		}
		created++
		snap.states[nodeID] = model.NodeState{Status: model.StepQueued}
		if err := dispatch(ctx, tx, exec.OrgID, stepID); err != nil {
			return progressResult{}, err
		}
	}
	if created > 0 {
		if err := repository.AddStepCount(ctx, tx, exec.ID, created); err != nil {
			return progressResult{}, err
		}
	}
	for _, st := range snap.states {
		if !st.Status.Terminal() {
			return progressResult{}, nil
		}
	}
	if len(snap.states) == 0 {
		return progressResult{}, nil
	}
	out := map[string]any{}
	for _, sink := range model.Sinks(def) {
		if v, ok := snap.outputs[sink]; ok {
			out[sink] = v
		}
	}
	return progressResult{finished: model.ExecSucceeded, output: out},
		finish(ctx, tx, exec, model.ExecSucceeded, out, nil)
}

// finish moves the execution to a terminal state, cancels leftover steps and
// emits a domain event for downstream consumers (notifications, analytics).
func finish(ctx context.Context, tx *sqlx.Tx, exec repository.LockedExecution, status model.ExecutionStatus,
	output any, se *model.StepError) error {
	if status != model.ExecSucceeded {
		if err := repository.CancelOpenSteps(ctx, tx, exec.ID); err != nil {
			return err
		}
	}
	if err := repository.FinishExecution(ctx, tx, exec.ID, status, output, se); err != nil {
		return err
	}
	payload := map[string]any{"execution_id": exec.ID, "workflow_id": exec.WorkflowID,
		"workspace_id": exec.WorkspaceID, "status": status}
	if se != nil {
		payload["error_code"] = se.Code
	}
	_, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: exec.OrgID,
		Subject: "hugi.events.workflow.execution_finished", Payload: payload})
	return err
}
