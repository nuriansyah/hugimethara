package services

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// S11 control-plane use cases: cancel, re-run, schedules and the approvals
// inbox. Like the rest of WorkflowService they run as hugi_app in a
// tenant-bound transaction.

// loadExecution reads an execution and authorizes action on its workflow.
func (s *WorkflowService) loadExecution(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	id uuid.UUID) (model.Execution, error) {
	e, err := repository.GetExecution(ctx, tx, id)
	if db.IsNoRows(err) {
		return e, model.ErrExecutionNotFound
	}
	if err != nil {
		return e, err
	}
	return e, s.authorize(ctx, p, action, e.WorkspaceID, e.WorkflowID.String())
}

func (s *WorkflowService) auditExecution(ctx context.Context, tx *sqlx.Tx, p principal.Principal, action string,
	e model.Execution, details map[string]any) error {
	return s.audit(ctx, tx, p, action, model.Workflow{ID: e.WorkflowID, WorkspaceID: e.WorkspaceID}, details)
}

// CancelExecution stops an unfinished execution.
func (s *WorkflowService) CancelExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Execution{}, err
	}
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		e, err := s.loadExecution(ctx, tx, p, "execute", executionID)
		if err != nil {
			return err
		}
		locked, err := repository.LockExecution(ctx, tx, executionID)
		if err != nil {
			return err
		}
		if _, err := model.ExecutionTransition(model.ExecutionStatus(locked.Status), model.ExCancel); err != nil {
			return model.ErrExecutionFinished
		}
		if err := finish(ctx, tx, locked, model.ExecCancelled, nil, &model.StepError{Code: model.ReasonCancelled,
			Message: "The execution was cancelled by a user."}); err != nil {
			return err
		}
		return s.auditExecution(ctx, tx, p, "workflow.execution_cancelled", e,
			map[string]any{"execution_id": executionID, "previous_status": locked.Status})
	})
	if err != nil {
		return model.Execution{}, err
	}
	return s.GetExecution(ctx, executionID)
}

// RerunExecution re-runs a failed or timed-out execution from its failed
// step: the failed step and the steps cancelled because of it are removed and
// progression queues them again with inputs recomputed from the succeeded
// steps, which are not re-run. The deadline restarts from now.
func (s *WorkflowService) RerunExecution(ctx context.Context, executionID uuid.UUID) (model.Execution, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.Execution{}, err
	}
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		e, err := s.loadExecution(ctx, tx, p, "execute", executionID)
		if err != nil {
			return err
		}
		locked, err := repository.LockExecution(ctx, tx, executionID)
		if err != nil {
			return err
		}
		if _, err := model.ExecutionTransition(model.ExecutionStatus(locked.Status), model.ExRerun); err != nil {
			return model.ErrNotRetryable
		}
		def, err := repository.EngineDefinition(ctx, tx, locked.VersionID)
		if err != nil {
			return err
		}
		removed, err := repository.DeleteRerunSteps(ctx, tx, executionID)
		if err != nil {
			return err
		}
		if _, err := repository.ResetExecution(ctx, tx, executionID,
			s.clock.Now().Add(def.Limits.EffectiveMaxDuration()), len(removed)); err != nil {
			return err
		}
		if locked, err = repository.LockExecution(ctx, tx, executionID); err != nil {
			return err
		}
		if _, err := progress(ctx, tx, s.ids, locked, def); err != nil {
			return err
		}
		details := map[string]any{"execution_id": executionID, "rerun_nodes": removed,
			"previous_status": e.Status, "rerun": e.RerunCount + 1}
		if e.Error != nil {
			details["previous_error"] = map[string]any{"code": e.Error.Code, "node_id": e.Error.NodeID,
				"message": e.Error.Message}
		}
		return s.auditExecution(ctx, tx, p, "workflow.execution_rerun", e, details)
	})
	if err != nil {
		return model.Execution{}, err
	}
	return s.GetExecution(ctx, executionID)
}

// ListSchedules lists the workspace's deployed schedule triggers.
func (s *WorkflowService) ListSchedules(ctx context.Context, workspaceID uuid.UUID) ([]model.Schedule, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, p, "read", workspaceID, ""); err != nil {
		return nil, err
	}
	var out []model.Schedule
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		out, err = repository.ListSchedules(ctx, tx, workspaceID)
		return err
	})
	return out, err
}

