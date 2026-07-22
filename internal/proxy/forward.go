package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

type Forwarder struct {
	DB    *db.DB
	Stats *stats.Service
}

// Handle implements the OpenAI Chat Completions compatible proxy for a link.
func (f *Forwarder) Handle(w http.ResponseWriter, r *http.Request, link models.ProxyLink) {
	body, _ := io.ReadAll(r.Body)
	origModel := extractModel(body)
	attributes := parseAttrMerge(link, r.Header)

	var lastErr error
	var lastStatus int
	chain := link.Chain

	for i, entry := range chain {
		provider, err := f.DB.GetProvider(entry.ProviderID)
		if err != nil || !provider.Enabled {
			lastErr = fmt.Errorf("provider %s unavailable", entry.ProviderID)
			continue
		}
		adapter, ok := providers.AdapterFor(provider.Type)
		if !ok {
			lastErr = fmt.Errorf("no adapter for type %s", provider.Type)
			continue
		}

		// Use chain-entry API key override if set, otherwise fall back to global provider key.
		if entry.ApiKey != "" {
			provider.APIKey = entry.ApiKey
		}

		attempts := entry.RetryCount + 1
		for attempt := 0; attempt < attempts; attempt++ {
			modelOverride := origModel
			// Use fallback model only when escalating past the first attempt or
			// past the first chain entry (i.e. actually falling back).
			if (attempt > 0 || i > 0) && entry.FallbackModel != "" {
				modelOverride = entry.FallbackModel
			}
			req := providers.OpenAIReq{Raw: body, Model: modelOverride}
			start := time.Now()
			res := adapter.Forward(r.Context(), providers.FromModel(provider), req, modelOverride)
			latency := time.Since(start).Milliseconds()
			usedModel := modelOverride

			success := res.Err == nil && res.StatusCode >= 200 && res.StatusCode < 300
			f.Stats.Record(models.RequestLog{
				LinkID: link.ID, Path: link.Path, ProviderID: provider.ID,
				ProviderName: provider.Name, Model: usedModel,
				StatusCode: res.StatusCode, LatencyMS: latency,
				Success: success, Attributes: attributes,
				ErrorMessage: errStr(res.Err), CreatedAt: time.Now(),
			})

			if success {
				writeJSON(w, res.StatusCode, res.Body)
				return
			}
			lastErr = res.Err
			lastStatus = res.StatusCode

			// Escalate when this attempt should fall back:
			//   - transport error
			//   - status in OnStatusCodes
			//   - client/configurable error message matches OnErrors / timeout
			//   - 5xx by default (rules empty)
			if !shouldFallback(entry.Rules, res) {
				// Non-fallbackable failure (e.g. 4xx). Surface to client.
				if res.Body != nil {
					writeJSON(w, res.StatusCode, res.Body)
				} else {
					writeError(w, res.StatusCode, lastErr.Error())
				}
				return
			}
			// otherwise: try next attempt or next provider
		}
	}

	if lastStatus > 0 || lastErr != nil {
		msg := "all providers failed"
		if lastErr != nil {
			msg = lastErr.Error()
		}
		writeError(w, pickStatus(lastStatus), msg)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "no available providers")
}

func shouldFallback(r models.Rules, res providers.Result) bool {
	if res.Err != nil {
		msg := res.Err.Error()
		for _, e := range r.OnErrors {
			if strings.Contains(msg, e) {
				return true
			}
		}
		if r.OnTimeout && strings.Contains(msg, "timeout") {
			return true
		}
		// transport errors always escalate
		return res.StatusCode == 0
	}
	for _, c := range r.OnStatusCodes {
		if c == res.StatusCode {
			return true
		}
	}
	if len(r.OnStatusCodes) == 0 && res.StatusCode >= 500 {
		return true
	}
	return false
}

func extractModel(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return ""
	}
	if s, ok := m["model"].(string); ok {
		return s
	}
	return ""
}

func errStr(e error) string {
	if e == nil {
		return ""
	}
	return e.Error()
}

func pickStatus(s int) int {
	if s == 0 {
		return http.StatusBadGateway
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	b, _ := json.Marshal(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    "proxy_error",
		},
	})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// parseAttrMerge reads attributes from an optional request header (JSON) and
// merges them onto the link's configured attributes; returns merged map.
func parseAttrMerge(link models.ProxyLink, header http.Header) models.Map {
	merged := models.Map{}
	for k, v := range link.Attributes {
		merged[k] = v
	}
	if h := header.Get("X-Gateway-Attributes"); h != "" {
		var extra models.Map
		if err := json.Unmarshal([]byte(h), &extra); err == nil {
			for k, v := range extra {
				merged[k] = v
			}
		}
	}
	return merged
}

var _ = bytes.NewReader
var _ = context.TODO
var _ = strconv.Itoa
