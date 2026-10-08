// Command worker runs the AI jobs (ADR-0005). It wires config, logging, the
// store, the channel files, the AI gateway and signal handling.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/KarthikReddy8809/catalift/server/internal/ai"
	"github.com/KarthikReddy8809/catalift/server/internal/channels"
	"github.com/KarthikReddy8809/catalift/server/internal/config"
	"github.com/KarthikReddy8809/catalift/server/internal/store"
	"github.com/KarthikReddy8809/catalift/server/internal/worker"
)

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
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer st.Close()
	if err := st.RequireMigrations(ctx); err != nil {
		return err
	}

	set, err := channels.LoadDir(cfg.ChannelsDir)
	if err != nil {
		return fmt.Errorf("channels: %w", err)
	}
	for _, e := range set.Errors {
		logger.Error("channel file disabled", "file", e.File, "channel", e.ID, "reason", e.Error)
	}

	gw := ai.NewGateway(st.Pool, provider(cfg), cfg.AIModel)
	host, _ := os.Hostname() // an empty name only makes the claim less readable
	logger.Info("worker started", "provider", gw.Provider(), "channels", set.IDs(), "concurrency", worker.Concurrency)
	worker.New(st.Pool, gw, channels.NewRegistry(set), logger, fmt.Sprintf("%s:%d", host, os.Getpid())).Run(ctx)
	logger.Info("worker stopped")
	return nil
}

// provider is OpenRouter when a key is set, else the free local stand-in.
func provider(cfg config.Config) ai.Provider {
	if cfg.OpenRouterKey != "" {
		return ai.NewOpenRouter(cfg.OpenRouterKey, cfg.AIModel)
	}
	return ai.Local{}
}
