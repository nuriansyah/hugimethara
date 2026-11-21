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

// DueSchedules lists schedule triggers whose next fire time has passed.
func DueSchedules(ctx context.Context, q sqlx.QueryerContext, limit int) ([]StepRef, error) {
	var out []StepRef
	err := sqlx.SelectContext(ctx, q, &out, queries.SQL.Get("DueSchedules"), limit)
	return out, err
}

// DueSchedule is a schedule trigger claimed for firing.
type DueSchedule struct {
	ID             uuid.UUID `db:"id"`
	OrgID          uuid.UUID `db:"org_id"`
	WorkflowID     uuid.UUID `db:"workflow_id"`
	EnvironmentID  uuid.UUID `db:"environment_id"`
	Config         []byte    `db:"config"`
	NextFireAt     time.Time `db:"next_fire_at"`
	Enabled        bool      `db:"enabled"`
	VersionID      uuid.UUID `db:"version_id"`
	WorkflowStatus string    `db:"workflow_status"`
	WorkspaceID    uuid.UUID `db:"workspace_id"`
}

// LockDueSchedule claims one due schedule trigger; ok is false when another
// worker holds it or it is no longer due.
func LockDueSchedule(ctx context.Context, tx *sqlx.Tx, id uuid.UUID) (DueSchedule, bool, error) {
	var s DueSchedule
	err := tx.GetContext(ctx, &s, commands.SQL.Get("LockDueSchedule"), id)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}

// SetNextFire stores the next activation (nil stops the schedule).
func SetNextFire(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, next *time.Time) error {
	_, err := tx.ExecContext(ctx, commands.SQL.Get("SetNextFire"), id, next)
	return err
}

// EventTrigger is an enabled event trigger of an active workflow.
type EventTrigger struct {
	ID            uuid.UUID `db:"id"`
	OrgID         uuid.UUID `db:"org_id"`
	WorkflowID    uuid.UUID `db:"workflow_id"`
	EnvironmentID uuid.UUID `db:"environment_id"`
	Config        []byte    `db:"config"`
	VersionID     uuid.UUID `db:"version_id"`
	WorkspaceID   uuid.UUID `db:"workspace_id"`
}

// EventTriggers lists the triggers listening to event in an organization.
func EventTriggers(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID, event string) ([]EventTrigger, error) {
	var out []EventTrigger
	err := tx.SelectContext(ctx, &out, queries.SQL.Get("EventTriggers"), orgID, event)
	return out, err
}

// TriggerConfig decodes a stored trigger configuration.
func TriggerConfig(raw []byte) map[string]any {
	var cfg map[string]any
	_ = json.Unmarshal(raw, &cfg)
	return cfg
}

type scheduleRow struct {
	WorkflowID      uuid.UUID    `db:"workflow_id"`
	WorkflowName    string       `db:"workflow_name"`
	WorkflowStatus  string       `db:"workflow_status"`
	EnvironmentID   uuid.UUID    `db:"environment_id"`
	EnvironmentName string       `db:"environment_name"`
	Version         int          `db:"version"`
	Enabled         bool         `db:"enabled"`
	Cron            string       `db:"cron"`
	Timezone        string       `db:"timezone"`
	NextFireAt      sql.NullTime `db:"next_fire_at"`
}

// ListSchedules lists a workspace's deployed schedule triggers.
func ListSchedules(ctx context.Context, tx *sqlx.Tx, workspaceID uuid.UUID) ([]model.Schedule, error) {
	var rows []scheduleRow
	if err := tx.SelectContext(ctx, &rows, queries.SQL.Get("ListSchedules"), workspaceID); err != nil {
		return nil, err
	}
	out := make([]model.Schedule, len(rows))
	for i, r := range rows {
		out[i] = model.Schedule{WorkflowID: r.WorkflowID, WorkflowName: r.WorkflowName,
			WorkflowStatus: r.WorkflowStatus, EnvironmentID: r.EnvironmentID, EnvironmentName: r.EnvironmentName,
			Version: r.Version, Enabled: r.Enabled, Cron: r.Cron, Timezone: r.Timezone, NextFireAt: timePtr(r.NextFireAt)}
	}
	return out, nil
}

// WebhookTriggers lists the deployed triggers bound to a webhook endpoint.
func WebhookTriggers(ctx context.Context, tx *sqlx.Tx, orgID uuid.UUID, endpointID string, environmentID,
	workspaceID uuid.UUID) ([]EventTrigger, error) {
	var out []EventTrigger
	err := tx.SelectContext(ctx, &out, queries.SQL.Get("WebhookTriggers"), orgID, endpointID, environmentID, workspaceID)
	return out, err
}

// EndpointScope is where a webhook endpoint lives.
type EndpointScope struct {
	WorkspaceID   uuid.UUID `db:"workspace_id"`
	EnvironmentID uuid.UUID `db:"environment_id"`
}

// WebhookEndpointScope returns a webhook endpoint's workspace and environment
// (sql.ErrNoRows when it does not exist in the tenant).
func WebhookEndpointScope(ctx context.Context, tx *sqlx.Tx, endpointID uuid.UUID) (EndpointScope, error) {
	var s EndpointScope
	err := tx.GetContext(ctx, &s, queries.SQL.Get("WebhookEndpointScope"), endpointID)
	return s, err
}

// ConnectorScope is the definition key and declared operations of the
// connector instance an action.connector node references.
type ConnectorScope struct {
	Key        string `db:"key"`
	Operations []byte `db:"operations"`
}

// ConnectorInstanceScope looks up the instance with refKey in a workspace and
// environment.
func ConnectorInstanceScope(ctx context.Context, tx *sqlx.Tx, workspaceID, environmentID uuid.UUID, refKey string) (ConnectorScope, error) {
	var s ConnectorScope
	err := tx.GetContext(ctx, &s, queries.SQL.Get("ConnectorInstanceScope"), workspaceID, environmentID, refKey)
	return s, err
}
