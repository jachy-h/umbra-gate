package proxy

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
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
	providerCfg, ok := p.cfg.Provider(providerName)
	if !ok {
		slog.Warn("unknown provider", "provider", providerName)
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}

	remainingPath := ""
	if len(parts) > 1 {
		remainingPath = parts[1]
	}

	upstream, err := buildUpstreamURL(providerCfg.BaseURL, remainingPath, r.URL.RawQuery)
	if err != nil {
		slog.Error("invalid provider base_url", "provider", providerName, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	switch providerCfg.Type {
	case config.ProviderTypeOpenAI:
		p.handleOpenAI(w, r, providerName, &providerCfg, upstream)
	case config.ProviderTypeAnthropic:
		p.handleAnthropic(w, r, providerName, &providerCfg, upstream)
	default:
		p.handlePassthrough(w, r, providerName, &providerCfg, upstream)
	}
}

// buildUpstreamURL safely joins the provider base URL with the client-supplied
// path remainder, preserving the query string. Both inputs are tolerated with
// or without leading/trailing slashes.
func buildUpstreamURL(baseURL, remainingPath, rawQuery string) (*url.URL, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	joined := base.JoinPath(remainingPath)
	if rawQuery != "" {
		if joined.RawQuery == "" {
			joined.RawQuery = rawQuery
		} else {
			joined.RawQuery = joined.RawQuery + "&" + rawQuery
		}
	}
	return joined, nil
}

// hopByHopHeaders are stripped from forwarded requests per RFC 7230.
// The gateway also removes any client-supplied auth headers because we
// substitute them with the provider's configured credentials.
var hopByHopHeaders = map[string]struct{}{
	"Connection":          {},
	"Keep-Alive":          {},
	"Proxy-Authenticate":  {},
	"Proxy-Authorization": {},
	"Te":                  {},
	"Trailer":             {},
	"Transfer-Encoding":   {},
	"Upgrade":             {},
}

// copyForwardableHeaders copies headers from the inbound request onto the
// outbound upstream request, dropping hop-by-hop headers and stripping any
// client-provided credentials.
func copyForwardableHeaders(dst, src http.Header) {
	for key, values := range src {
		canon := http.CanonicalHeaderKey(key)
		if _, hop := hopByHopHeaders[canon]; hop {
			continue
		}
		switch canon {
		case "Authorization", "X-Api-Key", "Host", "Content-Length":
			continue
		}
		for _, v := range values {
			dst.Add(canon, v)
		}
	}
}

// copyResponseHeaders copies upstream response headers to the client,
// skipping hop-by-hop entries.
func copyResponseHeaders(dst, src http.Header) {
	for key, values := range src {
		canon := http.CanonicalHeaderKey(key)
		if _, hop := hopByHopHeaders[canon]; hop {
			continue
		}
		for _, v := range values {
			dst.Add(canon, v)
		}
	}
}
