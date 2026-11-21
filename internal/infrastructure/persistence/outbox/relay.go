package outbox

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	db "github.com/nuriansyah/hugimethara/internal/platform/database"
)

// Publisher is the subset of jetstream.JetStream the relay needs.
type Publisher interface {
	PublishMsg(ctx context.Context, msg *nats.Msg, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
}

// RelayOptions tunes the relay loop.
type RelayOptions struct {
	BatchSize    int32         // default 100
	PollInterval time.Duration // idle wait between empty polls; default 500ms
}

// Relay drains the outbox to JetStream. Several relays may run concurrently:
// rows are claimed with FOR UPDATE SKIP LOCKED, and the JetStream message id
// (= outbox id) lets the stream's duplicate window drop re-publishes after a
// crash between publish and commit.
type Relay struct {
	pool      *sqlx.DB // connected as hugi_system
	pub       Publisher
	opts      RelayOptions
	log       *slog.Logger
	published *prometheus.CounterVec
	pending   prometheus.Gauge
}

func NewRelay(pool *sqlx.DB, pub Publisher, reg prometheus.Registerer, log *slog.Logger, o RelayOptions) *Relay {
	if o.BatchSize <= 0 {
		o.BatchSize = 100
	}
	if o.PollInterval <= 0 {
		o.PollInterval = 500 * time.Millisecond
	}
	r := &Relay{
		pool: pool, pub: pub, opts: o, log: log,
		published: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "hugi_outbox_published_total", Help: "Outbox rows relayed, by result.",
		}, []string{"result"}),
		pending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "hugi_outbox_pending", Help: "Outbox rows not yet published.",
		}),
	}
	reg.MustRegister(r.published, r.pending)
	return r
}

// Run loops until ctx is cancelled.
func (r *Relay) Run(ctx context.Context) error {
	gaugeTick := time.NewTicker(15 * time.Second)
	defer gaugeTick.Stop()
	r.updatePending(ctx)
	for {
		n, err := r.RelayOnce(ctx)
		if err != nil && ctx.Err() == nil {
			r.log.ErrorContext(ctx, "outbox relay batch failed", "err", err)
		}
		if n == int(r.opts.BatchSize) && err == nil {
			continue // more work likely waiting
		}
		select {
		case <-ctx.Done():
			return nil
		case <-gaugeTick.C:
			r.updatePending(ctx)
		case <-time.After(r.opts.PollInterval):
		}
	}
}

// RelayOnce publishes one batch and returns how many rows were claimed.
func (r *Relay) RelayOnce(ctx context.Context) (int, error) {
	var claimed int
	err := db.InTx(ctx, r.pool, func(tx *sqlx.Tx) error {
		var rows []outboxRow
		if err := tx.SelectContext(ctx, &rows, q.Get("ClaimPendingOutbox"), r.opts.BatchSize); err != nil {
			return err
		}
		claimed = len(rows)
		sent := make([]int64, 0, len(rows))
		for _, row := range rows {
			if err := r.publish(ctx, row); err != nil {
				r.published.WithLabelValues("error").Inc()
				r.log.WarnContext(ctx, "outbox publish failed", "outbox_id", row.ID, "subject", row.Subject, "err", err)
				if _, err := tx.ExecContext(ctx, q.Get("MarkOutboxFailed"), row.ID, truncate(err.Error(), 500)); err != nil {
					return err
				}
				continue
			}
			r.published.WithLabelValues("ok").Inc()
			sent = append(sent, row.ID)
		}
		if len(sent) > 0 {
			_, err := tx.ExecContext(ctx, q.Get("MarkOutboxSent"), sent)
			return err
		}
		return nil
	})
	return claimed, err
}

func (r *Relay) publish(ctx context.Context, row outboxRow) error {
	msg := nats.NewMsg(row.Subject)
	msg.Data = row.Payload
	var hdr map[string]string
	if len(row.Headers) > 0 {
		_ = json.Unmarshal(row.Headers, &hdr)
	}
	for k, v := range hdr {
		msg.Header.Set(k, v)
	}
	msg.Header.Set("Hugi-Org-Id", row.OrgID)
	id := strconv.FormatInt(row.ID, 10)
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := r.pub.PublishMsg(pctx, msg, jetstream.WithMsgID("outbox-"+id))
	return err
}

func (r *Relay) updatePending(ctx context.Context) {
	var n int64
	if err := r.pool.GetContext(ctx, &n, q.Get("CountPendingOutbox")); err == nil {
		r.pending.Set(float64(n))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
