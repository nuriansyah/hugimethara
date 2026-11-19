//go:build integration

package ratelimit_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/cache"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/ratelimit"
	"github.com/nuriansyah/hugimethara/backend/internal/platform/testsupport"
)

func TestFixedWindowLimit(t *testing.T) {
	ctx := context.Background()
	rdb, err := cache.Open(ctx, testsupport.RedisURL(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = rdb.Close() })
	l := ratelimit.New(rdb, slog.New(slog.NewTextHandler(io.Discard, nil)))
	rule := ratelimit.Rule{Name: "test-" + uuid.NewString(), Max: 3, Window: time.Minute} // unique per run: Redis may be shared

	for range 3 {
		require.NoError(t, l.Allow(ctx, rule, "k1"))
	}
	err = l.Allow(ctx, rule, "k1")
	var lim *ratelimit.Limited
	require.True(t, errors.As(err, &lim))
	require.Greater(t, lim.RetryAfterDur, time.Duration(0))
	require.LessOrEqual(t, lim.RetryAfterDur, time.Minute)
	p := lim.Problem()
	require.Equal(t, 429, p.Status)
	require.Equal(t, lim.RetryAfterDur, p.RetryAfter)

	require.NoError(t, l.Allow(ctx, rule, "k2"), "keys are independent")
}

func TestFailsOpenWhenRedisUnavailable(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 100 * time.Millisecond, MaxRetries: -1})
	l := ratelimit.New(rdb, slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, l.Allow(context.Background(), ratelimit.Rule{Name: "x", Max: 1, Window: time.Minute}, "k"))
	var nilLimiter *ratelimit.Limiter
	require.NoError(t, nilLimiter.Allow(context.Background(), ratelimit.Rule{Name: "x", Max: 1, Window: time.Minute}, "k"))
}
