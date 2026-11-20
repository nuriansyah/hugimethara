// Package migrate runs the embedded goose migrations and reports whether a
// database is at the version this binary expects.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx"
	"github.com/jmoiron/sqlx"
	"github.com/pressly/goose/v3"

	"github.com/nuriansyah/hugimethara/migrations/postgres"
)

// NewProvider opens url (owner role) and returns a goose provider over the embedded migrations.
func NewProvider(url string) (*goose.Provider, *sql.DB, error) {
	sqlDB, err := sql.Open("pgx", url)
	if err != nil {
		return nil, nil, errors.New("migrate: invalid database URL")
	}
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		_ = sqlDB.Close()
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	return p, sqlDB, nil
}

// ExpectedVersion is the highest migration version embedded in this binary.
func ExpectedVersion() (int64, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return 0, err
	}
	var highest int64
	for _, e := range entries {
		name := e.Name()
		if path.Ext(name) != ".sql" {
			continue
		}
		prefix, _, ok := strings.Cut(name, "_")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(prefix, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("migrate: bad migration name %q", name)
		}
		highest = max(highest, v)
	}
	return highest, nil
}

// Check is a readiness check: the database must be migrated to at least the
// version this binary was built with (older binaries may run against newer
// schemas during rolling deploys, which is why "at least" and not "equal").
func Check(pool *sqlx.DB) func(context.Context) error {
	want, err := ExpectedVersion()
	return func(ctx context.Context) error {
		if err != nil {
			return errors.New("cannot determine expected migration version")
		}
		var got int64
		err := pool.GetContext(ctx, &got,
			`SELECT COALESCE(MAX(version_id), 0) FROM goose_db_version WHERE is_applied`)
		if err != nil {
			return errors.New("cannot read migration version")
		}
		if got < want {
			return fmt.Errorf("database at migration %d, binary expects %d", got, want)
		}
		return nil
	}
}
