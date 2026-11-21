package services

import (
	"context"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/security/egress"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	entmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/model"
	entsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/entitlement/services"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
)

// normalizeAllowlist validates CIDRs/IPs and returns canonical, de-duplicated prefixes.
func normalizeAllowlist(in []string) ([]string, *apperrors.FieldError) {
	if len(in) > 100 {
		return nil, &apperrors.FieldError{Field: "egress_allowlist", Code: "too_many", Message: "At most 100 entries."}
	}
	out := []string{}
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pol, err := egress.ParsePolicy([]string{raw})
		if err != nil {
			return nil, &apperrors.FieldError{Field: "egress_allowlist", Code: "invalid_cidr",
				Message: "\"" + raw + "\" is not an IP address or CIDR range (e.g. 10.20.0.0/16)."}
		}
		if pol.Allow[0].Bits() == 0 {
			return nil, &apperrors.FieldError{Field: "egress_allowlist", Code: "too_broad",
				Message: "\"" + raw + "\" would allow every address. List specific internal ranges."}
		}
		c := pol.Allow[0].String()
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *OrganizationService) ListEnvironments(ctx context.Context) ([]model.Environment, error) {
	p, err := s.authorize(ctx, "read", model.EnvironmentResource, "", nil)
	if err != nil {
		return nil, err
	}
	var rows []repository.EnvironmentRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, queries.SQL.Get("ListEnvironments"))
	}); err != nil {
		return nil, err
	}
	out := make([]model.Environment, len(rows))
	for i, r := range rows {
		out[i] = r.ToModel()
	}
	return out, nil
}

func (s *OrganizationService) CreateEnvironment(ctx context.Context, in dto.EnvironmentInput) (model.Environment, error) {
	p, err := s.authorize(ctx, "create", model.EnvironmentResource, "", nil)
	if err != nil {
		return model.Environment{}, err
	}
	in.Slug, in.Name = strings.ToLower(strings.TrimSpace(in.Slug)), strings.TrimSpace(in.Name)
	allow, fe := normalizeAllowlist(in.EgressAllowlist)
	if err := model.Collect(model.ValidateShortSlug("slug", in.Slug), model.ValidateText("name", in.Name, 1, 100, "Name"), fe); err != nil {
		return model.Environment{}, err
	}
	var out repository.EnvironmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if err := entsvc.LockFeature(ctx, tx, p.OrgID, entmodel.EnvironmentsMax); err != nil {
			return err
		}
		ent, err := entsvc.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if len(allow) > 0 {
			if err := ent.Require(entmodel.EgressAllowlist); err != nil {
				return err
			}
		}
		var n int64
		if err := tx.GetContext(ctx, &n, queries.SQL.Get("CountEnvironments")); err != nil {
			return err
		}
		if err := ent.CheckLimit(entmodel.EnvironmentsMax, n, 1); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertEnvironment"), id, p.OrgID, in.Slug, in.Name, in.IsProduction, allow); err != nil {
			if db.IsUniqueViolation(err) {
				return model.Conflict("slug_taken", "An environment with this address already exists.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetEnvironment"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "environment.created",
			ResourceType: model.EnvironmentResource, ResourceID: id.String(),
			Details: map[string]any{"slug": in.Slug, "is_production": in.IsProduction, "egress_allowlist": allow}})
	})
	return out.ToModel(), err
}

func (s *OrganizationService) UpdateEnvironment(ctx context.Context, id uuid.UUID, in dto.EnvironmentInput) (model.Environment, error) {
	p, err := s.authorize(ctx, "update", model.EnvironmentResource, id.String(), nil)
	if err != nil {
		return model.Environment{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	allow, fe := normalizeAllowlist(in.EgressAllowlist)
	if err := model.Collect(model.ValidateText("name", in.Name, 1, 100, "Name"), fe); err != nil {
		return model.Environment{}, err
	}
	var out repository.EnvironmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before repository.EnvironmentRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetEnvironment"), id); err != nil {
			if db.IsNoRows(err) {
				return model.ErrEnvironmentNotFound
			}
			return err
		}
		prev := before.ToModel()
		if len(allow) > 0 && !slices.Equal(allow, prev.EgressAllowlist) {
			ent, err := entsvc.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			if err := ent.Require(entmodel.EgressAllowlist); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("UpdateEnvironment"), id, in.Name, in.IsProduction, allow); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, queries.SQL.Get("GetEnvironment"), id); err != nil {
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "environment.updated",
			ResourceType: model.EnvironmentResource, ResourceID: id.String(),
			Details: map[string]any{
				"from": map[string]any{"name": prev.Name, "is_production": prev.IsProduction, "egress_allowlist": prev.EgressAllowlist},
				"to":   map[string]any{"name": in.Name, "is_production": in.IsProduction, "egress_allowlist": allow},
			}})
	})
	return out.ToModel(), err
}

// DeleteEnvironment removes an environment. The last one cannot be deleted;
// later slices add foreign keys that block deleting environments in use.
func (s *OrganizationService) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	p, err := s.authorize(ctx, "delete", model.EnvironmentResource, id.String(), nil)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before repository.EnvironmentRow
		if err := tx.GetContext(ctx, &before, queries.SQL.Get("GetEnvironment"), id); err != nil {
			if db.IsNoRows(err) {
				return model.ErrEnvironmentNotFound
			}
			return err
		}
		var n int64
		if err := tx.GetContext(ctx, &n, queries.SQL.Get("CountEnvironments")); err != nil {
			return err
		}
		if n <= 1 {
			return model.Conflict("last_environment", "An organization needs at least one environment.")
		}
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("DeleteEnvironment"), id); err != nil {
			if db.IsForeignKeyViolation(err) {
				return model.Conflict("environment_in_use", "This environment is still used by other resources.")
			}
			return err
		}
		return auditsvc.Record(ctx, tx, auditmodel.Event{OrgID: p.OrgID, Action: "environment.deleted",
			ResourceType: model.EnvironmentResource, ResourceID: id.String(), Details: map[string]any{"slug": before.Slug}})
	})
}
