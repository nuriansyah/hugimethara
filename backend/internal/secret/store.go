package secret

import (
	"embed"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
)

//go:embed queries.sql
var queryFS embed.FS

var q = db.MustLoad(queryFS, "queries.sql")
