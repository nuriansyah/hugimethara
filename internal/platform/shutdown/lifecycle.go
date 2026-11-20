// Package lifecycle runs a binary's long-lived components and shuts them down
// gracefully on SIGINT/SIGTERM.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"
)

// SignalContext is cancelled on SIGINT or SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

// Component is a blocking unit of work that returns when ctx is cancelled.
type Component func(ctx context.Context) error

// HTTPServer returns a Component serving h on addr with production timeouts.
func HTTPServer(name, addr string, h http.Handler, shutdownTimeout time.Duration, log *slog.Logger) Component {
	return func(ctx context.Context) error {
		srv := &http.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       60 * time.Second,
			WriteTimeout:      90 * time.Second,
			IdleTimeout:       120 * time.Second,
			MaxHeaderBytes:    1 << 20,
		}
		errc := make(chan error, 1)
		go func() {
			log.Info("http server listening", "server", name, "addr", addr)
			errc <- srv.ListenAndServe()
		}()
		select {
		case err := <-errc:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("%s: %w", name, err)
		case <-ctx.Done():
			sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
			defer cancel()
			log.Info("http server shutting down", "server", name)
			return srv.Shutdown(sctx)
		}
	}
}

// MetricsServer serves /metrics from reg on addr.
func MetricsServer(addr string, reg *prometheus.Registry, shutdownTimeout time.Duration, log *slog.Logger) Component {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{Registry: reg}))
	return HTTPServer("metrics", addr, mux, shutdownTimeout, log)
}

// Run starts all components and waits. The first component error cancels the
// others; Run returns that error (nil on clean signal-driven shutdown).
func Run(ctx context.Context, components ...Component) error {
	g, gctx := errgroup.WithContext(ctx)
	for _, c := range components {
		g.Go(func() error { return c(gctx) })
	}
	return g.Wait()
}
