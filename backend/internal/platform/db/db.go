// Package db owns the PostgreSQL pool and the transaction helpers that bind a
// transaction to a tenant for Row-Level Security (see ADR-0002).
package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantSetting is the session variable read by RLS policies via hugi_current_org().
const TenantSetting = "app.org_id"

// Open creates a pool and verifies connectivity.
func Open(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("db: invalid database URL") // never echo: it carries credentials
	}
	if maxConns > 0 {
		cfg.MaxConns = maxConns
	}
	cfg.ConnConfig.RuntimeParams["application_name"] = "hugimethara"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

// Beginner is satisfied by *pgxpool.Pool and pgx.Tx.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// InTx runs fn inside a transaction that is NOT bound to a tenant. Use it only
// for platform-level work on tables without tenant data (e.g. the global
// connector catalog); RLS denies all tenant rows in such a transaction.
func InTx(ctx context.Context, b Beginner, fn func(pgx.Tx) error) error {
	return run(ctx, b, uuid.Nil, fn)
}

// InTenantTx runs fn in a transaction where RLS restricts every tenant table
// to orgID. The setting is transaction-local, so pooled connections never
// leak a tenant binding to the next borrower.
func InTenantTx(ctx context.Context, b Beginner, orgID uuid.UUID, fn func(pgx.Tx) error) error {
	if orgID == uuid.Nil {
		return errors.New("db: InTenantTx requires an organization id")
	}
	return run(ctx, b, orgID, fn)
}

func run(ctx context.Context, b Beginner, orgID uuid.UUID, fn func(pgx.Tx) error) (err error) {
	tx, err := b.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin: %w", err)
	}
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			panic(p)
		}
		if err != nil {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
	}()
	if orgID != uuid.Nil {
		if _, err = tx.Exec(ctx, "SELECT set_config($1, $2, true)", TenantSetting, orgID.String()); err != nil {
			return fmt.Errorf("db: bind tenant: %w", err)
		}
	}
	if err = fn(tx); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit: %w", err)
	}
	return nil
}
