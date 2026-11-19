// Package testsupport provides real dependencies for integration tests (build
// tag "integration"). Postgres and Redis come from existing servers named by
// environment variables, so tests run without Docker:
//
//	HUGI_TEST_POSTGRES_URL  superuser URL of a disposable server, e.g.
//	                        postgres://hugi_owner@127.0.0.1:55499/postgres?sslmode=disable
//	HUGI_TEST_REDIS_URL     e.g. redis://127.0.0.1:56379/0
//
// Each StartPostgres call creates a fresh database (dropped on cleanup),
// provisions roles exactly like deploy/compose, and runs the embedded
// migrations. NATS runs in-process (see nats.go). Tests skip when a variable
// is unset.
package testsupport

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/migrate"
)

const testPassword = "test-only-password"

// Postgres is a migrated database with one pool per runtime role.
type Postgres struct {
	OwnerURL string
	Owner    *sqlx.DB // migration/owner role: fixtures and assertions only
	App      *sqlx.DB // hugi_app: request path, RLS enforced
	System   *sqlx.DB // hugi_system: cross-tenant platform jobs
}

// StartPostgres creates a fresh, migrated database.
func StartPostgres(t *testing.T) *Postgres {
	t.Helper()
	serverURL := os.Getenv("HUGI_TEST_POSTGRES_URL")
	if serverURL == "" {
		t.Skip("HUGI_TEST_POSTGRES_URL not set")
	}
	ctx := context.Background()
	admin, err := db.Open(ctx, serverURL, 2)
	if err != nil {
		t.Fatalf("connect test server: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	// Roles and grants are cluster-wide objects. Test packages run in parallel
	// processes, so concurrent migrations would race on them ("tuple
	// concurrently updated"). Serialize setup with an advisory lock held in the
	// shared maintenance database for the whole provisioning step.
	lockConn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockConn.ExecContext(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		t.Fatalf("acquire setup lock: %v", err)
	}
	unlock := func() {
		_, _ = lockConn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(727274)`)
		_ = lockConn.Close()
	}

	// Roles are cluster-wide: create once, idempotently, with LOGIN like compose does.
	for _, role := range []string{"hugi_app", "hugi_system"} {
		_, err := admin.ExecContext(ctx, fmt.Sprintf(`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '%[1]s') THEN
				CREATE ROLE %[1]s LOGIN PASSWORD '%[2]s';
			END IF; END $$`, role, testPassword))
		if err != nil {
			unlock()
			t.Fatalf("create role %s: %v", role, err)
		}
	}

	name := "hugi_test_" + strings.ReplaceAll(uuid.Must(uuid.NewV7()).String(), "-", "")
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		unlock()
		t.Fatalf("create database: %v", err)
	}
	ownerURL := withDB(t, serverURL, name, nil)

	pg := &Postgres{OwnerURL: ownerURL}
	func() {
		defer unlock()
		MigrateUp(t, ownerURL)
	}()
	pg.Owner = open(t, ownerURL)
	pg.App = open(t, withDB(t, serverURL, name, url.UserPassword("hugi_app", testPassword)))
	pg.System = open(t, withDB(t, serverURL, name, url.UserPassword("hugi_system", testPassword)))

	t.Cleanup(func() {
		_ = pg.Owner.Close()
		_ = pg.App.Close()
		_ = pg.System.Close()
		_, _ = admin.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return pg
}

func withDB(t *testing.T, serverURL, dbName string, user *url.Userinfo) string {
	t.Helper()
	u, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	if user != nil {
		u.User = user
	}
	return u.String()
}

func open(t *testing.T, u string) *sqlx.DB {
	t.Helper()
	d, err := db.Open(context.Background(), u, 10)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	return d
}

// MigrateUp applies all embedded migrations using the owner URL.
func MigrateUp(t *testing.T, ownerURL string) {
	t.Helper()
	p, sqlDB, err := migrate.NewProvider(ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sqlDB.Close() }()
	if _, err := p.Up(context.Background()); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
}

// CreateOrg inserts a bare organization row as the owner role (fixtures only).
func (pg *Postgres) CreateOrg(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := pg.Owner.ExecContext(context.Background(),
		`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Test Org')`, id, "t-"+strings.ReplaceAll(id.String(), "-", "")[:20])
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
}

// RedisURL returns the test Redis URL or skips.
func RedisURL(t *testing.T) string {
	t.Helper()
	u := os.Getenv("HUGI_TEST_REDIS_URL")
	if u == "" {
		t.Skip("HUGI_TEST_REDIS_URL not set")
	}
	return u
}

// WithClusterLock runs fn while holding the cluster-wide setup lock (use it for
// tests that re-run migrations, which touch shared roles and grants).
func WithClusterLock(t *testing.T, fn func()) {
	t.Helper()
	ctx := context.Background()
	admin, err := db.Open(ctx, os.Getenv("HUGI_TEST_POSTGRES_URL"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close() }()
	conn, err := admin.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(727274)`) }()
	fn()
}