// ---- approvals inbox -------------------------------------------------------------

func isAssignee(t model.ApprovalTask, userID uuid.UUID, groups []uuid.UUID) bool {
	for _, u := range t.Users {
		if u == userID {
			return true
		}
	}
	for _, g := range t.Groups {
		for _, mine := range groups {
			if g == mine {
				return true
			}
		}
	}
	return false
}

func withCanDecide(t model.ApprovalTask, userID uuid.UUID, groups []uuid.UUID) model.ApprovalTask {
	t.CanDecide = t.Status == model.TaskPending && isAssignee(t, userID, groups)
	return t
}

// ListTasks lists human tasks for the inbox.
func (s *WorkflowService) ListTasks(ctx context.Context, f dto.TaskFilter) ([]model.ApprovalTask, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return nil, err
	}
	switch model.TaskStatus(f.Status) {
	case "", model.TaskPending, model.TaskApproved, model.TaskRejected, model.TaskSubmitted, model.TaskExpired,
		model.TaskCancelled:
	default:
		return nil, validation("status", "invalid_status", "Unknown task status.")
	}
	rf := repository.TaskFilter{WorkspaceID: f.WorkspaceID, ExecutionID: f.ExecutionID, Status: f.Status, Limit: f.Limit}
	if rf.Limit <= 0 {
		rf.Limit = defaultHistoryLimit
	}
	if rf.Limit > maxHistoryLimit {
		rf.Limit = maxHistoryLimit
	}
	if f.Mine {
		rf.Assignee = &p.ID
	} else {
		if f.WorkspaceID == nil {
			return nil, validation("workspace_id", "required", "workspace_id is required unless assigned_to=me.")
		}
		if err := s.authorize(ctx, p, "read", *f.WorkspaceID, ""); err != nil {
			return nil, err
		}
	}
	var out []model.ApprovalTask
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		groups, err := repository.UserGroups(ctx, tx, p.ID)
		if err != nil {
			return err
		}
		rows, err := repository.ListTasks(ctx, tx, rf)
		if err != nil {
			return err
		}
		out = make([]model.ApprovalTask, len(rows))
		for i, t := range rows {
			out[i] = withCanDecide(t, p.ID, groups)
		}
		return nil
	})
	return out, err
}

// GetTask returns a task to an assignee or to someone who can read the
// workflow; anyone else gets not-found.
func (s *WorkflowService) GetTask(ctx context.Context, id uuid.UUID) (model.ApprovalTask, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.ApprovalTask{}, err
	}
	var out model.ApprovalTask
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		t, groups, err := s.visibleTask(ctx, tx, p, id)
		out = withCanDecide(t, p.ID, groups)
		return err
	})
	return out, err
}

func (s *WorkflowService) visibleTask(ctx context.Context, tx *sqlx.Tx, p principal.Principal,
	id uuid.UUID) (model.ApprovalTask, []uuid.UUID, error) {
	t, err := repository.GetTask(ctx, tx, id)
	if db.IsNoRows(err) {
		return t, nil, model.ErrTaskNotFound
	}
	if err != nil {
		return t, nil, err
	}
	groups, err := repository.UserGroups(ctx, tx, p.ID)
	if err != nil {
		return t, nil, err
	}
	if isAssignee(t, p.ID, groups) {
		return t, groups, nil
	}
	if s.authorize(ctx, p, "read", t.WorkspaceID, t.WorkflowID.String()) != nil {
		return t, nil, model.ErrTaskNotFound
	}
	return t, groups, nil
}

