package proxy

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anomalyco/llm-gateway/config"
	"github.com/anomalyco/llm-gateway/db"
)

type Proxy struct {
	cfg    *config.Config
	db     *db.DB
	client *http.Client
}

func New(cfg *config.Config, database *db.DB) *Proxy {
	return &Proxy{
		cfg: cfg,
		db:  database,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	providerName := parts[0]
	providerCfg, ok := p.cfg.Providers[providerName]
	if !ok {
		slog.Warn("unknown provider", "provider", providerName)
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	remainingPath := "/"
	if len(parts) > 1 {
		remainingPath = "/" + parts[1]
	}

	target, err := url.Parse(providerCfg.BaseURL)
	if err != nil {
		slog.Error("invalid provider base_url", "provider", providerName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	switch providerCfg.Protocol {
	case "openai-compatible":
		p.handleOpenAI(w, r, providerName, &providerCfg, target, remainingPath)
	case "anthropic":
		p.handleAnthropic(w, r, providerName, &providerCfg, target, remainingPath)
	default:
		slog.Warn("unknown protocol", "protocol", providerCfg.Protocol)
		http.Error(w, "unknown protocol", http.StatusInternalServerError)
	}
}
