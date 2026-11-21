package bus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// WorkConsumer describes a durable pull consumer on a work-queue stream.
type WorkConsumer struct {
	Stream      string
	Durable     string
	Filter      string
	MaxInflight int           // concurrent handlers; default 16
	AckWait     time.Duration // must exceed the longest handler run; default 6m
	RetryDelay  time.Duration // redelivery delay after a handler error; default 5s
	// DeliverNew starts a new durable at the end of the stream instead of
	// replaying retained history (limits streams such as HUGI_EVENTS).
	DeliverNew bool
}

// Handler processes one message payload. A nil error acknowledges it; an
// error asks JetStream to redeliver after RetryDelay. Handlers must be
// idempotent: delivery is at-least-once.
type Handler func(ctx context.Context, data []byte) error

// Message is a delivered message with its routing metadata.
type Message struct {
	Subject string
	Data    []byte
	Header  map[string][]string
}

// MessageHandler is a Handler that also sees the subject and headers (e.g.
// Hugi-Org-Id and Nats-Msg-Id set by the outbox relay).
type MessageHandler func(ctx context.Context, m Message) error

// Consume runs the consumer until ctx is cancelled, then waits for in-flight
// handlers to finish (they see ctx cancelled and should return promptly).
func (b *Bus) Consume(ctx context.Context, wc WorkConsumer, log *slog.Logger, handle Handler) error {
	return b.ConsumeMessages(ctx, wc, log, func(ctx context.Context, m Message) error { return handle(ctx, m.Data) })
}

// ConsumeMessages is Consume with access to subject and headers.
func (b *Bus) ConsumeMessages(ctx context.Context, wc WorkConsumer, log *slog.Logger, handle MessageHandler) error {
	if wc.MaxInflight <= 0 {
		wc.MaxInflight = 16
	}
	if wc.AckWait <= 0 {
		wc.AckWait = 6 * time.Minute
	}
	if wc.RetryDelay <= 0 {
		wc.RetryDelay = 5 * time.Second
	}
	deliver := jetstream.DeliverAllPolicy
	if wc.DeliverNew {
		deliver = jetstream.DeliverNewPolicy
	}
	cons, err := b.JS.CreateOrUpdateConsumer(ctx, wc.Stream, jetstream.ConsumerConfig{
		Durable:       wc.Durable,
		DeliverPolicy: deliver,
		FilterSubject: wc.Filter,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       wc.AckWait,
		MaxAckPending: wc.MaxInflight * 2,
		MaxDeliver:    -1,
	})
	if err != nil {
		return fmt.Errorf("bus: consumer %s: %w", wc.Durable, err)
	}
	sem := make(chan struct{}, wc.MaxInflight)
	var wg sync.WaitGroup
	cc, err := cons.Consume(func(m jetstream.Msg) {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() { <-sem; wg.Done() }()
			if err := handle(ctx, Message{Subject: m.Subject(), Data: m.Data(), Header: m.Headers()}); err != nil {
				if ctx.Err() == nil {
					log.WarnContext(ctx, "message handler failed; will redeliver", "consumer", wc.Durable,
						"subject", m.Subject(), "err", err)
				}
				_ = m.NakWithDelay(wc.RetryDelay)
				return
			}
			_ = m.Ack()
		}()
	})
	if err != nil {
		return fmt.Errorf("bus: consume %s: %w", wc.Durable, err)
	}
	<-ctx.Done()
	cc.Stop()
	wg.Wait()
	return nil
}
