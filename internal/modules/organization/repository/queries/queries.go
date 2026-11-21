// Package queries holds the read-side SQL of the organization repository: statements
// here never mutate state and are safe to run in read-only transactions.
package queries

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var files embed.FS

// SQL is the registry of named read statements, parsed once at start-up.
var SQL = db.MustLoad(files, "queries.sql")
