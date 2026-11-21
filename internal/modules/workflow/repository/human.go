package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

// ---- suspension ----------------------------------------------------------------

// SuspendStep parks a running step as waiting. ok is false when this worker
// no longer holds the lease.
func SuspendStep(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, worker string, input []byte) (bool, error) {
	return affected(tx.ExecContext(ctx, commands.SQL.Get("SuspendStep"), id, worker, nullJSON(input)))
}

// ResumeStep completes a waiting step with output and port. ok is false when
// the step is no longer waiting (already resumed, or cancelled).
func ResumeStep(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, output any, port string) (bool, error) {
	out, err := json.Marshal(output)
	if err != nil {
		return false, err
	}
	return affected(tx.ExecContext(ctx, commands.SQL.Get("ResumeStep"), id, out, port))
}

// FailWaitingStep fails a waiting step (e.g. an expired task without a
// timeout branch).
func FailWaitingStep(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, se *model.StepError) (bool, error) {
	errJSON, err := marshalErr(se)
	if err != nil {
		return false, err
	}
	return affected(tx.ExecContext(ctx, commands.SQL.Get("FailWaitingStep"), id, errJSON))
}

// WaitingStep is the state needed to resume a step.
type WaitingStep struct {
	ID       uuid.UUID `db:"id"`
	NodeID   string    `db:"node_id"`
	NodeType string    `db:"node_type"`
	Status   string    `db:"status"`
	Input    []byte    `db:"input"`
}

// StepByID reads one step.
func StepByID(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (WaitingStep, error) {
	var s WaitingStep
	err := tx.GetContext(ctx, &s, queries.SQL.Get("StepByID"), id)
	return s, err
}

// ---- timers ----------------------------------------------------------------------

// Timer kinds.
const (
	TimerDelay       = "delay"
	TimerTaskTimeout = "task_timeout"
)

// Timer is a due timer locked for firing.
type Timer struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	ExecutionID uuid.UUID `db:"execution_id"`
	StepID      uuid.UUID `db:"step_id"`
	Kind        string    `db:"kind"`
}

// InsertTimer arms a timer for a waiting step.
func InsertTimer(ctx context.Context, tx *sqlx.Tx, id, orgID, executionID, stepID uuid.UUID, kind string, at time.Time) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertTimer"), id, orgID, executionID, stepID, kind, at)
	return err
}

// DueTimers lists timers whose time has come (no lock; LockDueTimer claims).
func DueTimers(ctx context.Context, q sqlx.QueryerContext, limit int) ([]StepRef, error) {
	var out []StepRef
	err := sqlx.SelectContext(ctx, q, &out, queries.SQL.Get("DueTimers"), limit)
	return out, err
}

// TimerExecution returns the execution a timer belongs to.
func TimerExecution(ctx context.Context, tx *sqlx.Tx, timerID uuid.UUID) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.GetContext(ctx, &id, queries.SQL.Get("TimerExecution"), timerID)
	return id, err
}

// LockDueTimer claims one due timer; ok is false if another worker has it or
// it already fired.
func LockDueTimer(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (Timer, bool, error) {
	var t Timer
	err := tx.GetContext(ctx, &t, commands.SQL.Get("LockDueTimer"), id)
	if errors.Is(err, sql.ErrNoRows) {
		return t, false, nil
	}
	return t, err == nil, err
}

// MarkTimerFired records that a timer was handled.
func MarkTimerFired(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("MarkTimerFired"), id)
	return err
}

// ConsumeTimers disarms every pending timer of an execution.
func ConsumeTimers(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("ConsumeTimers"), executionID)
	return err
}

// ---- human tasks -----------------------------------------------------------------

// NewTask is the insert shape of a human task.
type NewTask struct {
	ID, OrgID, WorkspaceID, WorkflowID, ExecutionID, StepID uuid.UUID
	NodeID                                                  string
	Task                                                    model.HumanTask
	Context                                                 []byte
	RequestedBy                                             *uuid.UUID
	DueAt                                                   *time.Time
}

func uuidArray(ids []uuid.UUID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = id.String()
	}
	return out
}

// InsertTask creates the task a waiting step is blocked on.
func InsertTask(ctx context.Context, tx *sqlx.Tx, t NewTask) error {
	form, err := json.Marshal(t.Task.Fields)
	if err != nil {
		return err
	}
	if t.Task.Fields == nil {
		form = []byte("[]")
	}
	_, err = tx.ExecContext(ctx, commands.SQL.Get("InsertTask"), t.ID, t.OrgID, t.WorkspaceID, t.WorkflowID,
		t.ExecutionID, t.StepID, t.NodeID, t.Task.Kind, t.Task.Title, t.Task.Instructions, nullJSON(t.Context), form,
		uuidArray(t.Task.Users), uuidArray(t.Task.Groups), t.Task.AllowSelfApproval, t.RequestedBy, t.DueAt)
	return err
}

