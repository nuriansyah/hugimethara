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

// NewExecution is the insert shape of an execution.
type NewExecution struct {
	ID, OrgID, WorkspaceID, WorkflowID, VersionID, EnvironmentID uuid.UUID
	TriggerType, TriggerRef, IdempotencyKey                      string
	Input                                                        json.RawMessage
	EgressAllowlist                                              []string
	MaxSteps                                                     int
	CreatedBy                                                    uuid.UUID
	RequestID                                                    string
	DeadlineAt                                                   time.Time
	TriggerDepth                                                 int
}

// InsertExecution creates an execution. created is false when an execution
// with the same idempotency key already exists; id is then the existing one.
func InsertExecution(ctx context.Context, tx *sqlx.Tx, e NewExecution) (id uuid.UUID, created bool, err error) {
	allow, err := json.Marshal(e.EgressAllowlist)
	if err != nil {
		return uuid.Nil, false, err
	}
	if e.EgressAllowlist == nil {
		allow = []byte("[]")
	}
	var key any
	if e.IdempotencyKey != "" {
		key = e.IdempotencyKey
	}
	input := []byte(e.Input)
	if len(input) == 0 {
		input = []byte("{}")
	}
	var createdBy any
	if e.CreatedBy != uuid.Nil {
		createdBy = e.CreatedBy
	}
	err = tx.GetContext(ctx, &id, commands.SQL.Get("InsertExecution"), e.ID, e.OrgID, e.WorkspaceID, e.WorkflowID,
		e.VersionID, e.EnvironmentID, e.TriggerType, e.TriggerRef, key, input, allow, e.MaxSteps, createdBy,
		e.RequestID, e.DeadlineAt, e.TriggerDepth)
	if errors.Is(err, sql.ErrNoRows) && e.IdempotencyKey != "" {
		err = tx.GetContext(ctx, &id, queries.SQL.Get("ExecutionByIdempotencyKey"), e.WorkflowID, e.IdempotencyKey)
		return id, false, err
	}
	return id, err == nil, err
}

// NewStep is the insert shape of a step created by progression.
type NewStep struct {
	ID, OrgID, ExecutionID uuid.UUID
	NodeID, NodeType       string
	Status                 model.StepStatus // queued or skipped
	Input                  []byte
}

// InsertStep creates a step; inserted is false if the node already has one
// (concurrent progression), which callers treat as success.
func InsertStep(ctx context.Context, tx *sqlx.Tx, s NewStep) (inserted bool, err error) {
	var id uuid.UUID
	err = tx.GetContext(ctx, &id, commands.SQL.Get("InsertStep"), s.ID, s.OrgID, s.ExecutionID, s.NodeID,
		s.NodeType, string(s.Status), nullJSON(s.Input))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

// AddStepCount increments an execution's step counter.
func AddStepCount(ctx context.Context, tx *sqlx.Tx, executionID uuid.UUID, n int) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("AddStepCount"), executionID, n)
	return err
}

// ListExecutions returns the execution history, newest first.
func ListExecutions(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID, workflowID *uuid.UUID,
	status string, limit int) ([]model.Execution, error) {
	var rows []executionRow
	var wf any
	if workflowID != nil {
		wf = *workflowID
	}
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListExecutions"), workspaceID, wf, status, limit); err != nil {
		return nil, err
	}
	out := make([]model.Execution, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// GetExecution returns an execution with all its steps.
func GetExecution(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Execution, error) {
	var row executionRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetExecution"), id); err != nil {
		return model.Execution{}, err
	}
	e := row.toModel()
	var steps []stepRow
	if err := tx.SelectContext(ctx, &steps, queries.SQL.Get("ListSteps"), id); err != nil {
		return model.Execution{}, err
	}
	e.Steps = make([]model.Step, len(steps))
	for i, s := range steps {
		e.Steps[i] = s.toModel()
	}
	return e, nil
}

func nullJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	return b
}

// ExecutionByKey returns the execution started with an idempotency key.
func ExecutionByKey(ctx context.Context, tx *sqlx.Tx, workflowID uuid.UUID, key string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.GetContext(ctx, &id, queries.SQL.Get("ExecutionByIdempotencyKey"), workflowID, key)
	return id, err
}

// LockStarts serializes execution starts of one workflow in one environment
// for the rest of the transaction (advisory lock; any role may take it).
func LockStarts(ctx context.Context, tx *sqlx.Tx, workflowID, environmentID uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("LockStarts"), workflowID.String()+"/"+environmentID.String())
	return err
}
