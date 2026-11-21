// Package commands holds the write-side SQL of the workflow repository: every
// statement here mutates state (or locks rows for update).
package commands

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed commands.sql
var files embed.FS

// SQL is the registry of named statements, parsed once at start-up.
var SQL = db.MustLoad(files, "commands.sql")
