package services

import (
	"context"
	"log/slog"
	"time"
)

type ReportWorker struct {
	svc      *DashboardService
	log      *slog.Logger
	interval time.Duration
}

func NewReportWorker(svc *DashboardService, log *slog.Logger) *ReportWorker {
	return &ReportWorker{svc: svc, log: log, interval: 30 * time.Second}
}

func (w *ReportWorker) Run(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			n, err := w.svc.RunDueReports(ctx, 10)
			if err != nil {
				w.log.ErrorContext(ctx, "scheduled report worker tick failed", "err", err)
			} else if n > 0 {
				w.log.InfoContext(ctx, "scheduled reports fired", "count", n)
			}
		}
	}
}