type taskRow struct {
	ID                uuid.UUID     `db:"id"`
	WorkspaceID       uuid.UUID     `db:"workspace_id"`
	WorkflowID        uuid.UUID     `db:"workflow_id"`
	WorkflowName      string        `db:"workflow_name"`
	ExecutionID       uuid.UUID     `db:"execution_id"`
	StepID            uuid.UUID     `db:"step_id"`
	NodeID            string        `db:"node_id"`
	Kind              string        `db:"kind"`
	Title             string        `db:"title"`
	Instructions      string        `db:"instructions"`
	Context           []byte        `db:"context"`
	Form              []byte        `db:"form"`
	Users             db.TextArray  `db:"assignee_users"`
	Groups            db.TextArray  `db:"assignee_groups"`
	AllowSelfApproval bool          `db:"allow_self_approval"`
	RequestedBy       uuid.NullUUID `db:"requested_by"`
	Status            string        `db:"status"`
	Response          []byte        `db:"response"`
	Comment           string        `db:"comment"`
	DecidedBy         uuid.NullUUID `db:"decided_by"`
	DecidedAt         sql.NullTime  `db:"decided_at"`
	DueAt             sql.NullTime  `db:"due_at"`
	CreatedAt         time.Time     `db:"created_at"`
}

func parseIDs(in []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(in))
	for _, s := range in {
		if id, err := uuid.Parse(s); err == nil {
			out = append(out, id)
		}
	}
	return out
}

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	return &n.UUID
}

func (r taskRow) toModel() model.ApprovalTask {
	t := model.ApprovalTask{ID: r.ID, WorkspaceID: r.WorkspaceID, WorkflowID: r.WorkflowID,
		WorkflowName: r.WorkflowName, ExecutionID: r.ExecutionID, StepID: r.StepID, NodeID: r.NodeID, Kind: r.Kind,
		Title: r.Title, Instructions: r.Instructions, Context: r.Context, Users: parseIDs(r.Users),
		Groups: parseIDs(r.Groups), AllowSelfApproval: r.AllowSelfApproval, RequestedBy: nullUUID(r.RequestedBy),
		Status: model.TaskStatus(r.Status), Response: r.Response, Comment: r.Comment,
		DecidedBy: nullUUID(r.DecidedBy), DecidedAt: timePtr(r.DecidedAt), DueAt: timePtr(r.DueAt),
		CreatedAt: r.CreatedAt}
	_ = json.Unmarshal(r.Form, &t.Fields)
	return t
}

// TaskFilter narrows ListTasks; nil fields do not filter.
type TaskFilter struct {
	WorkspaceID *uuid.UUID
	Status      string
	Assignee    *uuid.UUID
	ExecutionID *uuid.UUID
	Limit       int
}

func optUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// ListTasks lists human tasks, newest first.
func ListTasks(ctx context.Context, tx *sqlx.Tx, f TaskFilter) ([]model.ApprovalTask, error) {
	var rows []taskRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListTasks"), optUUID(f.WorkspaceID), f.Status,
		optUUID(f.Assignee), optUUID(f.ExecutionID), f.Limit); err != nil {
		return nil, err
	}
	out := make([]model.ApprovalTask, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// GetTask reads one task (sql.ErrNoRows when absent).
func GetTask(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.ApprovalTask, error) {
	var r taskRow
	err := tx.GetContext(ctx, &r, queries.SQL.Get("GetTask"), id)
	return r.toModel(), err
}

// LockTask reads and locks one task for a decision.
func LockTask(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.ApprovalTask, error) {
	var r taskRow
	err := tx.GetContext(ctx, &r, commands.SQL.Get("LockTask"), id)
	return r.toModel(), err
}

// DecideTask records a decision on a pending task.
func DecideTask(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status model.TaskStatus, response any,
	comment string, by uuid.UUID) (bool, error) {
	raw, err := JSON(response)
	if err != nil {
		return false, err
	}
	return affected(tx.ExecContext(ctx, commands.SQL.Get("DecideTask"), id, string(status), nullJSON(raw), comment, by))
}

// ExpireTask expires the pending task of a step; ok is false if none was pending.
func ExpireTask(ctx context.Context, tx *sqlx.Tx, stepID uuid.UUID) (bool, error) {
	var id uuid.UUID
	err := tx.GetContext(ctx, &id, commands.SQL.Get("ExpireTask"), stepID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// CancelTasks cancels every pending task of an execution.
func CancelTasks(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("CancelTasks"), executionID)
	return err
}

// UserGroups returns the groups a user belongs to.
func UserGroups(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	err := tx.SelectContext(ctx, &out, queries.SQL.Get("UserGroups"), userID)
	return out, err
}

// ---- re-run ----------------------------------------------------------------------

// DeleteRerunSteps removes an execution's failed and cancelled steps and
// returns their node ids.
func DeleteRerunSteps(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) ([]string, error) {
	var out []string
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("DeleteRerunSteps"), executionID)
	return out, err
}

// ResetExecution reopens a failed or timed-out execution.
func ResetExecution(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, deadline time.Time, removedSteps int) (bool, error) {
	return affected(tx.ExecContext(ctx, commands.SQL.Get("ResetExecution"), id, deadline, removedSteps))
}

// CountActiveExecutions counts unfinished executions of a workflow in an environment.
func CountActiveExecutions(ctx context.Context, tx *sqlx.Tx, workflowID, environmentID uuid.UUID) (int, error) {
	var n int
	err := tx.GetContext(ctx, &n, queries.SQL.Get("CountActiveExecutions"), workflowID, environmentID)
	return n, err
}

func affected(res sql.Result, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}
