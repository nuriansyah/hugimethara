// Command hugi-migrate applies the embedded database migrations.
//
//	hugi-migrate up        apply all pending migrations
//	hugi-migrate down      roll back the most recent migration (refused in production)
//	hugi-migrate status    list migrations and whether they are applied
//	hugi-migrate version   print the current and expected versions
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/nuriansyah/hugimethara/internal/platform/config"
	"github.com/nuriansyah/hugimethara/internal/platform/logger"
	"github.com/nuriansyah/hugimethara/internal/platform/migrate"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hugi-migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: hugi-migrate up|down|status|version")
	}
	cfg, err := config.Load("hugi-migrate", config.NeedMigrateDatabase)
	if err != nil {
		return err
	}
	log := logging.New(os.Stdout, cfg.Log.Level, cfg.Log.Format, cfg.ServiceName, cfg.Env)
	p, sqlDB, err := migrate.NewProvider(cfg.MigrateDatabaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = sqlDB.Close() }()
	ctx := context.Background()

	switch args[0] {
	case "up":
		res, err := p.Up(ctx)
		for _, r := range res {
			log.Info("applied", "version", r.Source.Version, "file", r.Source.Path, "duration_ms", r.Duration.Milliseconds())
		}
		if err != nil {
			return err
		}
		v, _ := p.GetDBVersion(ctx)
		log.Info("database up to date", "version", v, "applied_now", len(res))
	case "down":
		if cfg.IsProduction() {
			return errors.New("down is disabled in production; write a forward migration instead")
		}
		r, err := p.Down(ctx)
		if err != nil {
			return err
		}
		log.Info("rolled back", "version", r.Source.Version, "file", r.Source.Path)
	case "status":
		st, err := p.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range st {
			fmt.Printf("%-6d %-10s %s\n", s.Source.Version, s.State, s.Source.Path)
		}
	case "version":
		cur, err := p.GetDBVersion(ctx)
		if err != nil {
			return err
		}
		want, err := migrate.ExpectedVersion()
		if err != nil {
			return err
		}
		fmt.Printf("database=%d expected=%d\n", cur, want)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}
