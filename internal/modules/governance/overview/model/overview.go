package model

import (
	"time"

	"github.com/google/uuid"
)

// Window is the KPI time window.
type Window string

const (
	Window24h Window = "24h"
	Window7d  Window = "7d"
	Window30d Window = "30d"
)

// Duration returns the window length and the series bucket ("hour" or "day").
func (w Window) Duration() (time.Duration, string, bool) {
	switch w {
	case Window24h:
		return 24 * time.Hour, "hour", true
	case Window7d:
		return 7 * 24 * time.Hour, "day", true
	case Window30d:
		return 30 * 24 * time.Hour, "day", true
	}
	return 0, "", false
}

// Overview is the KPI snapshot of one workspace. A nil section means the
// caller may not read that resource type in the workspace.
type Overview struct {
	WorkspaceID uuid.UUID
	Window      Window
	From        time.Time
	To          time.Time
	Workflows   *WorkflowKPIs
	Connectors  *ConnectorKPIs
	Webhooks    *WebhookKPIs
}

type WorkflowKPIs struct {
	Total            int               `db:"total"`
	Succeeded        int               `db:"succeeded"`
	Failed           int               `db:"failed"` // failed + timed_out
	Cancelled        int               `db:"cancelled"`
	Running          int               `db:"running"` // pending + running now (not windowed)
	P50Ms            *int              `db:"p50_ms"`
	P95Ms            *int              `db:"p95_ms"`
	PendingApprovals int               `db:"pending_approvals"`
	ActiveWorkflows  int               `db:"active_workflows"`
	Series           []SeriesPoint     `db:"-"`
	TopFailing       []FailingWorkflow `db:"-"`
}

type SeriesPoint struct {
	Bucket    time.Time `db:"bucket"`
	Succeeded int       `db:"succeeded"`
	Failed    int       `db:"failed"`
}

type FailingWorkflow struct {
	WorkflowID    uuid.UUID `db:"workflow_id"`
	Name          string    `db:"name"`
	Failures      int       `db:"failures"`
	LastErrorCode string    `db:"last_error_code"`
	LastFailedAt  time.Time `db:"last_failed_at"`
}

type ConnectorKPIs struct {
	Total     int                  `db:"total"`
	Enabled   int                  `db:"enabled"`
	Healthy   int                  `db:"healthy"`
	Unhealthy int                  `db:"unhealthy"`
	Unchecked int                  `db:"unchecked"`
	Failing   []UnhealthyConnector `db:"-"`
}

type UnhealthyConnector struct {
	ConnectorID   uuid.UUID `db:"id"`
	RefKey        string    `db:"ref_key"`
	DefinitionKey string    `db:"definition_key"`
	EnvironmentID uuid.UUID `db:"environment_id"`
	ErrorCode     string    `db:"error_code"`
	CheckedAt     time.Time `db:"checked_at"`
}

type WebhookKPIs struct {
	Endpoints             int `db:"endpoints"`
	Received              int `db:"received"`
	RejectedTotal         int `db:"rejected_total"` // cumulative on the endpoints
	Subscriptions         int `db:"subscriptions"`
	DeliveriesSucceeded   int `db:"deliveries_succeeded"`
	DeliveriesPending     int `db:"deliveries_pending"`
	DeliveriesDeadLetters int `db:"deliveries_dead_lettered"`
}
