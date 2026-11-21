package repository

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type EventRow struct {
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
