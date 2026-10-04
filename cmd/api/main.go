// Command api runs the CataliftApp HTTP service. It wires config, logging,
// telemetry, the store, the HTTP server and signal handling, and nothing else.
package main

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
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/KarthikReddy8809/catalift/internal/auth"
	"github.com/KarthikReddy8809/catalift/internal/catalogue"
	"github.com/KarthikReddy8809/catalift/internal/channels"
	"github.com/KarthikReddy8809/catalift/internal/config"
	"github.com/KarthikReddy8809/catalift/internal/exports"
	"github.com/KarthikReddy8809/catalift/internal/generation"
	"github.com/KarthikReddy8809/catalift/internal/health"
	"github.com/KarthikReddy8809/catalift/internal/httpapi"
	"github.com/KarthikReddy8809/catalift/internal/listings"
	"github.com/KarthikReddy8809/catalift/internal/store"
	"github.com/KarthikReddy8809/catalift/internal/telemetry"
)

var version = "dev"

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Tracing: a no-op until OTEL_EXPORTER_OTLP_ENDPOINT is set.
	shutdownTracing, err := telemetry.Setup(ctx, "catalift-app", version)
	if err != nil {
		return fmt.Errorf("telemetry: %w", err)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			logger.Warn("tracing shutdown", "err", err)
		}
	}()

	// With no DATABASE_URL the service runs health routes only; /v1 needs the store.
	var checkers []health.Checker
	var deps *httpapi.Deps
	if cfg.DatabaseURL != "" {
		st, err := store.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return fmt.Errorf("store: %w", err)
		}
		defer st.Close()
		checkers = append(checkers, st)
		deps, err = buildDeps(ctx, cfg, st, logger)
		if err != nil {
			return err
		}
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector(), collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewWithAPI(logger, version, reg, deps, checkers...),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      120 * time.Second, // photo batches of up to 200 MB
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", srv.Addr, "version", version)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve: %w", err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("stopped")
	return nil
}

// buildDeps loads the channel files, re-checks listings whose channel file
// changed (D7) and builds the services the /v1 routes call.
func buildDeps(ctx context.Context, cfg config.Config, st *store.Store, logger *slog.Logger) (*httpapi.Deps, error) {
	set, err := channels.LoadDir(cfg.ChannelsDir)
	if err != nil {
		return nil, fmt.Errorf("channels: %w", err)
	}
	for _, e := range set.Errors {
		logger.Error("channel file disabled", "file", e.File, "channel", e.ID, "reason", e.Error)
	}
	ls := listings.NewService(st.Pool, set)
	if err := ls.Recheck(ctx); err != nil {
		return nil, fmt.Errorf("recheck listings: %w", err)
	}
	return &httpapi.Deps{
		Auth:          auth.NewService(store.New(st.Pool)),
		SignInLimiter: auth.NewLimiter(10, 15*time.Minute, time.Now),
		Catalogue:     catalogue.NewService(st.Pool, cfg.DataDir),
		Generation:    generation.NewService(st.Pool, set),
		Listings:      ls,
		Exports:       exports.NewService(st.Pool, set, cfg.DataDir),
		Channels:      set,
		SecureCookies: cfg.SecureCookies,
	}, nil
}
