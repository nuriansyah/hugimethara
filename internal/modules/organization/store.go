package organization

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

type orgRow struct {
	ID        uuid.UUID `db:"id"`
	Slug      string    `db:"slug"`
	Name      string    `db:"name"`
	Status    string    `db:"status"`
	Settings  []byte    `db:"settings"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (r orgRow) toModel() Organization {
	return Organization{ID: r.ID, Slug: r.Slug, Name: r.Name, Status: r.Status,
		Settings: ParseSettings(r.Settings), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
}

type departmentRow struct {
	ID             uuid.UUID `db:"id"`
	Slug           string    `db:"slug"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	WorkspaceCount int64     `db:"workspace_count"`
}

type environmentRow struct {
	ID              uuid.UUID `db:"id"`
	Slug            string    `db:"slug"`
	Name            string    `db:"name"`
	IsProduction    bool      `db:"is_production"`
	EgressAllowlist []byte    `db:"egress_allowlist"` // JSON array
	CreatedAt       time.Time `db:"created_at"`
	UpdatedAt       time.Time `db:"updated_at"`
}

type workspaceRow struct {
	ID             uuid.UUID      `db:"id"`
	Slug           string         `db:"slug"`
	Name           string         `db:"name"`
	Description    string         `db:"description"`
	DepartmentID   uuid.NullUUID  `db:"department_id"`
	DepartmentName sql.NullString `db:"department_name"`
	ArchivedAt     sql.NullTime   `db:"archived_at"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	MemberCount    int64          `db:"member_count"`
}

type memberRow struct {
	ID               uuid.UUID `db:"id"`
	Email            string    `db:"email"`
	DisplayName      string    `db:"display_name"`
	Status           string    `db:"status"`
	IsOwner          bool      `db:"is_owner"`
	MembershipStatus string    `db:"membership_status"`
	JoinedAt         time.Time `db:"joined_at"`
	AddedAt          time.Time `db:"added_at"`
	Roles            []byte    `db:"roles"` // JSON array of role keys
}