// DecideTask resolves a pending task and resumes the waiting step with the
// decision as its output. The assignee rule is the only authorization: being
// able to edit the workflow does not let you approve its requests.
func (s *WorkflowService) DecideTask(ctx context.Context, id uuid.UUID, in dto.DecideInput) (model.ApprovalTask, error) {
	p, err := s.principal(ctx)
	if err != nil {
		return model.ApprovalTask{}, err
	}
	in.Comment = strings.TrimSpace(in.Comment)
	if len([]rune(in.Comment)) > 2000 {
		return model.ApprovalTask{}, validation("comment", "too_long", "comment must be at most 2000 characters.")
	}
	err = db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		t, groups, err := s.visibleTask(ctx, tx, p, id)
		if err != nil {
			return err
		}
		if !isAssignee(t, p.ID, groups) {
			return model.ErrNotAssignee
		}
		// Lock order: execution, then task — the same as timer expiry.
		exec, err := repository.LockExecution(ctx, tx, t.ExecutionID)
		if err != nil {
			return err
		}
		if t, err = repository.LockTask(ctx, tx, id); err != nil {
			return err
		}
		if t.Status != model.TaskPending || model.ExecutionStatus(exec.Status).Terminal() {
			return model.ErrTaskDecided
		}
		status, port, output, err := decision(t, p.ID, in)
		if err != nil {
			return err
		}
		if ok, err := repository.DecideTask(ctx, tx, id, status, output, in.Comment, p.ID); err != nil || !ok {
			if err == nil {
				err = model.ErrTaskDecided
			}
			return err
		}
		def, err := repository.EngineDefinition(ctx, tx, exec.VersionID)
		if err != nil {
			return err
		}
		if _, err := resume(ctx, tx, s.ids, exec, def, t.StepID, port, output); err != nil {
			return err
		}
		if _, err := outbox.Publish(ctx, tx, outbox.Event{OrgID: p.OrgID,
			Subject: "hugi.events.workflow.approval_decided", Payload: map[string]any{
				"task_id": id, "execution_id": t.ExecutionID, "workflow_id": t.WorkflowID,
				"workspace_id": t.WorkspaceID, "node_id": t.NodeID, "decision": status, "decided_by": p.ID,
				"trigger_depth": exec.TriggerDepth}}); err != nil {
			return err
		}
		return s.audit(ctx, tx, p, "workflow.task_decided", model.Workflow{ID: t.WorkflowID, WorkspaceID: t.WorkspaceID},
			map[string]any{"task_id": id, "execution_id": t.ExecutionID, "node_id": t.NodeID, "decision": status})
	})
	if err != nil {
		return model.ApprovalTask{}, err
	}
	return s.GetTask(ctx, id)
}

// decision validates a decision against the task and returns the task status,
// the branch port and the step output it resolves to.
func decision(t model.ApprovalTask, userID uuid.UUID, in dto.DecideInput) (model.TaskStatus, string, map[string]any, error) {
	out := map[string]any{"decided_by": userID.String(), "comment": in.Comment}
	switch {
	case t.Kind == "approval" && in.Decision == model.DecisionApprove:
		if !t.AllowSelfApproval && t.RequestedBy != nil && *t.RequestedBy == userID {
			return "", "", nil, model.ErrSelfApproval
		}
		out["decision"] = string(model.TaskApproved)
		return model.TaskApproved, model.PortApproved, withRequest(out, t), nil
	case t.Kind == "approval" && in.Decision == model.DecisionReject:
		out["decision"] = string(model.TaskRejected)
		return model.TaskRejected, model.PortRejected, withRequest(out, t), nil
	case t.Kind == "input" && in.Decision == model.DecisionSubmit:
		data, issues := model.ValidateFormData(t.Fields, in.Data)
		if len(issues) > 0 {
			fields := make([]apperrors.FieldError, len(issues))
			for i, is := range issues {
				fields[i] = apperrors.FieldError{Field: is.Path, Code: is.Code, Message: is.Message}
			}
			return "", "", nil, apperrors.Validation(fields...)
		}
		out["decision"] = string(model.TaskSubmitted)
		out["data"] = data
		return model.TaskSubmitted, model.PortSubmitted, withRequest(out, t), nil
	}
	allowed := "approve or reject"
	if t.Kind == "input" {
		allowed = "submit"
	}
	return "", "", nil, validation("decision", "invalid_decision", "decision must be "+allowed+" for this task.")
}

// withRequest carries the task context (the step input) into the output, so
// nodes after an approval still see what was approved as input.request.
func withRequest(out map[string]any, t model.ApprovalTask) map[string]any {
	if len(t.Context) > 0 {
		var v any
		if err := json.Unmarshal(t.Context, &v); err == nil {
			out["request"] = v
		}
	}
	return out
}
