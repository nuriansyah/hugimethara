package organization

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nuriansyah/hugimethara/backend/internal/audit"
	"github.com/nuriansyah/hugimethara/backend/internal/authz"
	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/organization/orgdb"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/backend/internal/principal"
)

// ErrSlugTaken is returned when another organization already uses the slug.
var ErrSlugTaken = httpx.NewProblem(http.StatusConflict, "slug_taken", "Address unavailable",
	"Another organization already uses this address. Choose a different one.")

// Service manages organizations.
type Service struct {
	pool *pgxpool.Pool
	az   authz.Authorizer
}

func NewService(pool *pgxpool.Pool, az authz.Authorizer) *Service {
	return &Service{pool: pool, az: az}
}

// NewOrganization is the input to Create.
type NewOrganization struct {
	ID   uuid.UUID
	Slug string
	Name string
}

// Create inserts an organization inside tx (which must be bound to o.ID).
// Validation is the caller's responsibility (see identity signup).
func (s *Service) Create(ctx context.Context, tx pgx.Tx, o NewOrganization) error {
	settings, _ := json.Marshal(DefaultSettings())
	err := orgdb.New(tx).InsertOrganization(ctx, orgdb.InsertOrganizationParams{
		ID: o.ID, Slug: o.Slug, Name: strings.TrimSpace(o.Name), Settings: settings,
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrSlugTaken
	}
	if err != nil {
		return fmt.Errorf("organization: insert: %w", err)
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
	var out Organization
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		row, err := orgdb.New(tx).GetOrganization(ctx, p.OrgID)
		if err != nil {
			return err
		}
		out = fromRow(row.ID, row.Slug, row.Name, row.Status, row.Settings, row.CreatedAt.Time, row.UpdatedAt.Time)
		return nil
	})
	if errors.Is(err, pgx.ErrNoRows) {
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
	err := db.InTenantTx(ctx, s.pool, p.OrgID, func(tx pgx.Tx) error {
		q := orgdb.New(tx)
		cur, err := q.GetOrganization(ctx, p.OrgID)
		if err != nil {
			return err
		}
		before := fromRow(cur.ID, cur.Slug, cur.Name, cur.Status, cur.Settings, cur.CreatedAt.Time, cur.UpdatedAt.Time)
		name, settings := before.Name, before.Settings
		if u.Name != nil {
			name = strings.TrimSpace(*u.Name)
		}
		if u.Settings != nil {
			settings = *u.Settings
		}
		rawSettings, _ := json.Marshal(settings)
		row, err := q.UpdateOrganization(ctx, orgdb.UpdateOrganizationParams{ID: p.OrgID, Name: name, Settings: rawSettings})
		if err != nil {
			return err
		}
		out = fromRow(row.ID, row.Slug, row.Name, row.Status, row.Settings, row.CreatedAt.Time, row.UpdatedAt.Time)

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
		_, err = event.Publish(ctx, tx, event.Event{
			OrgID: p.OrgID, Subject: "hugi.events.organization.updated",
			Payload: map[string]any{"organization_id": p.OrgID, "changed": keys(changes)},
		})
		return err
	})
	return out, err
}

func fromRow(id uuid.UUID, slug, name, status string, settings []byte, created, updated time.Time) Organization {
	return Organization{ID: id, Slug: slug, Name: name, Status: status, Settings: ParseSettings(settings),
		CreatedAt: created, UpdatedAt: updated}
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
