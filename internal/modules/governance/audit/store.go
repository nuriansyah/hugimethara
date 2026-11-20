package audit

import (
	"database/sql"
	"embed"
	"time"

	"github.com/google/uuid"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")

type eventRow struct {
	ID           uuid.UUID      `db:"id"`
	OccurredAt   time.Time      `db:"occurred_at"`
	ActorType    string         `db:"actor_type"`
	ActorID      uuid.NullUUID  `db:"actor_id"`
	ActorEmail   sql.NullString `db:"actor_email"`
	Action       string         `db:"action"`
	ResourceType string         `db:"resource_type"`
	ResourceID   sql.NullString `db:"resource_id"`
	WorkspaceID  uuid.NullUUID  `db:"workspace_id"`
	Outcome      string         `db:"outcome"`
	RequestID    sql.NullString `db:"request_id"`
	IP           sql.NullString `db:"ip"`
	Details      []byte         `db:"details"`
}

// nullable helpers for query parameters.
func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func nullUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}
