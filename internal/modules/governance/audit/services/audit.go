package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/model"
	"github.com/nuriansyah/hugimethara/internal/modules/governance/audit/repository/commands"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// Record inserts e using tx, which must be bound to e.OrgID.
func Record(ctx context.Context, tx *sqlx.Tx, e model.Event) error {
	if e.OrgID == uuid.Nil || e.Action == "" || e.ResourceType == "" {
		return errors.New("audit: org, action and resource type are required")
	}
	if e.Outcome == "" {
		e.Outcome = model.Success
	}
	actor := model.ResolveActor(ctx, e.Actor)
	details, err := json.Marshal(redact(e.Details))
	if err != nil {
		return fmt.Errorf("audit: marshal details: %w", err)
	}
	ci := principal.ClientInfoFrom(ctx)
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
	_, err = tx.ExecContext(ctx, commands.SQL.Get("InsertAuditEvent"),
		uuid.Must(uuid.NewV7()), e.OrgID, time.Now(), actor.Type, actorID, e.Action, e.ResourceType,
		model.NullStr(e.ResourceID), workspaceID, e.Outcome, model.NullStr(logging.RequestID(ctx)), ip,
		model.NullStr(ci.UserAgent), details)
	if err != nil {
		return fmt.Errorf("audit: insert: %w", err)
	}
	return nil
}

// RecordStandalone writes an event in its own tenant-bound transaction. Use it
// for events not tied to a data change (e.g. failed sign-in).
func RecordStandalone(ctx context.Context, pool *sqlx.DB, e model.Event) error {
	return db.InTenantTx(ctx, pool, e.OrgID, func(tx *sqlx.Tx) error { return Record(ctx, tx, e) })
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
