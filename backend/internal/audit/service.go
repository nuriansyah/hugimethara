package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nuriansyah/hugimethara/backend/internal/audit/auditdb"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// ResourceType is the authz resource type for audit events.
const ResourceType = "audit_event"

// Filter narrows a listing. Zero values mean "no filter".
type Filter struct {
	Action       string // exact action or prefix ("organization" matches "organization.*")
	ActorID      uuid.UUID
	ResourceType string
	Since, Until time.Time
}

// Entry is one listed event.
type Entry struct {
	ID           uuid.UUID
	OccurredAt   time.Time
	ActorType    string
	ActorID      *uuid.UUID
	ActorEmail   string
	Action       string
	ResourceType string
	ResourceID   string
	WorkspaceID  *uuid.UUID
	Outcome      string
	RequestID    string
	IP           string
	Details      map[string]any
}

type cursor struct {
	T  time.Time `json:"t"`
	ID uuid.UUID `json:"id"`
}

// Service lists audit events for authorized principals.
type Service struct {
	pool *pgxpool.Pool
	az   authz.Authorizer
}

func NewService(pool *pgxpool.Pool, az authz.Authorizer) *Service {
	return &Service{pool: pool, az: az}
}

// List returns one page of the caller's organization audit trail, newest first.
func (s *Service) List(ctx context.Context, f Filter, page httpx.Page) ([]Entry, string, error) {
	p, ok := principalFrom(ctx)
	if !ok {
		return nil, "", authz.Unauthenticated()
	}
	if _, err := authz.Require(ctx, s.az, "read", authz.Resource{Type: ResourceType, OrgID: p}); err != nil {
		return nil, "", err
	}
	limit := min(max(page.Limit, 1), httpx.MaxPageLimit)
	params := auditdb.ListAuditEventsParams{PageLimit: int32(limit + 1)} //nolint:gosec // G115: bounded to 201
	if f.Action != "" {
		params.Action = pgtype.Text{String: f.Action, Valid: true}
	}
	if f.ActorID != uuid.Nil {
		params.ActorID = uuid.NullUUID{UUID: f.ActorID, Valid: true}
	}
	if f.ResourceType != "" {
		params.ResourceType = pgtype.Text{String: f.ResourceType, Valid: true}
	}
	if !f.Since.IsZero() {
		params.Since = pgtype.Timestamptz{Time: f.Since, Valid: true}
	}
	if !f.Until.IsZero() {
		params.Until = pgtype.Timestamptz{Time: f.Until, Valid: true}
	}
	if page.Cursor != "" {
		var c cursor
		if err := httpx.DecodeCursor(page.Cursor, &c); err != nil {
			return nil, "", err
		}
		params.CursorTime = pgtype.Timestamptz{Time: c.T, Valid: true}
		params.CursorID = uuid.NullUUID{UUID: c.ID, Valid: true}
	}

	var rows []auditdb.ListAuditEventsRow
	err := db.InTenantTx(ctx, s.pool, p, func(tx pgx.Tx) error {
		var err error
		rows, err = auditdb.New(tx).ListAuditEvents(ctx, params)
		return err
	})
	if err != nil {
		return nil, "", err
	}

	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next, _ = httpx.EncodeCursor(cursor{T: last.OccurredAt.Time, ID: last.ID})
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		e := Entry{
			ID: r.ID, OccurredAt: r.OccurredAt.Time, ActorType: r.ActorType, ActorEmail: r.ActorEmail.String,
			Action: r.Action, ResourceType: r.ResourceType, ResourceID: r.ResourceID.String,
			Outcome: r.Outcome, RequestID: r.RequestID.String,
		}
		if r.ActorID.Valid {
			id := r.ActorID.UUID
			e.ActorID = &id
		}
		if r.WorkspaceID.Valid {
			id := r.WorkspaceID.UUID
			e.WorkspaceID = &id
		}
		if r.Ip != nil {
			e.IP = r.Ip.String()
		}
		_ = json.Unmarshal(r.Details, &e.Details)
		out[i] = e
	}
	return out, next, nil
}
