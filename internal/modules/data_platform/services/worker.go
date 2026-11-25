package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	model "github.com/nuriansyah/hugimethara/internal/modules/data_platform/model"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/commands"
	"github.com/nuriansyah/hugimethara/internal/modules/data_platform/repository/queries"
	secretsmodel "github.com/nuriansyah/hugimethara/internal/modules/integration/secrets/model"
	db "github.com/nuriansyah/hugimethara/internal/platform/database"
	"github.com/nuriansyah/hugimethara/internal/platform/principal"
)

// IngestionWorker runs due ingestion pipelines on a timer. It is a separate
// process from the API and uses the hugi_system pool only to claim work.
type IngestionWorker struct {
	pool     *sqlx.DB
	system   *sqlx.DB
	svc      *DataPlatformService
	log      *slog.Logger
	interval time.Duration
}

func NewIngestionWorker(pool, system *sqlx.DB, svc *DataPlatformService, log *slog.Logger) *IngestionWorker {
	return &IngestionWorker{pool: pool, system: system, svc: svc, log: log, interval: 30 * time.Second}
}

// Run blocks until ctx is cancelled.
func (w *IngestionWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			n, err := w.RunDue(ctx, 10)
			if err != nil {
				w.log.ErrorContext(ctx, "ingestion worker tick failed", "err", err)
			} else if n > 0 {
				w.log.InfoContext(ctx, "ingestion worker ran pipelines", "count", n)
			}
		}
	}
}

// RunDue claims and executes up to limit due pipelines. Claimed pipelines are
// pushed two minutes out so a second worker cannot pick them up mid-run.
func (w *IngestionWorker) RunDue(ctx context.Context, limit int) (int, error) {
	if limit <= 0 || limit > 100 {
		limit = 10
	}
	var pipelines []repository.PipelineRow
	claimUntil := time.Now().UTC().Add(2 * time.Minute)
	if err := db.InTx(ctx, w.system, func(tx *sqlx.Tx) error {
		if err := tx.SelectContext(ctx, &pipelines, queries.SQL.Get("DuePipelines"), limit); err != nil {
			return err
		}
		for _, pipeline := range pipelines {
			if _, err := tx.ExecContext(ctx, commands.SQL.Get("ClaimPipeline"), pipeline.ID, claimUntil); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return 0, err
	}
	ran := 0
	for _, row := range pipelines {
		if err := w.runOne(ctx, row); err != nil {
			w.log.ErrorContext(ctx, "pipeline run failed", "pipeline_id", row.ID, "err", err)
			continue
		}
		ran++
	}
	return ran, nil
}

func (w *IngestionWorker) runOne(ctx context.Context, row repository.PipelineRow) error {
	pipeline := row.Model()
	p := principal.Principal{Type: "system", OrgID: row.OrgID}
	var source model.DataSource
	var secret []byte
	var before string
	if err := db.InTenantTx(ctx, w.pool, row.OrgID, func(tx *sqlx.Tx) error {
		src, err := w.svc.loadDataSource(ctx, tx, pipeline.DataSourceID)
		if err != nil {
			return err
		}
		source = src
		if src.SecretID != nil {
			if w.svc.opener == nil {
				return errSecretResolver
			}
			secret, err = w.svc.opener.Open(ctx, tx, row.OrgID, *src.SecretID)
			if err != nil {
				return err
			}
		}
		var runRow repository.RunRow
		if err := tx.GetContext(ctx, &runRow, queries.SQL.Get("LatestSuccessfulRun"), pipeline.ID); err == nil {
			before = runRow.WatermarkAfter
		}
		return nil
	}); err != nil {
		return err
	}
	defer secretsmodel.Zero(secret)
	runID := uuid.New()
	if err := db.InTenantTx(ctx, w.pool, row.OrgID, func(tx *sqlx.Tx) error {
		if _, err := tx.ExecContext(ctx, commands.SQL.Get("InsertRun"), row.OrgID, runID, pipeline.ID, pipeline.Mode, before, nil); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, commands.SQL.Get("StartRun"), runID)
		return err
	}); err != nil {
		return err
	}
	_, err := w.svc.executePipeline(ctx, p, pipeline, source, secret, runID, before)
	return err
}
