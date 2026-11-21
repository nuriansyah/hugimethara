// Package queries holds the read-side SQL of the webhook repository: statements
// here never mutate state.
package queries

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var files embed.FS

// SQL is the registry of named statements, parsed once at start-up.
var SQL = db.MustLoad(files, "queries.sql")
