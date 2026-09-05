package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"tokoloop/internal/app"
	"tokoloop/internal/config"
)

func main() {
	cfg, e := config.Load()
	if e != nil {
		slog.Error("configuration error", "error", e)
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, e := app.New(ctx, cfg)
	if e != nil {
		slog.Error("startup failed", "error", e)
		os.Exit(1)
	}
	defer a.DB.Close()
	if os.Getenv("TOKO_READINESS_ONLY") != "true" {
		go a.Worker(ctx)
	}
	go func() { <-ctx.Done(); a.HTTP.Shutdown() }()
	slog.Info("Toko Loop ready", "port", cfg.Port, "ai_configured", cfg.GeminiKey != "")
	if e = a.HTTP.Listen(":" + cfg.Port); e != nil {
		slog.Error("server stopped", "error", e)
	}
}
