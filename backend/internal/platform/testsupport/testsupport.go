// Package testsupport starts real dependencies in containers for integration
// tests (build tag "integration"). It provisions Postgres exactly like
// deploy/compose does: login roles first, then the embedded migrations.
package testsupport

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/migrate"
)

const (
	PostgresImage = "postgres:18.7-alpine"
	NATSImage     = "nats:2.15.3-alpine"
	RedisImage    = "redis:8.8.4-alpine"
	testPassword  = "test-only-password"
)

// Postgres is a migrated database with one pool per runtime role.
type Postgres struct {
	OwnerURL string
	Owner    *pgxpool.Pool // migration/owner role: fixtures and assertions only
	App      *pgxpool.Pool // hugi_app: request path, RLS enforced
	System   *pgxpool.Pool // hugi_system: cross-tenant platform jobs
}

// StartPostgres starts a container, creates login roles, runs migrations and
// opens pools. Everything is torn down with t.Cleanup.
func StartPostgres(t *testing.T) *Postgres {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, PostgresImage,
		tcpostgres.WithDatabase("hugimethara"),
		tcpostgres.WithUsername("hugi_owner"),
		tcpostgres.WithPassword(testPassword),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60*time.Second)),
	)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	ownerURL, err := c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}

	owner, err := db.Open(ctx, ownerURL, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"hugi_app", "hugi_system"} {
		if _, err := owner.Exec(ctx, fmt.Sprintf("CREATE ROLE %s LOGIN PASSWORD '%s'", role, testPassword)); err != nil {
			t.Fatalf("create role %s: %v", role, err)
		}
	}
	owner.Close()

	MigrateUp(t, ownerURL)

	pg := &Postgres{OwnerURL: ownerURL}
	pg.Owner, err = db.Open(ctx, ownerURL, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pg.Owner.Close)
	pg.App = openAs(t, ownerURL, "hugi_app")
	pg.System = openAs(t, ownerURL, "hugi_system")
	return pg
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

func openAs(t *testing.T, ownerURL, role string) *pgxpool.Pool {
	t.Helper()
	u, err := url.Parse(ownerURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, testPassword)
	pool, err := db.Open(context.Background(), u.String(), 10)
	if err != nil {
		t.Fatalf("open pool as %s: %v", role, err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// StartNATS starts a JetStream-enabled NATS server and returns its URL.
func StartNATS(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := tcnats.Run(ctx, NATSImage, testcontainers.WithCmdArgs("-js"))
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("start nats: %v", err)
	}
	u, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// CreateOrg inserts a bare organization row as the owner role (fixtures only).
func (pg *Postgres) CreateOrg(t *testing.T, id uuid.UUID) {
	t.Helper()
	_, err := pg.Owner.Exec(context.Background(),
		`INSERT INTO organizations (id, slug, name) VALUES ($1, $2, 'Test Org')`, id, "t-"+id.String()[:8]+id.String()[24:])
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
}

// StartRedis starts Redis and returns its URL.
func StartRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	c, err := tcredis.Run(ctx, RedisImage)
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("start redis: %v", err)
	}
	u, err := c.ConnectionString(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
