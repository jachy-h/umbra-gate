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

type routeContext struct {
	AgentID       string
	ProviderName  string
	RemainingPath string
	Endpoint      string
	ProjectID     string
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
	route, ok := parseRoute(r)
	if !ok {
		http.NotFound(w, r)
		return
	}

	providerCfg, ok := p.cfg.Provider(route.ProviderName)
	if !ok {
		slog.Warn("unknown provider", "provider", route.ProviderName, "agent", route.AgentID)
		http.Error(w, "unknown provider", http.StatusNotFound)
		return
	}
	upstream, err := buildUpstreamURL(providerCfg.BaseURL, route.RemainingPath, r.URL.RawQuery)
	if err != nil {
		slog.Error("invalid provider base_url", "provider", route.ProviderName, "agent", route.AgentID, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	p.handlePassthrough(w, r, route, &providerCfg, upstream)
}

func parseRoute(r *http.Request) (routeContext, bool) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		return routeContext{}, false
	}
	route := routeContext{
		AgentID:   "unknown",
		ProjectID: strings.TrimSpace(r.Header.Get("X-Umbra-Project")),
	}
	if parts[0] == "a" {
		if len(parts) < 3 || strings.TrimSpace(parts[1]) == "" || strings.TrimSpace(parts[2]) == "" {
			return routeContext{}, false
		}
		route.AgentID = strings.TrimSpace(parts[1])
		route.ProviderName = strings.TrimSpace(parts[2])
		if len(parts) > 3 {
			route.RemainingPath = strings.Join(parts[3:], "/")
		}
	} else {
		route.ProviderName = strings.TrimSpace(parts[0])
		if len(parts) > 1 {
			route.RemainingPath = strings.Join(parts[1:], "/")
		}
	}
	route.Endpoint = route.RemainingPath
	return route, route.ProviderName != ""
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
