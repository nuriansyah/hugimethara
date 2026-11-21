package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	logging "github.com/nuriansyah/hugimethara/internal/platform/logger"
)

var subjectRe = regexp.MustCompile(`^hugi\.[a-z0-9_-]+(\.[a-z0-9_-]+)+$`)

// Event is a message to deliver after the surrounding transaction commits.
type Event struct {
	OrgID   uuid.UUID
	Subject string // e.g. "hugi.events.workflow.published"
	Payload any    // JSON-serializable
}

// Publish appends ev to the outbox using tx. tx must be tenant-bound
// (db.InTenantTx) for ev.OrgID, otherwise RLS rejects the insert.
// Trace context and request id are captured so consumers continue the trace.
func Publish(ctx context.Context, tx *sqlx.Tx, ev Event) (int64, error) {
	if ev.OrgID == uuid.Nil {
		return 0, errors.New("event: org id required")
	}
	if !subjectRe.MatchString(ev.Subject) {
		return 0, fmt.Errorf("event: invalid subject %q", ev.Subject)
	}
	payload, err := json.Marshal(ev.Payload)
	if err != nil {
		return 0, fmt.Errorf("event: marshal payload: %w", err)
	}
	hdr := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, hdr)
	if id := logging.RequestID(ctx); id != "" {
		hdr["X-Request-Id"] = id
	}
	headers, _ := json.Marshal(map[string]string(hdr))

	var id int64
	if err := tx.GetContext(ctx, &id, q.Get("InsertOutbox"), ev.OrgID, ev.Subject, payload, headers); err != nil {
		return 0, fmt.Errorf("event: insert outbox: %w", err)
	}
	return id, nil
}
