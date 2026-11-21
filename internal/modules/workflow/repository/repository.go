// Package repository is the workflow persistence adapter. Unlike the
// statement-registry modules, workflow exposes typed functions per operation
// (ADR-0014): services never see SQL, and every write lives in commands/,
// every read in queries/.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/workflow/model"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

// ---- rows -------------------------------------------------------------------

type workflowRow struct {
	ID            uuid.UUID     `db:"id"`
	WorkspaceID   uuid.UUID     `db:"workspace_id"`
	Name          string        `db:"name"`
	Slug          string        `db:"slug"`
	Description   string        `db:"description"`
	Draft         []byte        `db:"draft"`
	Status        string        `db:"status"`
	CreatedAt     time.Time     `db:"created_at"`
	UpdatedAt     time.Time     `db:"updated_at"`
	LatestVersion sql.NullInt32 `db:"latest_version"`
}

func (r workflowRow) toModel() (model.Workflow, error) {
	w := model.Workflow{ID: r.ID, WorkspaceID: r.WorkspaceID, Name: r.Name, Slug: r.Slug,
		Description: r.Description, Status: r.Status, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	if r.LatestVersion.Valid {
		v := int(r.LatestVersion.Int32)
		w.LatestVersion = &v
	}
	d, err := model.ParseDefinition(r.Draft)
	w.Draft = d
	return w, err
}

type versionRow struct {
	ID          uuid.UUID `db:"id"`
	WorkflowID  uuid.UUID `db:"workflow_id"`
	Version     int       `db:"version"`
	Definition  []byte    `db:"definition"`
	Checksum    string    `db:"checksum"`
	PublishedBy uuid.UUID `db:"published_by"`
	PublishedAt time.Time `db:"published_at"`
}

func (r versionRow) toModel() (model.Version, error) {
	d, err := model.ParseDefinition(r.Definition)
	return model.Version{ID: r.ID, WorkflowID: r.WorkflowID, Version: r.Version, Definition: d,
		Checksum: r.Checksum, PublishedBy: r.PublishedBy, PublishedAt: r.PublishedAt}, err
}

type deploymentRow struct {
	WorkflowID    uuid.UUID `db:"workflow_id"`
	EnvironmentID uuid.UUID `db:"environment_id"`
	VersionID     uuid.UUID `db:"version_id"`
	Version       int       `db:"version"`
	Enabled       bool      `db:"enabled"`
	UpdatedAt     time.Time `db:"updated_at"`
}

func (r deploymentRow) toModel() model.Deployment {
	return model.Deployment(r)
}

type executionRow struct {
	ID            uuid.UUID    `db:"id"`
	WorkspaceID   uuid.UUID    `db:"workspace_id"`
	WorkflowID    uuid.UUID    `db:"workflow_id"`
	VersionID     uuid.UUID    `db:"version_id"`
	Version       int          `db:"version"`
	EnvironmentID uuid.UUID    `db:"environment_id"`
	TriggerType   string       `db:"trigger_type"`
	Status        string       `db:"status"`
	Input         []byte       `db:"input"`
	Output        []byte       `db:"output"`
	Error         []byte       `db:"error"`
	StepCount     int          `db:"step_count"`
	CreatedAt     time.Time    `db:"created_at"`
	StartedAt     sql.NullTime `db:"started_at"`
	FinishedAt    sql.NullTime `db:"finished_at"`
	DeadlineAt    time.Time    `db:"deadline_at"`
}

func (r executionRow) toModel() model.Execution {
	return model.Execution{ID: r.ID, WorkspaceID: r.WorkspaceID, WorkflowID: r.WorkflowID,
		VersionID: r.VersionID, Version: r.Version, EnvironmentID: r.EnvironmentID, TriggerType: r.TriggerType,
		Status: model.ExecutionStatus(r.Status), Input: r.Input, Output: r.Output, Error: stepError(r.Error),
		StepCount: r.StepCount, CreatedAt: r.CreatedAt, StartedAt: timePtr(r.StartedAt),
		FinishedAt: timePtr(r.FinishedAt), DeadlineAt: r.DeadlineAt}
}

type stepRow struct {
	ID         uuid.UUID    `db:"id"`
	NodeID     string       `db:"node_id"`
	NodeType   string       `db:"node_type"`
	Attempt    int          `db:"attempt"`
	Status     string       `db:"status"`
	Input      []byte       `db:"input"`
	Output     []byte       `db:"output"`
	Port       string       `db:"port"`
	Error      []byte       `db:"error"`
	QueuedAt   sql.NullTime `db:"queued_at"`
	StartedAt  sql.NullTime `db:"started_at"`
	FinishedAt sql.NullTime `db:"finished_at"`
}

func (r stepRow) toModel() model.Step {
	return model.Step{ID: r.ID, NodeID: r.NodeID, NodeType: r.NodeType, Attempt: r.Attempt,
		Status: model.StepStatus(r.Status), Input: r.Input, Output: r.Output, Port: r.Port,
		Error: stepError(r.Error), QueuedAt: timePtr(r.QueuedAt), StartedAt: timePtr(r.StartedAt),
		FinishedAt: timePtr(r.FinishedAt)}
}

func stepError(raw []byte) *model.StepError {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var e model.StepError
	if json.Unmarshal(raw, &e) != nil {
		return &model.StepError{Code: "unknown", Message: string(raw)}
	}
	return &e
}

func timePtr(t sql.NullTime) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// JSON marshals v for a jsonb parameter; nil stays SQL NULL.
func JSON(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	if raw, ok := v.(json.RawMessage); ok {
		if len(raw) == 0 {
			return nil, nil
		}
		return raw, nil
	}
	return json.Marshal(v)
}

// ---- workflows ----------------------------------------------------------------

// NewWorkflow is the insert shape of a workflow.
type NewWorkflow struct {
	ID, OrgID, WorkspaceID uuid.UUID
	Name, Slug, Desc       string
	Draft                  model.Definition
	CreatedBy              uuid.UUID
}

// InsertWorkflow creates a workflow.
func InsertWorkflow(ctx context.Context, tx *sqlx.Tx, w NewWorkflow) (model.Workflow, error) {
	draft, err := json.Marshal(w.Draft)
	if err != nil {
		return model.Workflow{}, err
	}
	var row workflowRow
	if err := tx.GetContext(ctx, &row, commands.SQL.Get("InsertWorkflow"), w.ID, w.OrgID, w.WorkspaceID,
		w.Name, w.Slug, w.Desc, draft, w.CreatedBy); err != nil {
		return model.Workflow{}, err
	}
	return row.toModel()
}

// UpdateWorkflow overwrites the mutable fields of w.
func UpdateWorkflow(ctx context.Context, tx *sqlx.Tx, w model.Workflow) (model.Workflow, error) {
	draft, err := json.Marshal(w.Draft)
	if err != nil {
		return model.Workflow{}, err
	}
	var row workflowRow
	if err := tx.GetContext(ctx, &row, commands.SQL.Get("UpdateWorkflow"), w.ID, w.Name, w.Description,
		draft, w.Status); err != nil {
		return model.Workflow{}, err
	}
	return row.toModel()
}

// DeleteWorkflow removes a workflow and (by cascade) its history.
func DeleteWorkflow(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (int64, error) {
	res, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteWorkflow"), id)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// LockWorkflow takes the workflow row lock (serializes publishing).
func LockWorkflow(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	var got uuid.UUID
	return tx.GetContext(ctx, &got, commands.SQL.Get("LockWorkflow"), id)
}

// ListWorkflows lists a workspace's workflows.
func ListWorkflows(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID) ([]model.Workflow, error) {
	var rows []workflowRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListWorkflows"), workspaceID); err != nil {
		return nil, err
	}
	out := make([]model.Workflow, len(rows))
	for i, r := range rows {
		w, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out[i] = w
	}
	return out, nil
}

// GetWorkflow returns one workflow (sql.ErrNoRows when absent).
func GetWorkflow(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Workflow, error) {
	var row workflowRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetWorkflow"), id); err != nil {
		return model.Workflow{}, err
	}
	return row.toModel()
}

// ---- versions & deployments -----------------------------------------------------

// InsertVersion stores an immutable published definition.
func InsertVersion(ctx context.Context, tx *sqlx.Tx, id, orgID, workflowID uuid.UUID, version int,
	definition []byte, checksum string, publishedBy uuid.UUID) (model.Version, error) {
	var row versionRow
	if err := tx.GetContext(ctx, &row, commands.SQL.Get("InsertVersion"), id, orgID, workflowID, version,
		definition, checksum, publishedBy); err != nil {
		return model.Version{}, err
	}
	return row.toModel()
}

// ActivateWorkflow marks a draft workflow active after its first publish.
func ActivateWorkflow(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("ActivateWorkflow"), id)
	return err
}

