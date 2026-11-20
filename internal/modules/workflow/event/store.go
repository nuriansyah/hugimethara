package event

import (
	"embed"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")

type outboxRow struct {
	ID       int64  `db:"id"`
	OrgID    string `db:"org_id"`
	Subject  string `db:"subject"`
	Payload  []byte `db:"payload"`
	Headers  []byte `db:"headers"`
	Attempts int32  `db:"attempts"`
}
