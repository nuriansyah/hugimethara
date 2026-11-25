// Package commands holds the write-side SQL of the dashboard repository.
package commands

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed commands.sql
var files embed.FS

var SQL = db.MustLoad(files, "commands.sql")
