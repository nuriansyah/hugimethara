// Package cache owns the Redis client. Redis holds caches, rate-limit buckets
// and version stamps only — never source-of-truth data.
package cache

import (
	"context"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Open parses the URL, creates a client and pings it.
func Open(ctx context.Context, url string) (*redis.Client, error) {
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, errors.New("cache: invalid redis URL")
	}
	c := redis.NewClient(opt)
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("cache: ping: %w", err)
	}
	return c, nil
}

// Check returns a readiness check for the client.
func Check(c *redis.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := c.Ping(ctx).Err(); err != nil {
			return errors.New("ping failed")
		}
		return nil
	}
}
