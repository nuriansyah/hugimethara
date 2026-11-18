// Package ratelimit implements fixed-window rate limits in Redis.
package ratelimit

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nuriansyah/hugimethara/backend/internal/platform/httpx"
)

// Rule is a limit of Max events per Window.
type Rule struct {
	Name   string
	Max    int64
	Window time.Duration
}

// Limiter checks rules against Redis counters.
type Limiter struct {
	rdb *redis.Client
	log *slog.Logger
}

func New(rdb *redis.Client, log *slog.Logger) *Limiter { return &Limiter{rdb: rdb, log: log} }

// script increments the counter and sets the TTL atomically on first hit.
var script = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return {n, redis.call('PTTL', KEYS[1])}
`)

// Allow records one event for key under rule. It returns a *Limited error when
// the limit is exceeded. If Redis is unavailable it fails open (logged):
// availability of sign-in must not depend on the cache, and per-account
// lockout in Postgres still bounds password guessing.
func (l *Limiter) Allow(ctx context.Context, rule Rule, key string) error {
	if l == nil || l.rdb == nil {
		return nil
	}
	k := fmt.Sprintf("rl:%s:%d:%s", rule.Name, time.Now().UnixMilli()/rule.Window.Milliseconds(), key)
	res, err := script.Run(ctx, l.rdb, []string{k}, rule.Window.Milliseconds()).Int64Slice()
	if err != nil {
		l.log.WarnContext(ctx, "rate limiter unavailable, failing open", "rule", rule.Name, "err", err)
		return nil
	}
	if res[0] > rule.Max {
		retry := time.Duration(res[1]) * time.Millisecond
		if retry <= 0 {
			retry = rule.Window
		}
		return &Limited{RetryAfterDur: retry}
	}
	return nil
}

// Limited is returned when a rate limit is exceeded; it renders as HTTP 429.
type Limited struct{ RetryAfterDur time.Duration }

func (e *Limited) Error() string { return "rate limited" }

// Problem converts the error to its HTTP representation.
func (e *Limited) Problem() *httpx.Problem {
	p := httpx.NewProblem(http.StatusTooManyRequests, "rate_limited", "Too many requests",
		"Too many attempts. Try again in "+strconv.Itoa(int(e.RetryAfterDur.Seconds())+1)+" seconds.")
	p.RetryAfter = e.RetryAfterDur
	return p
}
