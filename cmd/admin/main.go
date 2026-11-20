// Command hugi-admin performs platform-operator actions that the API never
// exposes to tenants. It connects with the owner role (HUGI_ADMIN_DATABASE_URL,
// falling back to HUGI_MIGRATE_DATABASE_URL) and records every change in the
// affected organization's audit log as a "system" actor.
//
//	hugi-admin plans
//	hugi-admin show      <org-slug>
//	hugi-admin set-plan  <org-slug> <plan-key> [-status active|trialing|past_due|canceled]
//	hugi-admin override  <org-slug> <feature-key> [-limit N | -unlimited] [-disable] [-expires RFC3339] -reason TEXT
//	hugi-admin clear-override <org-slug> <feature-key> -reason TEXT
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/nuriansyah/hugimethara/internal/platform/database"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hugi-admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: hugi-admin plans | show | set-plan | override | clear-override (see package doc)")
	}
	url := os.Getenv("HUGI_ADMIN_DATABASE_URL")
	if url == "" {
		url = os.Getenv("HUGI_MIGRATE_DATABASE_URL")
	}
	if url == "" {
		return errors.New("HUGI_ADMIN_DATABASE_URL (or HUGI_MIGRATE_DATABASE_URL) is required")
	}
	d, err := db.Open(ctx, url, 2)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()

	switch args[0] {
	case "plans":
		return listPlans(ctx, d)
	case "show":
		if len(args) != 2 {
			return errors.New("usage: hugi-admin show <org-slug>")
		}
		return show(ctx, d, args[1])
	case "set-plan":
		fs := flag.NewFlagSet("set-plan", flag.ContinueOnError)
		status := fs.String("status", "active", "subscription status")
		if len(args) < 3 {
			return errors.New("usage: hugi-admin set-plan <org-slug> <plan-key> [-status S]")
		}
		if err := fs.Parse(args[3:]); err != nil {
			return err
		}
		return setPlan(ctx, d, args[1], args[2], *status)
	case "override", "clear-override":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		limit := fs.Int64("limit", -1, "limit value")
		unlimited := fs.Bool("unlimited", false, "remove the limit")
		disable := fs.Bool("disable", false, "disable the feature")
		expires := fs.String("expires", "", "RFC3339 expiry")
		reason := fs.String("reason", "", "why (required, recorded in audit)")
		if len(args) < 3 {
			return fmt.Errorf("usage: hugi-admin %s <org-slug> <feature-key> [flags]", args[0])
		}
		if err := fs.Parse(args[3:]); err != nil {
			return err
		}
		if *reason == "" {
			return errors.New("-reason is required")
		}
		if args[0] == "clear-override" {
			return clearOverride(ctx, d, args[1], args[2], *reason)
		}
		var lim any
		switch {
		case *unlimited:
			lim = nil
		case *limit >= 0:
			lim = *limit
		}
		var exp any
		if *expires != "" {
			t, err := time.Parse(time.RFC3339, *expires)
			if err != nil {
				return fmt.Errorf("-expires: %w", err)
			}
			exp = t
		}
		return setOverride(ctx, d, args[1], args[2], !*disable, lim, exp, *reason)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func orgID(ctx context.Context, tx *sqlx.Tx, slug string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.GetContext(ctx, &id, `SELECT id FROM organizations WHERE slug = $1::citext`, slug)
	if db.IsNoRows(err) {
		return id, fmt.Errorf("organization %q not found", slug)
	}
	return id, err
}

func auditSystem(ctx context.Context, tx *sqlx.Tx, org uuid.UUID, action string, details map[string]any) error {
	b, _ := json.Marshal(details)
	_, err := tx.ExecContext(ctx, `INSERT INTO audit_events (id, org_id, actor_type, action, resource_type, resource_id, details)
		VALUES ($1, $2, 'system', $3, 'subscription', $2::text, $4)`, uuid.Must(uuid.NewV7()), org, action, b)
	return err
}

func listPlans(ctx context.Context, d *sqlx.DB) error {
	var rows []struct {
		Key     string        `db:"key"`
		Name    string        `db:"name"`
		Feature string        `db:"feature_key"`
		Enabled bool          `db:"enabled"`
		Limit   sql.NullInt64 `db:"limit_value"`
	}
	if err := d.SelectContext(ctx, &rows, `SELECT p.key, p.name, pe.feature_key, pe.enabled, pe.limit_value
		FROM plans p JOIN plan_entitlements pe ON pe.plan_key = p.key ORDER BY p.sort_order, pe.feature_key`); err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "PLAN\tFEATURE\tENABLED\tLIMIT")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%v\t%s\n", r.Key, r.Feature, r.Enabled, limitStr(r.Limit))
	}
	return w.Flush()
}

