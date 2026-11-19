//go:build integration

package db_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/migrate"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/testsupport"
)

func TestTenantIsolationEnforcedByRLS(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	ctx := context.Background()
	orgA, orgB := uuid.New(), uuid.New()
	pg.CreateOrg(t, orgA)
	pg.CreateOrg(t, orgB)

	publish := func(org, target uuid.UUID) error {
		return db.InTenantTx(ctx, pg.App, org, func(tx *sqlx.Tx) error {
			_, err := event.Publish(ctx, tx, event.Event{OrgID: target, Subject: "hugi.events.test.created", Payload: map[string]string{"k": "v"}})
			return err
		})
	}
	count := func(org uuid.UUID) int {
		var n int
		err := db.InTenantTx(ctx, pg.App, org, func(tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &n, "SELECT count(*) FROM outbox")
		})
		require.NoError(t, err)
		return n
	}

	require.NoError(t, publish(orgA, orgA))
	require.NoError(t, publish(orgB, orgB))

	// A tenant-bound transaction cannot write another tenant's rows...
	require.Error(t, publish(orgA, orgB), "RLS WITH CHECK must reject cross-tenant insert")
	// ...nor see them.
	require.Equal(t, 1, count(orgA))
	require.Equal(t, 1, count(orgB))

	// A transaction not bound to any tenant sees and writes nothing.
	var n int
	require.NoError(t, db.InTx(ctx, pg.App, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &n, "SELECT count(*) FROM outbox")
	}))
	require.Zero(t, n)
	require.Error(t, db.InTx(ctx, pg.App, func(tx *sqlx.Tx) error {
		_, err := event.Publish(ctx, tx, event.Event{OrgID: orgA, Subject: "hugi.events.test.created", Payload: 1})
		return err
	}))

	// The tenant binding is transaction-local: a later unbound tx on the same
	// pool (likely the same connection) must not inherit it.
	require.NoError(t, db.InTx(ctx, pg.App, func(tx *sqlx.Tx) error {
		return tx.GetContext(ctx, &n, "SELECT count(*) FROM outbox")
	}))
	require.Zero(t, n)

	// The runtime role has no UPDATE/DELETE on the outbox at all.
	err := db.InTenantTx(ctx, pg.App, orgA, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE outbox SET sent_at = now()")
		return err
	})
	require.ErrorContains(t, err, "permission denied")
}

func TestRuntimeRolesCannotBypassRLS(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	ctx := context.Background()
	for _, pool := range []*sqlx.DB{pg.App, pg.System} {
		var bypass, super bool
		require.NoError(t, pool.QueryRowContext(ctx,
			"SELECT rolbypassrls, rolsuper FROM pg_roles WHERE rolname = current_user").Scan(&bypass, &super))
		require.False(t, bypass)
		require.False(t, super)
	}
}

func TestMigrationsRoundTripAndReadiness(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	ctx := context.Background()
	require.NoError(t, migrate.Check(pg.App)(ctx))

	p, sqlDB, err := migrate.NewProvider(pg.OwnerURL)
	require.NoError(t, err)
	defer func() { _ = sqlDB.Close() }()
	testsupport.WithClusterLock(t, func() {
		_, err = p.DownTo(ctx, 0)
		require.NoError(t, err)
		require.Error(t, migrate.Check(pg.App)(ctx), "readiness must fail when schema is behind")
		_, err = p.Up(ctx)
		require.NoError(t, err)
	})
	require.NoError(t, migrate.Check(pg.App)(ctx))
}
