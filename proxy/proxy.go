package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/anomalyco/llm-gateway/config"
	"github.com/anomalyco/llm-gateway/db"
)

type Proxy struct {
	cfg *config.Config
	db  *db.DB
}

func New(cfg *config.Config, database *db.DB) *Proxy {
	return &Proxy{cfg: cfg, db: database}
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

func (p *Proxy) handleAnthropic(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, target *url.URL, path string) {
	http.Error(w, "anthropic protocol not yet implemented", http.StatusNotImplemented)
}

func (p *Proxy) singleHostReverseProxy(target *url.URL) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.Host = target.Host
	}
	return proxy
}
