package organization

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/authorization"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit"
	"github.com/nuriansyah/hugimethara/internal/modules/workflow/event"
	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// ErrSlugTaken is returned when another organization already uses the slug.
var ErrSlugTaken = httpx.NewProblem(http.StatusConflict, "slug_taken", "Address unavailable",
	"Another organization already uses this address. Choose a different one.")

// Service manages organizations.
type Service struct {
	pool   *sqlx.DB
	az     Authorizer
	scopes authz.ScopeResolver
	acl    *authz.Service
}

// Authorizer is what the organization domain needs from authz.
type Authorizer interface {
	authz.Authorizer
	authz.ScopeResolver
	Check(ctx context.Context, in authz.AuthorizationInput) (authz.AuthorizationDecision, error)
}

func NewService(pool *sqlx.DB, az Authorizer, acl *authz.Service) *Service {
	return &Service{pool: pool, az: az, scopes: az, acl: acl}
}

// NewOrganization is the input to Create.
type NewOrganization struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// Create inserts an organization and its default environments inside tx
// (which must be bound to o.ID). Validation is the caller's responsibility
// (see identity signup).
func (s *Service) Create(ctx context.Context, tx *sqlx.Tx, o NewOrganization) error {
	settings, _ := json.Marshal(DefaultSettings())
	_, err := tx.ExecContext(ctx, q.Get("InsertOrganization"), o.ID, o.Slug, strings.TrimSpace(o.Name), settings)
	if db.IsUniqueViolation(err) {
		return ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("organization: insert: %w", err)
	}
	for _, e := range DefaultEnvironments {
		if _, err := tx.ExecContext(ctx, q.Get("InsertEnvironment"),
			uuid.Must(uuid.NewV7()), o.ID, e.Slug, e.Name, e.IsProduction, []string{}); err != nil {
			return fmt.Errorf("organization: seed environment %s: %w", e.Slug, err)
		}
	}
	return nil
}

// Current returns the caller's organization.
func (s *Service) Current(ctx context.Context) (Organization, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Organization{}, authz.Unauthenticated()
	}
	if _, err := authz.Require(ctx, s.az, "read", authz.Resource{Type: ResourceType, ID: p.OrgID.String(), OrgID: p.OrgID}); err != nil {
		return Organization{}, err
	}
	var row orgRow
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &row, q.Get("GetOrganization"), p.OrgID)
	})
	out := row.toModel()
	if db.IsNoRows(err) {
		return Organization{}, httpx.NotFound("Organization not found.")
	}
	return out, err
}

// Update is a partial update; nil fields are left unchanged.
type Update struct {
	Name     *string
	Settings *Settings
}

// UpdateCurrent applies u to the caller's organization, recording an audit event
// and an outbox event in the same transaction.
func (s *Service) UpdateCurrent(ctx context.Context, u Update) (Organization, error) {
	p, ok := principal.From(ctx)
	if !ok {
		return Organization{}, authz.Unauthenticated()
	}
	if _, err := authz.Require(ctx, s.az, "update", authz.Resource{Type: ResourceType, ID: p.OrgID.String(), OrgID: p.OrgID}); err != nil {
		return Organization{}, err
	}
	var fieldErrs []httpx.FieldError
	if u.Name != nil {
		if fe := ValidateName("name", *u.Name); fe != nil {
			fieldErrs = append(fieldErrs, *fe)
		}
	}
	if u.Settings != nil {
		fieldErrs = append(fieldErrs, u.Settings.Validate("settings.")...)
	}
	if len(fieldErrs) > 0 {
		return Organization{}, httpx.Validation(fieldErrs...)
	}

	var out Organization
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx *sqlx.Tx) error {
		var cur orgRow
		if err := tx.GetContext(ctx, &cur, q.Get("GetOrganization"), p.OrgID); err != nil {
			return err
		}
		before := cur.toModel()
		name, settings := before.Name, before.Settings
		if u.Name != nil {
			name = strings.TrimSpace(*u.Name)
		}
		if u.Settings != nil {
			settings = *u.Settings
		}
		rawSettings, _ := json.Marshal(settings)
		var row orgRow
		if err := tx.GetContext(ctx, &row, q.Get("UpdateOrganization"), p.OrgID, name, rawSettings); err != nil {
			return err
		}
		out = row.toModel()

		changes := diff(before, out)
		if len(changes) == 0 {
			return nil
		}
		if err := audit.Record(ctx, tx, audit.Event{
			OrgID: p.OrgID, Action: "organization.updated", ResourceType: ResourceType,
			ResourceID: p.OrgID.String(), Details: map[string]any{"changes": changes},
		}); err != nil {
			return err
		}
		_, err := event.Publish(ctx, tx, event.Event{
			OrgID: p.OrgID, Subject: "hugi.events.organization.updated",
			Payload: map[string]any{"organization_id": p.OrgID, "changed": keys(changes)},
		})
		return err
	})
	return out, err
}

func diff(a, b Organization) map[string]any {
	ch := map[string]any{}
	if a.Name != b.Name {
		ch["name"] = map[string]any{"from": a.Name, "to": b.Name}
	}
	if a.Settings.SessionIdleTimeoutMinutes != b.Settings.SessionIdleTimeoutMinutes {
		ch["settings.session_idle_timeout_minutes"] = map[string]any{"from": a.Settings.SessionIdleTimeoutMinutes, "to": b.Settings.SessionIdleTimeoutMinutes}
	}
	if a.Settings.SessionAbsoluteTimeoutHours != b.Settings.SessionAbsoluteTimeoutHours {
		ch["settings.session_absolute_timeout_hours"] = map[string]any{"from": a.Settings.SessionAbsoluteTimeoutHours, "to": b.Settings.SessionAbsoluteTimeoutHours}
	}
	return ch
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// UsageCounters reports current usage for the plan limits this domain enforces.
func UsageCounters() map[string]func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
	count := func(name string) func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
		return func(ctx context.Context, tx *sqlx.Tx) (int64, error) {
			var n int64
			err := tx.GetContext(ctx, &n, q.Get(name))
			return n, err
		}
	}
	return map[string]func(ctx context.Context, tx *sqlx.Tx) (int64, error){
		"workspaces.max":   count("CountActiveWorkspaces"),
		"environments.max": count("CountEnvironments"),
	}
}
