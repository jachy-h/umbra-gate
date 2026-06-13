package main

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

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

	slog.Info("starting server", "listen", cfg.Listen)
	if err := http.ListenAndServe(cfg.Listen, mux); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
