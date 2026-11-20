// Package audit records and lists the organization audit trail. Events are
// written in the same transaction as the change they describe, so an audit
// row exists if and only if the change committed.
package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/httpx"
	"github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Outcomes.
const (
	Success = "success"
	Failure = "failure"
	Denied  = "denied"
)

// Actor overrides the actor derived from the request principal (e.g. during
// signup, before a principal exists).
type Actor struct {
	Type string // user | service_account | system | anonymous
	ID   uuid.UUID
}

// Event is one audit record.
type Event struct {
	OrgID        uuid.UUID
	Action       string // e.g. "organization.updated"
	ResourceType string
	ResourceID   string
	WorkspaceID  *uuid.UUID
	Outcome      string         // default Success
	Details      map[string]any // never secrets; sensitive-looking keys are redacted
	Actor        *Actor
}

// Record inserts e using tx, which must be bound to e.OrgID.
func Record(ctx context.Context, tx *sqlx.Tx, e Event) error {
	if e.OrgID == uuid.Nil || e.Action == "" || e.ResourceType == "" {
		return errors.New("audit: org, action and resource type are required")
	}
	if e.Outcome == "" {
		e.Outcome = Success
	}
	actor := resolveActor(ctx, e.Actor)
	details, err := json.Marshal(redact(e.Details))
	if err != nil {
		return fmt.Errorf("audit: marshal details: %w", err)
	}
	ci := httpx.ClientInfoFrom(ctx)
	var actorID, workspaceID, ip any
	if actor.ID != uuid.Nil {
		actorID = actor.ID
	}
	if e.WorkspaceID != nil {
		workspaceID = *e.WorkspaceID
	}
	if ci.IP.IsValid() {
		ip = ci.IP.String()
	}
	_, err = tx.ExecContext(ctx, q.Get("InsertAuditEvent"),
		uuid.Must(uuid.NewV7()), e.OrgID, time.Now(), actor.Type, actorID, e.Action, e.ResourceType,
		nullStr(e.ResourceID), workspaceID, e.Outcome, nullStr(logging.RequestID(ctx)), ip,
		nullStr(ci.UserAgent), details)
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

// RecordStandalone writes an event in its own tenant-bound transaction. Use it
// for events not tied to a data change (e.g. failed sign-in).
func RecordStandalone(ctx context.Context, pool *sqlx.DB, e Event) error {
	return db.InTenantTx(ctx, pool, e.OrgID, func(tx *sqlx.Tx) error { return Record(ctx, tx, e) })
}

func resolveActor(ctx context.Context, override *Actor) Actor {
	if override != nil {
		return *override
	}
	if p, ok := principal.From(ctx); ok {
		return Actor{Type: string(p.Type), ID: p.ID}
	}
	return Actor{Type: "anonymous"}
}

func redact(m map[string]any) map[string]any {
	if len(m) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if logging.IsSensitiveKey(k) {
			out[k] = logging.Redacted
			continue
		}
		if nested, ok := v.(map[string]any); ok {
			v = redact(nested)
		}
		out[k] = v
	}
	return out
}
