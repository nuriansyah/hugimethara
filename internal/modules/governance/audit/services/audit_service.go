package services

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/repository/queries"
	entmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/pagination"
)

// AuditService lists audit events for authorized principals.
type AuditService struct {
	pool *sqlx.DB
	az   authzsvc.Authorizer
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer) *AuditService {
	return &AuditService{pool: pool, az: az}
}

// List returns one page of the caller's organization audit trail, newest first.
func (s *AuditService) List(ctx context.Context, f dto.Filter, page pagination.Page) ([]model.Entry, string, error) {
	p, ok := model.PrincipalFrom(ctx)
	if !ok {
		return nil, "", authzmodel.Unauthenticated()
	}
	if _, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: model.ResourceType, OrgID: p}); err != nil {
		return nil, "", err
	}
	limit := min(max(page.Limit, 1), pagination.MaxPageLimit)
	var curT, curID any
	if page.Cursor != "" {
		var c model.Cursor
		if err := pagination.DecodeCursor(page.Cursor, &c); err != nil {
			return nil, "", err
		}
		curT, curID = c.T, c.ID
	}

	var rows []repository.EventRow
	err := db.InTenantTx(ctx, s.pool, p, func(tx *sqlx.Tx) error {
		// The plan decides how far back the log can be viewed.
		ent, err := entsvc.Load(ctx, tx, p)
		if err != nil {
			return err
		}
		if days, limited := ent.Limit(entmodel.AuditLogRetentionDays); limited {
			floor := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
			if f.Since.Before(floor) {
				f.Since = floor
			}
		}
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListAuditEvents"),
			model.NullStr(f.Action), model.NullUUID(f.ActorID), model.NullStr(f.ResourceType), model.NullTime(f.Since), model.NullTime(f.Until),
			curT, curID, limit+1)
	})
	if err != nil {
		return nil, "", err
	}

	var next string
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next, _ = pagination.EncodeCursor(model.Cursor{T: last.OccurredAt, ID: last.ID})
	}
	out := make([]model.Entry, len(rows))
	for i, r := range rows {
		e := model.Entry{
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
