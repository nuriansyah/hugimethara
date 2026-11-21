// Package commands holds the write-side SQL of the integration/secrets repository: every
// statement here mutates state and must run inside a tenant-bound transaction.
package commands

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed commands.sql
var files embed.FS

// SQL is the registry of named write statements, parsed once at start-up.
var SQL = db.MustLoad(files, "commands.sql")
