//go:build integration

package event_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/event"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/bus"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/db"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/logging"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/testsupport"
)

func TestRelayPublishesCommittedEventsExactlyOncePerRow(t *testing.T) {
	pg := testsupport.StartPostgres(t)
	natsURL := testsupport.StartNATS(t)
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	b, err := bus.Connect(natsURL, "test", log)
	require.NoError(t, err)
	t.Cleanup(b.Close)
	require.NoError(t, b.EnsureStreams(ctx))
	require.NoError(t, b.EnsureStreams(ctx), "stream provisioning must be idempotent")

	org := uuid.New()
	pg.CreateOrg(t, org)
	const total = 120
	reqCtx := logging.WithRequestID(ctx, "req-relay-test")
	require.NoError(t, db.InTenantTx(reqCtx, pg.App, org, func(tx *sqlx.Tx) error {
		for i := range total {
			if _, err := event.Publish(reqCtx, tx, event.Event{
				OrgID: org, Subject: "hugi.events.test.created", Payload: map[string]int{"n": i},
			}); err != nil {
				return err
			}
		}
		return nil
	}))
	// A rolled-back transaction must never be relayed.
	_ = db.InTenantTx(ctx, pg.App, org, func(tx *sqlx.Tx) error {
		_, _ = event.Publish(ctx, tx, event.Event{OrgID: org, Subject: "hugi.events.test.rolledback", Payload: 0})
		return io.ErrUnexpectedEOF
	})

	// Two relays race over the same rows.
	r1 := event.NewRelay(pg.System, b.JS, prometheus.NewRegistry(), log, event.RelayOptions{BatchSize: 25})
	r2 := event.NewRelay(pg.System, b.JS, prometheus.NewRegistry(), log, event.RelayOptions{BatchSize: 25})
	var wg sync.WaitGroup
	for _, r := range []*event.Relay{r1, r2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				n, err := r.RelayOnce(ctx)
				require.NoError(t, err)
				if n == 0 {
					return
				}
			}
		}()
	}
	wg.Wait()

	stream, err := b.JS.Stream(ctx, "HUGI_EVENTS")
	require.NoError(t, err)
	info, err := stream.Info(ctx)
	require.NoError(t, err)
	require.EqualValues(t, total, info.State.Msgs, "each committed row published once")

	var pending int
	require.NoError(t, pg.System.GetContext(ctx, &pending, "SELECT count(*) FROM outbox WHERE sent_at IS NULL"))
	require.Zero(t, pending)

	// Headers carry tenant and correlation for consumers.
	cons, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{AckPolicy: jetstream.AckExplicitPolicy})
	require.NoError(t, err)
	msgs, err := cons.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	require.NoError(t, err)
	var got int
	for m := range msgs.Messages() {
		got++
		require.Equal(t, org.String(), m.Headers().Get("Hugi-Org-Id"))
		require.Equal(t, "req-relay-test", m.Headers().Get("X-Request-Id"))
		require.Equal(t, "hugi.events.test.created", m.Subject())
	}
	require.Equal(t, 1, got)

	// Nothing left: another pass is a no-op.
	n, err := r1.RelayOnce(ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}