func show(ctx context.Context, d *sqlx.DB, slug string) error {
	var sub struct {
		Plan   string `db:"plan_key"`
		Status string `db:"status"`
	}
	err := d.GetContext(ctx, &sub, `SELECT s.plan_key, s.status FROM org_subscriptions s
		JOIN organizations o ON o.id = s.org_id WHERE o.slug = $1::citext`, slug)
	if db.IsNoRows(err) {
		return fmt.Errorf("organization %q not found", slug)
	}
	if err != nil {
		return err
	}
	fmt.Printf("organization=%s plan=%s status=%s\n", slug, sub.Plan, sub.Status)
	var ovs []struct {
		Feature string        `db:"feature_key"`
		Enabled bool          `db:"enabled"`
		Limit   sql.NullInt64 `db:"limit_value"`
		Reason  string        `db:"reason"`
		Expires sql.NullTime  `db:"expires_at"`
	}
	if err := d.SelectContext(ctx, &ovs, `SELECT o.feature_key, o.enabled, o.limit_value, o.reason, o.expires_at
		FROM org_entitlement_overrides o JOIN organizations g ON g.id = o.org_id WHERE g.slug = $1::citext`, slug); err != nil {
		return err
	}
	for _, o := range ovs {
		exp := "never"
		if o.Expires.Valid {
			exp = o.Expires.Time.Format(time.RFC3339)
		}
		fmt.Printf("override %s enabled=%v limit=%s expires=%s reason=%q\n", o.Feature, o.Enabled, limitStr(o.Limit), exp, o.Reason)
	}
	return nil
}

func setPlan(ctx context.Context, d *sqlx.DB, slug, plan, status string) error {
	return db.InTx(ctx, d, func(tx *sqlx.Tx) error {
		org, err := orgID(ctx, tx, slug)
		if err != nil {
			return err
		}
		var prev string
		if err := tx.GetContext(ctx, &prev, `SELECT plan_key FROM org_subscriptions WHERE org_id = $1 FOR UPDATE`, org); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE org_subscriptions SET plan_key = $2, status = $3 WHERE org_id = $1`, org, plan, status); err != nil {
			if db.IsForeignKeyViolation(err) {
				return fmt.Errorf("unknown plan %q (see: hugi-admin plans)", plan)
			}
			return err
		}
		fmt.Printf("%s: %s -> %s (%s)\n", slug, prev, plan, status)
		return auditSystem(ctx, tx, org, "subscription.plan_changed", map[string]any{"from": prev, "to": plan, "status": status})
	})
}

func setOverride(ctx context.Context, d *sqlx.DB, slug, feature string, enabled bool, limit, expires any, reason string) error {
	return db.InTx(ctx, d, func(tx *sqlx.Tx) error {
		org, err := orgID(ctx, tx, slug)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO org_entitlement_overrides (org_id, feature_key, enabled, limit_value, reason, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (org_id, feature_key) DO UPDATE
			SET enabled = EXCLUDED.enabled, limit_value = EXCLUDED.limit_value, reason = EXCLUDED.reason,
			    expires_at = EXCLUDED.expires_at, created_at = now()`, org, feature, enabled, limit, reason, expires)
		if db.IsForeignKeyViolation(err) {
			return fmt.Errorf("unknown feature %q (see: hugi-admin plans)", feature)
		}
		if err != nil {
			return err
		}
		fmt.Printf("%s: override %s set\n", slug, feature)
		return auditSystem(ctx, tx, org, "subscription.override_set",
			map[string]any{"feature": feature, "enabled": enabled, "limit": limit, "expires_at": expires, "reason": reason})
	})
}

func clearOverride(ctx context.Context, d *sqlx.DB, slug, feature, reason string) error {
	return db.InTx(ctx, d, func(tx *sqlx.Tx) error {
		org, err := orgID(ctx, tx, slug)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM org_entitlement_overrides WHERE org_id = $1 AND feature_key = $2`, org, feature)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("no override for %q", feature)
		}
		fmt.Printf("%s: override %s cleared\n", slug, feature)
		return auditSystem(ctx, tx, org, "subscription.override_cleared", map[string]any{"feature": feature, "reason": reason})
	})
}

func limitStr(v sql.NullInt64) string {
	if !v.Valid {
		return "unlimited"
	}
	return fmt.Sprint(v.Int64)
}
