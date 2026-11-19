package organization

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/entitlement"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/egress"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// Environment is a deployment stage (development, staging, production, …).
type Environment struct {
	ID              uuid.UUID
	Slug            string
	Name            string
	IsProduction    bool
	EgressAllowlist []string // CIDRs allowed through the SSRF guard
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// EnvironmentInput creates or updates an environment (Slug is create-only).
type EnvironmentInput struct {
	Slug            string
	Name            string
	IsProduction    bool
	EgressAllowlist []string
}

func (r environmentRow) toModel() Environment {
	e := Environment{ID: r.ID, Slug: r.Slug, Name: r.Name, IsProduction: r.IsProduction,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, EgressAllowlist: []string{}}
	_ = json.Unmarshal(r.EgressAllowlist, &e.EgressAllowlist)
	return e
}

var errEnvironmentNotFound = httpx.NotFound("Environment not found.")

// normalizeAllowlist validates CIDRs/IPs and returns canonical, de-duplicated prefixes.
func normalizeAllowlist(in []string) ([]string, *httpx.FieldError) {
	if len(in) > 100 {
		return nil, &httpx.FieldError{Field: "egress_allowlist", Code: "too_many", Message: "At most 100 entries."}
	}
	out := []string{}
	for _, raw := range in {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		pol, err := egress.ParsePolicy([]string{raw})
		if err != nil {
			return nil, &httpx.FieldError{Field: "egress_allowlist", Code: "invalid_cidr",
				Message: "\"" + raw + "\" is not an IP address or CIDR range (e.g. 10.20.0.0/16)."}
		}
		if pol.Allow[0].Bits() == 0 {
			return nil, &httpx.FieldError{Field: "egress_allowlist", Code: "too_broad",
				Message: "\"" + raw + "\" would allow every address. List specific internal ranges."}
		}
		c := pol.Allow[0].String()
		if !slices.Contains(out, c) {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *Service) ListEnvironments(ctx context.Context) ([]Environment, error) {
	p, err := s.authorize(ctx, "read", EnvironmentResource, "", nil)
	if err != nil {
		return nil, err
	}
	var rows []environmentRow
	if err := s.tx(ctx, p, func(tx *sqlx.Tx) error {
		return tx.SelectContext(ctx, &rows, q.Get("ListEnvironments"))
	}); err != nil {
		return nil, err
	}
	out := make([]Environment, len(rows))
	for i, r := range rows {
		out[i] = r.toModel()
	}
	return out, nil
}

func (s *Service) CreateEnvironment(ctx context.Context, in EnvironmentInput) (Environment, error) {
	p, err := s.authorize(ctx, "create", EnvironmentResource, "", nil)
	if err != nil {
		return Environment{}, err
	}
	in.Slug, in.Name = strings.ToLower(strings.TrimSpace(in.Slug)), strings.TrimSpace(in.Name)
	allow, fe := normalizeAllowlist(in.EgressAllowlist)
	if err := collect(validateShortSlug("slug", in.Slug), validateText("name", in.Name, 1, 100, "Name"), fe); err != nil {
		return Environment{}, err
	}
	var out environmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		if err := entitlement.LockFeature(ctx, tx, p.OrgID, entitlement.EnvironmentsMax); err != nil {
			return err
		}
		ent, err := entitlement.Load(ctx, tx, p.OrgID)
		if err != nil {
			return err
		}
		if len(allow) > 0 {
			if err := ent.Require(entitlement.EgressAllowlist); err != nil {
				return err
			}
		}
		var n int64
		if err := tx.GetContext(ctx, &n, q.Get("CountEnvironments")); err != nil {
			return err
		}
		if err := ent.CheckLimit(entitlement.EnvironmentsMax, n, 1); err != nil {
			return err
		}
		id := uuid.Must(uuid.NewV7())
		if _, err := tx.ExecContext(ctx, q.Get("InsertEnvironment"), id, p.OrgID, in.Slug, in.Name, in.IsProduction, allow); err != nil {
			if db.IsUniqueViolation(err) {
				return conflict("slug_taken", "An environment with this address already exists.")
			}
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetEnvironment"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "environment.created",
			ResourceType: EnvironmentResource, ResourceID: id.String(),
			Details: map[string]any{"slug": in.Slug, "is_production": in.IsProduction, "egress_allowlist": allow}})
	})
	return out.toModel(), err
}

func (s *Service) UpdateEnvironment(ctx context.Context, id uuid.UUID, in EnvironmentInput) (Environment, error) {
	p, err := s.authorize(ctx, "update", EnvironmentResource, id.String(), nil)
	if err != nil {
		return Environment{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	allow, fe := normalizeAllowlist(in.EgressAllowlist)
	if err := collect(validateText("name", in.Name, 1, 100, "Name"), fe); err != nil {
		return Environment{}, err
	}
	var out environmentRow
	err = s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before environmentRow
		if err := tx.GetContext(ctx, &before, q.Get("GetEnvironment"), id); err != nil {
			if db.IsNoRows(err) {
				return errEnvironmentNotFound
			}
			return err
		}
		prev := before.toModel()
		if len(allow) > 0 && !slices.Equal(allow, prev.EgressAllowlist) {
			ent, err := entitlement.Load(ctx, tx, p.OrgID)
			if err != nil {
				return err
			}
			if err := ent.Require(entitlement.EgressAllowlist); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, q.Get("UpdateEnvironment"), id, in.Name, in.IsProduction, allow); err != nil {
			return err
		}
		if err := tx.GetContext(ctx, &out, q.Get("GetEnvironment"), id); err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "environment.updated",
			ResourceType: EnvironmentResource, ResourceID: id.String(),
			Details: map[string]any{
				"from": map[string]any{"name": prev.Name, "is_production": prev.IsProduction, "egress_allowlist": prev.EgressAllowlist},
				"to":   map[string]any{"name": in.Name, "is_production": in.IsProduction, "egress_allowlist": allow},
			}})
	})
	return out.toModel(), err
}

// DeleteEnvironment removes an environment. The last one cannot be deleted;
// later slices add foreign keys that block deleting environments in use.
func (s *Service) DeleteEnvironment(ctx context.Context, id uuid.UUID) error {
	p, err := s.authorize(ctx, "delete", EnvironmentResource, id.String(), nil)
	if err != nil {
		return err
	}
	return s.tx(ctx, p, func(tx *sqlx.Tx) error {
		var before environmentRow
		if err := tx.GetContext(ctx, &before, q.Get("GetEnvironment"), id); err != nil {
			if db.IsNoRows(err) {
				return errEnvironmentNotFound
			}
			return err
		}
		var n int64
		if err := tx.GetContext(ctx, &n, q.Get("CountEnvironments")); err != nil {
			return err
		}
		if n <= 1 {
			return conflict("last_environment", "An organization needs at least one environment.")
		}
		if _, err := tx.ExecContext(ctx, q.Get("DeleteEnvironment"), id); err != nil {
			if db.IsForeignKeyViolation(err) {
				return conflict("environment_in_use", "This environment is still used by other resources.")
			}
			return err
		}
		return audit.Record(ctx, tx, audit.Event{OrgID: p.OrgID, Action: "environment.deleted",
			ResourceType: EnvironmentResource, ResourceID: id.String(), Details: map[string]any{"slug": before.Slug}})
	})
}
