// Package queries holds the read-side SQL of the data platform repository.
package queries

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var files embed.FS

// SQL is the registry of named read statements.
var SQL = db.MustLoad(files, "queries.sql")
