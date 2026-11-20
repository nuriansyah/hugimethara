package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
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
	pool *sqlx.DB
	az   authz.Authorizer
}

func NewService(pool *sqlx.DB, az authz.Authorizer) *Service {
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
	var curT, curID any
	if page.Cursor != "" {
		var c cursor
		if err := httpx.DecodeCursor(page.Cursor, &c); err != nil {
			return nil, "", err
		}
		curT, curID = c.T, c.ID
	}

	var rows []eventRow
	err := db.InTenantTx(ctx, s.pool, p, func(tx *sqlx.Tx) error {
		// The plan decides how far back the log can be viewed.
		ent, err := entitlement.Load(ctx, tx, p)
		if err != nil {
			return err
		}
		if days, limited := ent.Limit(entitlement.AuditLogRetentionDays); limited {
			floor := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			if f.Since.Before(floor) {
				f.Since = floor
			}
		}
		return tx.SelectContext(ctx, &rows, q.Get("ListAuditEvents"),
			nullStr(f.Action), nullUUID(f.ActorID), nullStr(f.ResourceType), nullTime(f.Since), nullTime(f.Until),
			curT, curID, limit+1)
	})
	if err != nil {
		return nil, "", err
	}

	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next, _ = httpx.EncodeCursor(cursor{T: last.OccurredAt, ID: last.ID})
	}
	out := make([]Entry, len(rows))
	for i, r := range rows {
		e := Entry{
			ID: r.ID, OccurredAt: r.OccurredAt, ActorType: r.ActorType, ActorEmail: r.ActorEmail.String,
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
		e.IP = r.IP.String
		_ = json.Unmarshal(r.Details, &e.Details)
		out[i] = e
	}
	return out, next, nil
}
