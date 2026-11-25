// Package commands holds the write-side SQL of the analytics repository.
package commands

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed commands.sql
var files embed.FS

// SQL is the registry of named write statements.
var SQL = db.MustLoad(files, "commands.sql")
