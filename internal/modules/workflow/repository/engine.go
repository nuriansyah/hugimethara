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
)

// LockedExecution is the execution state the engine needs, read under
// SELECT … FOR UPDATE.
type LockedExecution struct {
	ID              uuid.UUID     `db:"id"`
	OrgID           uuid.UUID     `db:"org_id"`
	WorkspaceID     uuid.UUID     `db:"workspace_id"`
	WorkflowID      uuid.UUID     `db:"workflow_id"`
	VersionID       uuid.UUID     `db:"version_id"`
	EnvironmentID   uuid.UUID     `db:"environment_id"`
	Status          string        `db:"status"`
	Input           []byte        `db:"input"`
	EgressAllowlist []byte        `db:"egress_allowlist"`
	StepCount       int           `db:"step_count"`
	MaxSteps        int           `db:"max_steps"`
	DeadlineAt      time.Time     `db:"deadline_at"`
	CreatedBy       uuid.NullUUID `db:"created_by"`
	TriggerDepth    int           `db:"trigger_depth"`
}

// Allowlist decodes the egress allowlist snapshot.
func (e LockedExecution) Allowlist() []string {
	var out []string
	_ = json.Unmarshal(e.EgressAllowlist, &out)
	return out
}

// LockExecution reads and locks an execution row.
func LockExecution(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (LockedExecution, error) {
	var e LockedExecution
	err := tx.GetContext(ctx, &e, commands.SQL.Get("LockExecution"), id)
	return e, err
}

// StartExecution moves a pending execution to running (no-op otherwise).
func StartExecution(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("StartExecution"), id)
	return err
}

// FinishExecution records the terminal status, output and error.
func FinishExecution(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, status model.ExecutionStatus, output any,
	stepErr *model.StepError) error {
	out, err := JSON(output)
	if err != nil {
		return err
	}
	var errJSON []byte
	if stepErr != nil {
		if errJSON, err = json.Marshal(stepErr); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, commands.SQL.Get("FinishExecution"), id, string(status), nullJSON(out), nullJSON(errJSON))
	return err
}

// ClaimedStep is a step this worker now holds a lease on.
type ClaimedStep struct {
	ID          uuid.UUID `db:"id"`
	OrgID       uuid.UUID `db:"org_id"`
	ExecutionID uuid.UUID `db:"execution_id"`
	NodeID      string    `db:"node_id"`
	NodeType    string    `db:"node_type"`
	Attempt     int       `db:"attempt"`
}

// ClaimStep leases a queued step. ok is false when another worker has it or
// it is no longer queued (duplicate delivery).
func ClaimStep(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, lease time.Duration, worker string) (ClaimedStep, bool, error) {
	var c ClaimedStep
	err := tx.GetContext(ctx, &c, commands.SQL.Get("ClaimStep"), id, lease.Seconds(), worker)
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	return c, err == nil, err
}

// ExtendLease heartbeats a running step; false means the lease was lost.
func ExtendLease(ctx context.Context, db sqlx.ExecerContext, id uuid.UUID, lease time.Duration, worker string) (bool, error) {
	res, err := db.ExecContext(ctx, commands.SQL.Get("ExtendLease"), id, lease.Seconds(), worker)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// StepResult is the outcome written by CompleteStep.
type StepResult struct {
	Status model.StepStatus
	Input  []byte
	Output []byte
	Port   string
	Error  *model.StepError
}

// CompleteStep records a terminal result. ok is false if this worker no
// longer holds the lease (the result is then discarded).
func CompleteStep(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, worker string, r StepResult) (bool, error) {
	errJSON, err := marshalErr(r.Error)
	if err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, commands.SQL.Get("CompleteStep"), id, string(r.Status), nullJSON(r.Input),
		nullJSON(r.Output), r.Port, nullJSON(errJSON), worker)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// ScheduleRetry parks a failed step until at.
func ScheduleRetry(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, worker string, input []byte,
	stepErr *model.StepError, at time.Time) (bool, error) {
	errJSON, err := marshalErr(stepErr)
	if err != nil {
		return false, err
	}
	res, err := tx.ExecContext(ctx, commands.SQL.Get("ScheduleRetry"), id, nullJSON(errJSON), at, worker, nullJSON(input))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// CancelOpenSteps cancels every non-terminal step of an execution.
func CancelOpenSteps(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("CancelOpenSteps"), executionID)
	return err
}

// StepState is one step's state for progression.
type StepState struct {
	NodeID string `db:"node_id"`
	Status string `db:"status"`
	Port   string `db:"port"`
	Output []byte `db:"output"`
}

// StepStates reads every step of an execution.
func StepStates(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID) ([]StepState, error) {
	var out []StepState
	err := tx.SelectContext(ctx, &out, queries.SQL.Get("StepStates"), executionID)
	return out, err
}

// EngineDefinition loads a pinned version's definition.
func EngineDefinition(ctx context.Context, tx *sqlx.Tx, versionID uuid.UUID) (model.Definition, error) {
	var raw []byte
	if err := tx.GetContext(ctx, &raw, queries.SQL.Get("EngineVersion"), versionID); err != nil {
		return model.Definition{}, err
	}
	return model.ParseDefinition(raw)
}

// StepRef identifies a step to (re)dispatch.
type StepRef struct {
	ID    uuid.UUID `db:"id"`
	OrgID uuid.UUID `db:"org_id"`
}

// RequeueExpiredLeases returns running steps whose lease expired to the queue.
func RequeueExpiredLeases(ctx context.Context, tx *sqlx.Tx, limit int) ([]StepRef, error) {
	var out []StepRef
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("RequeueExpiredLeases"), limit)
	return out, err
}

// RequeueDueRetries queues retry_scheduled steps whose time has come.
func RequeueDueRetries(ctx context.Context, tx *sqlx.Tx, limit int) ([]StepRef, error) {
	var out []StepRef
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("RequeueDueRetries"), limit)
	return out, err
}

// RedispatchStale returns queued steps not dispatched for longer than after.
func RedispatchStale(ctx context.Context, tx *sqlx.Tx, limit int, after time.Duration) ([]StepRef, error) {
	var out []StepRef
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("RedispatchStale"), limit, after.Seconds())
	return out, err
}

// ExpiredExecution identifies an execution the reaper timed out.
type ExpiredExecution struct {
	ID           uuid.UUID `db:"id"`
	OrgID        uuid.UUID `db:"org_id"`
	WorkflowID   uuid.UUID `db:"workflow_id"`
	WorkspaceID  uuid.UUID `db:"workspace_id"`
	TriggerDepth int       `db:"trigger_depth"`
}

// ExpireDeadlines times out executions past their deadline.
func ExpireDeadlines(ctx context.Context, tx *sqlx.Tx, limit int) ([]ExpiredExecution, error) {
	var out []ExpiredExecution
	err := tx.SelectContext(ctx, &out, commands.SQL.Get("ExpireDeadlines"), limit)
	return out, err
}

func marshalErr(e *model.StepError) ([]byte, error) {
	if e == nil {
		return nil, nil
	}
	return json.Marshal(e)
}
