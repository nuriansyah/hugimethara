package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/infrastructure/persistence/outbox"
	authzmodel "github.com/nuriansyah/hugimethara/internal/modules/authorization/model"
	authzdto "github.com/nuriansyah/hugimethara/internal/modules/authorization/model/dto"
	authzsvc "github.com/nuriansyah/hugimethara/internal/modules/authorization/services"
	auditmodel "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	auditsvc "github.com/nuriansyah/hugimethara/internal/modules/governance/audit/services"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/model/dto"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/organization/repository/queries"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	apperrors "github.com/nuriansyah/hugimethara/internal/platform/errors"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// OrganizationService manages organizations.
type OrganizationService struct {
	pool   *sqlx.DB
	az     Authorizer
	scopes authzsvc.ScopeResolver
	acl    *authzsvc.AuthorizationService
}

// Authorizer is what the organization domain needs from authz.
type Authorizer interface {
	authzsvc.Authorizer
	authzsvc.ScopeResolver
	Check(ctx context.Context, in authzdto.AuthorizationInput) (authzmodel.AuthorizationDecision, error)
}

func NewService(pool *sqlx.DB, az Authorizer, acl *authzsvc.AuthorizationService) *OrganizationService {
	return &OrganizationService{pool: pool, az: az, scopes: az, acl: acl}
}

// Create inserts an organization and its default environments inside tx
// (which must be bound to o.ID). Validation is the caller's responsibility
// (see identity signup).
func (s *OrganizationService) Create(ctx context.Context, tx *sqlx.Tx, o dto.NewOrganization) error {
	settings, _ := json.Marshal(model.DefaultSettings())
	_, err := tx.ExecContext(ctx, commands.SQL.Get("InsertOrganization"), o.ID, o.Slug, strings.TrimSpace(o.Name), settings)
	if db.IsUniqueViolation(err) {
		return model.ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("organization: insert: %w", err)
	}
	for _, e := range model.DefaultEnvironments {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertEnvironment"),
			uuid.Must(uuid.NewV7()), o.ID, e.Slug, e.Name, e.IsProduction, []string{}); err != nil {
			return fmt.Errorf("organization: seed environment %s: %w", e.Slug, err)
		}
	}
	return nil
}

// Current returns the caller's organization.
func (s *OrganizationService) Current(ctx context.Context) (model.Organization, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Organization{}, authzmodel.Unauthenticated()
	}
	if _, err := authzsvc.Require(ctx, s.az, "read", authzmodel.Resource{Type: model.ResourceType, ID: p.OrgID.String(), OrgID: p.OrgID}); err != nil {
		return model.Organization{}, err
	}
	var row repository.OrgRow
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, queries.SQL.Get("GetOrganization"), p.OrgID)
	})
	out := row.ToModel()
	if db.IsNoRows(err) {
		return model.Organization{}, apperrors.NotFound("Organization not found.")
	}
	return out, err
}

// UpdateCurrent applies u to the caller's organization, recording an audit event
// and an outbox event in the same transaction.
func (s *OrganizationService) UpdateCurrent(ctx context.Context, u dto.Update) (model.Organization, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return model.Organization{}, authzmodel.Unauthenticated()
	}
	if _, err := authzsvc.Require(ctx, s.az, "update", authzmodel.Resource{Type: model.ResourceType, ID: p.OrgID.String(), OrgID: p.OrgID}); err != nil {
		return model.Organization{}, err
	}
	var fieldErrs []apperrors.FieldError
	if u.Name != nil {
		if fe := model.ValidateName("name", *u.Name); fe != nil {
			fieldErrs = append(fieldErrs, *fe)
		}
	}
	if u.Settings != nil {
		fieldErrs = append(fieldErrs, u.Settings.Validate("settings.")...)
	}
	if len(fieldErrs) > 0 {
		return model.Organization{}, apperrors.Validation(fieldErrs...)
	}

	var out model.Organization
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var cur repository.OrgRow
		if err := tx.GetContext(ctx, &cur, queries.SQL.Get("GetOrganization"), p.OrgID); err != nil {
			return err
		}
		before := cur.ToModel()
		name, settings := before.Name, before.Settings
		if u.Name != nil {
			name = strings.TrimSpace(*u.Name)
		}
		if u.Settings != nil {
			settings = *u.Settings
		}
		rawSettings, _ := json.Marshal(settings)
		var row repository.OrgRow
		if err := tx.GetContext(ctx, &row, commands.SQL.Get("UpdateOrganization"), p.OrgID, name, rawSettings); err != nil {
			return err
		}
		out = row.ToModel()

		changes := model.Diff(before, out)
		if len(changes) == 0 {
			return nil
		}
		if err := auditsvc.Record(ctx, tx, auditmodel.Event{
			OrgID: p.OrgID, Action: "organization.updated", ResourceType: model.ResourceType,
			ResourceID: p.OrgID.String(), Details: map[string]any{"changes": changes},
		}); err != nil {
			return err
		}
		_, err := outbox.Publish(ctx, tx, outbox.Event{
			OrgID: p.OrgID, Subject: "hugi.events.organization.updated",
			Payload: map[string]any{"organization_id": p.OrgID, "changed": model.Keys(changes)},
		})
		return err
	})
	return out, err
}

// UsageCounters reports current usage for the plan limits this domain enforces.
func UsageCounters() map[string]func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
	count := func(name string) func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
		return func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
			var n int64
			err := tx.GetContext(ctx, &n, queries.SQL.Get(name))
			return n, err
		}
	}
	return map[string]func(ctx context.Context, tx *sqlx.Tx) (int64, error){
		"workspaces.max":   count("CountActiveWorkspaces"),
		"environments.max": count("CountEnvironments"),
	}
}