// LatestVersion returns the newest version (sql.ErrNoRows when unpublished).
func LatestVersion(ctx context.Context, tx *sqlx.Tx, workflowID uuid.UUID) (model.Version, error) {
	var row versionRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("LatestVersion"), workflowID); err != nil {
		return model.Version{}, err
	}
	return row.toModel()
}

// GetVersion returns a version by id.
func GetVersion(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (model.Version, error) {
	var row versionRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetVersion"), id); err != nil {
		return model.Version{}, err
	}
	return row.toModel()
}

// ListVersions lists versions newest first.
func ListVersions(ctx context.Context, tx *sqlx.Tx, workflowID uuid.UUID) ([]model.Version, error) {
	var rows []versionRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListVersions"), workflowID); err != nil {
		return nil, err
	}
	out := make([]model.Version, len(rows))
	for i, r := range rows {
		v, err := r.toModel()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

// UpsertDeployment pins a version to an environment and records its trigger.
func UpsertDeployment(ctx context.Context, tx *sqlx.Tx, orgID, workflowID, environmentID, versionID uuid.UUID,
	enabled bool, by uuid.UUID, trigger model.Trigger) error {
	if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpsertDeployment"), orgID, workflowID, environmentID,
		versionID, enabled, by); err != nil {
		return err
	}
	cfg, err := json.Marshal(trigger.Config)
	if err != nil {
		return err
	}
	if trigger.Config == nil {
		cfg = []byte("{}")
	}
	_, err = tx.ExecContext(ctx, commands.SQL.Get("UpsertTrigger"), orgID, workflowID, environmentID,
		trigger.Type, cfg)
	return err
}

// ListDeployments lists a workflow's deployments.
func ListDeployments(ctx context.Context, tx *sqlx.Tx, workflowID uuid.UUID) ([]model.Deployment, error) {
	var rows []deploymentRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListDeployments"), workflowID); err != nil {
		return nil, err
	}
	out := make([]model.Deployment, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

// GetDeployment returns the deployment in one environment.
func GetDeployment(ctx context.Context, tx *sqlx.Tx, workflowID, environmentID uuid.UUID) (model.Deployment, error) {
	var row deploymentRow
	if err := tx.GetContext(ctx, &row, queries.SQL.Get("GetDeployment"), workflowID, environmentID); err != nil {
		return model.Deployment{}, err
	}
	return row.toModel(), nil
}

// EnvironmentAllowlist returns an environment's egress allowlist entries.
func EnvironmentAllowlist(ctx context.Context, tx *sqlx.Tx, environmentID uuid.UUID) ([]string, error) {
	var out db.TextArray
	if err := tx.GetContext(ctx, &out, queries.SQL.Get("EnvironmentAllowlist"), environmentID); err != nil {
		return nil, err
	}
	return out, nil
}
