// Package bus wraps the NATS JetStream connection and declares the platform
// streams (ADR-0004). Business code publishes through the outbox, never directly.
package bus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// Stream definitions. Subjects carry the org id as the 4th token for
// per-tenant observability, e.g. hugi.wf.step.<org_id>.
var Streams = []jetstream.StreamConfig{
	{
		Name:        "HUGI_WF",
		Description: "Workflow step dispatch (work queue)",
		Subjects:    []string{"hugi.wf.>"},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.FileStorage,
		Duplicates:  10 * time.Minute,
	},
	{
		Name:        "HUGI_WEBHOOK",
		Description: "Outbound webhook delivery (work queue)",
		Subjects:    []string{"hugi.webhook.>"},
		Retention:   jetstream.WorkQueuePolicy,
		Storage:     jetstream.FileStorage,
		Duplicates:  10 * time.Minute,
	},
	{
		Name:        "HUGI_EVENTS",
		Description: "Platform domain events",
		Subjects:    []string{"hugi.events.>"},
		Retention:   jetstream.LimitsPolicy,
		MaxAge:      7 * 24 * time.Hour,
		Storage:     jetstream.FileStorage,
		Duplicates:  10 * time.Minute,
	},
}

// Bus holds the NATS connection and JetStream context.
type Bus struct {
	Conn *nats.Conn
	JS   jetstream.JetStream
}

// Connect dials NATS with reconnect enabled.
func Connect(url, name string, log *slog.Logger) (*Bus, error) {
	nc, err := nats.Connect(url,
		nats.Name(name),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				log.Warn("nats disconnected", "err", err)
			}
		}),
		nats.ReconnectHandler(func(*nats.Conn) { log.Info("nats reconnected") }),
	)
	if err != nil {
		return nil, fmt.Errorf("bus: connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("bus: jetstream: %w", err)
	}
	return &Bus{Conn: nc, JS: js}, nil
}

// EnsureStreams creates or updates the platform streams. Idempotent.
func (b *Bus) EnsureStreams(ctx context.Context) error {
	for _, cfg := range Streams {
		if _, err := b.JS.CreateOrUpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("bus: ensure stream %s: %w", cfg.Name, err)
		}
	}
	return nil
}

// Check is a readiness check: connected and JetStream reachable.
func (b *Bus) Check(ctx context.Context) error {
	if !b.Conn.IsConnected() {
		return errors.New("not connected")
	}
	if _, err := b.JS.AccountInfo(ctx); err != nil {
		return errors.New("jetstream unavailable")
	}
	return nil
}

// Close drains the connection.
func (b *Bus) Close() {
	if b != nil && b.Conn != nil {
		_ = b.Conn.Drain()
	}
}
