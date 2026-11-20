// Package db owns the PostgreSQL connection (sqlx over the pgx driver) and the
// transaction helpers that bind a transaction to a tenant for Row-Level
// Security (see ADR-0002). Queries live in plain .sql files (see sqlfile.go).
package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/jmoiron/sqlx"
)

// TenantSetting is the session variable read by RLS policies via hugi_current_org().
const TenantSetting = "app.org_id"

// DB is the shared connection pool.
type DB = sqlx.DB

// Tx is a transaction.
type Tx = sqlx.Tx

// Open creates a pool and verifies connectivity.
func Open(ctx context.Context, url string, maxConns int) (*sqlx.DB, error) {
	d, err := sqlx.Open("pgx", url)
	if err != nil {
		return nil, errors.New("db: invalid database URL") // never echo: it carries credentials
	}
	if maxConns <= 0 {
		maxConns = 20
	}
	d.SetMaxOpenConns(maxConns)
	d.SetMaxIdleConns(maxConns)
	d.SetConnMaxIdleTime(5 * time.Minute)
	d.SetConnMaxLifetime(time.Hour)
	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return d, nil
}

// InTx runs fn inside a transaction that is NOT bound to a tenant. Use it only
// for platform-level work (pre-tenant authentication lookups, cross-tenant
// jobs under the hugi_system role); RLS hides every tenant row from hugi_app here.
func InTx(ctx context.Context, d *sqlx.DB, fn func(*sqlx.Tx) error) error {
	return run(ctx, d, uuid.Nil, fn)
}

// InTenantTx runs fn in a transaction where RLS restricts every tenant table
// to orgID. The setting is transaction-local, so pooled connections never
// leak a tenant binding to the next borrower.
func InTenantTx(ctx context.Context, d *sqlx.DB, orgID uuid.UUID, fn func(*sqlx.Tx) error) error {
	if orgID == uuid.Nil {
		return errors.New("db: InTenantTx requires an organization id")
	}
	return run(ctx, d, orgID, fn)
}

func run(ctx context.Context, d *sqlx.DB, orgID uuid.UUID, fn func(*sqlx.Tx) error) (err error) {
	tx, err := d.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	if orgID != uuid.Nil {
		if _, err = tx.ExecContext(ctx, "SELECT set_config($1, $2, true)", TenantSetting, orgID.String()); err != nil {
			return fmt.Errorf("db: bind tenant: %w", err)
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}

// Check returns a readiness check for d.
func Check(d *sqlx.DB) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := d.PingContext(ctx); err != nil {
			return errors.New("ping failed")
		}
		return nil
	}
}
