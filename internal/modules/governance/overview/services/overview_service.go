// Package services implements the workspace overview use case.
package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzdto "github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/overview/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/overview/repository/queries"
	"github.com/nuriansyah/hugimethara/internal/platform/clock"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	problem "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Service is the use-case surface of the module.
type Service interface {
	Get(ctx context.Context, workspaceID uuid.UUID, window model.Window) (model.Overview, error)
}

var _ Service = (*OverviewService)(nil)

type OverviewService struct {
	pool  *sqlx.DB
	az    authzsvc.Authorizer
	clock clock.Clock
}

func NewService(pool *sqlx.DB, az authzsvc.Authorizer) *OverviewService {
	return &OverviewService{pool: pool, az: az, clock: clock.System{}}
}

// Get returns the workspace KPIs for window. Sections the caller cannot read
// (workflow, connector or webhook read in the workspace) are left nil; a
// caller who can read none of them is refused.
func (s *OverviewService) Get(ctx context.Context, workspaceID uuid.UUID, window model.Window) (model.Overview, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Overview{}, authzmodel.Unauthenticated()
	}
	if window == "" {
		window = model.Window24h
	}
	length, bucket, ok := window.Duration()
	if !ok {
		return model.Overview{}, problem.Validation(problem.FieldError{Field: "window", Code: "invalid_window",
			Message: "window must be 24h, 7d or 30d."})
	}
	if workspaceID == uuid.Nil {
		return model.Overview{}, problem.Validation(problem.FieldError{Field: "workspace_id", Code: "required",
			Message: "workspace_id is required."})
	}
	can := map[string]bool{}
	for _, rt := range []string{"workflow", "connector", "webhook"} {
		d, err := s.az.Evaluate(ctx, authzdto.AuthorizationInput{Principal: p, Action: "read",
			Resource: authzmodel.Resource{Type: rt, OrgID: p.OrgID, WorkspaceID: &workspaceID}})
		if err != nil {
			return model.Overview{}, err
		}
		can[rt] = d.Allowed
	}
	if !can["workflow"] && !can["connector"] && !can["webhook"] {
		// Same answer as every other workspace-scoped read the caller may not see.
		_, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: "workflow", OrgID: p.OrgID,
			WorkspaceID: &workspaceID})
		if err == nil {
			err = errors.New("overview: authorization changed during the request")
		}
		return model.Overview{}, err
	}

	now := s.clock.Now().UTC()
	from := now.Add(-length)
	out := model.Overview{WorkspaceID: workspaceID, Window: window, From: from, To: now}
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		if can["workflow"] {
			w := &model.WorkflowKPIs{}
			if err := tx.GetContext(ctx, w, queries.SQL.Get("WorkflowTotals"), workspaceID, from); err != nil {
				return err
			}
			w.Series = []model.SeriesPoint{}
			if err := tx.SelectContext(ctx, &w.Series, queries.SQL.Get("WorkflowSeries"), workspaceID, from, bucket); err != nil {
				return err
			}
			w.Series = fillSeries(w.Series, from, now, bucket)
			w.TopFailing = []model.FailingWorkflow{}
			if err := tx.SelectContext(ctx, &w.TopFailing, queries.SQL.Get("TopFailingWorkflows"), workspaceID, from); err != nil {
				return err
			}
			out.Workflows = w
		}
		if can["connector"] {
			c := &model.ConnectorKPIs{}
			if err := tx.GetContext(ctx, c, queries.SQL.Get("ConnectorHealth"), workspaceID); err != nil {
				return err
			}
			c.Failing = []model.UnhealthyConnector{}
			if err := tx.SelectContext(ctx, &c.Failing, queries.SQL.Get("UnhealthyConnectors"), workspaceID); err != nil {
				return err
			}
			out.Connectors = c
		}
		if can["webhook"] {
			h := &model.WebhookKPIs{}
			if err := tx.GetContext(ctx, h, queries.SQL.Get("WebhookTotals"), workspaceID, from); err != nil {
				return err
			}
			out.Webhooks = h
		}
		return nil
	})
	return out, err
}

// fillSeries returns one point per bucket from from to to, zero-filled.
func fillSeries(points []model.SeriesPoint, from, to time.Time, bucket string) []model.SeriesPoint {
	step := time.Hour
	start := from.Truncate(time.Hour)
	if bucket == "day" {
		step = 24 * time.Hour
		start = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	}
	have := make(map[int64]model.SeriesPoint, len(points))
	for _, p := range points {
		have[p.Bucket.UTC().Unix()] = p
	}
	out := []model.SeriesPoint{}
	for t := start; !t.After(to); t = t.Add(step) {
		p, ok := have[t.Unix()]
		if !ok {
			p = model.SeriesPoint{Bucket: t}
		}
		p.Bucket = t
		out = append(out, p)
	}
	return out
}
