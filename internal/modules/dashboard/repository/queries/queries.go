// Package queries holds the read-side SQL of the dashboard repository.
package queries

import (
	"embed"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

//go:embed queries.sql
var files embed.FS

var SQL = db.MustLoad(files, "queries.sql")
