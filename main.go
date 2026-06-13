package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/anomalyco/llm-gateway/api"
	"github.com/anomalyco/llm-gateway/config"
	"github.com/anomalyco/llm-gateway/dashboard"
	"github.com/anomalyco/llm-gateway/db"
	"github.com/anomalyco/llm-gateway/proxy"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	execDir, err := os.Getwd()
	if err != nil {
		slog.Error("failed to get working directory", "error", err)
		os.Exit(1)
	}

	configPath := filepath.Join(execDir, "config.yaml")
	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "path", configPath, "error", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(execDir, "router.db")
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "path", dbPath, "error", err)
		os.Exit(1)
	}
	defer database.Close()

	proxyHandler := proxy.New(cfg, database)
	apiHandler := api.New(database)
	dashHandler := dashboard.New(database)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiHandler))
	mux.Handle("/api", apiHandler)
	mux.Handle("/dashboard", dashHandler)
	mux.Handle("/dashboard/", dashHandler)
	mux.Handle("/", proxyHandler)

	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: mux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "listen", cfg.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}
}
