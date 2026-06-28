package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
)

type passthroughUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type passthroughResponse struct {
	Usage *passthroughUsage `json:"usage"`
}

type passthroughStreamChunk struct {
	Usage *passthroughUsage `json:"usage"`
}

func extractModel(body []byte) string {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Warn("failed to parse request body for model extraction", "error", err)
	}
	if req.Model == "" {
		return "unknown"
	}
	return req.Model
}

func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		slog.Warn("failed to parse request body for stream detection", "error", err)
	}
	return req.Stream
}

func (p *Proxy) handlePassthrough(w http.ResponseWriter, r *http.Request, route routeContext, providerCfg *config.ProviderConfig, upstream *url.URL) {
	startTime := time.Now()

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	r.Body.Close()
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	model := extractModel(bodyBytes)
	stream := isStreamRequest(bodyBytes)
	slog.Info("request", "agent", route.AgentID, "provider", route.ProviderName, "model", model, "path", upstream.Path)

	providerID, err := p.db.EnsureProvider(route.ProviderName)
	if err != nil {
		slog.Error("failed to ensure provider", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionID, err := p.db.CreateSessionWithMeta(providerID, model, db.SessionMeta{
		AgentID:   route.AgentID,
		ProjectID: route.ProjectID,
		Endpoint:  route.Endpoint,
		Stream:    stream,
	})
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if stream {
		p.proxyPassthroughStream(w, r, route, providerCfg, upstream, bodyBytes, sessionID, startTime)
	} else {
		p.proxyPassthroughNonStream(w, r, route, providerCfg, upstream, bodyBytes, sessionID, startTime)
	}
}

func (p *Proxy) buildPassthroughRequest(r *http.Request, route routeContext, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	copyAllForwardableHeaders(req.Header, r.Header)
	return req, nil
}

func (p *Proxy) proxyPassthroughNonStream(w http.ResponseWriter, r *http.Request, route routeContext, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte, sessionID int64, startTime time.Time) {
	req, err := p.buildPassthroughRequest(r, route, providerCfg, upstream, bodyBytes)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		slog.Error("failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		errMsg := err.Error()
		durationMs := time.Since(startTime).Milliseconds()
		p.db.CompleteSession(sessionID, 0, 0, durationMs, &errMsg)
		captureRequestLog(p.db, db.RequestLog{
			SessionID:       sessionID,
			ProviderName:    route.ProviderName,
			Method:          req.Method,
			URL:             req.URL.String(),
			RequestHeaders:  serializeHeaders(req.Header),
			RequestBody:     string(bodyBytes),
			ResponseStatus:  0,
			ResponseHeaders: "",
			ResponseBody:    "upstream error: " + errMsg,
			DurationMs:      durationMs,
		})
		slog.Error("upstream request failed", "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		slog.Error("failed to read upstream response", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	durationMs := time.Since(startTime).Milliseconds()

	if resp.StatusCode != http.StatusOK {
		errMsg := string(respBody)
		p.db.CompleteSession(sessionID, 0, 0, durationMs, &errMsg)
		captureRequestLog(p.db, db.RequestLog{
			SessionID:       sessionID,
			ProviderName:    route.ProviderName,
			Method:          req.Method,
			URL:             req.URL.String(),
			RequestHeaders:  serializeHeaders(req.Header),
			RequestBody:     string(bodyBytes),
			ResponseStatus:  resp.StatusCode,
			ResponseHeaders: serializeHeaders(resp.Header),
			ResponseBody:    string(respBody),
			DurationMs:      durationMs,
		})
		copyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody)
		return
	}

	var promptTokens, completionTokens int64
	var passthruResp passthroughResponse
	if err := json.Unmarshal(respBody, &passthruResp); err == nil && passthruResp.Usage != nil {
		promptTokens = int64(passthruResp.Usage.PromptTokens)
		completionTokens = int64(passthruResp.Usage.CompletionTokens)
	}

	slog.Info("upstream response",
		"session_id", sessionID,
		"provider", route.ProviderName,
		"status", resp.StatusCode,
		"body_size", len(respBody),
		"prompt_tokens", promptTokens,
		"completion_tokens", completionTokens,
		"duration_ms", durationMs,
	)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequestWithMeta(sessionID, promptTokens, completionTokens, durationMs, int64(resp.StatusCode), route.Endpoint, nil)
	captureRequestLog(p.db, db.RequestLog{
		SessionID:       sessionID,
		ProviderName:    route.ProviderName,
		Method:          req.Method,
		URL:             req.URL.String(),
		RequestHeaders:  serializeHeaders(req.Header),
		RequestBody:     string(bodyBytes),
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: serializeHeaders(resp.Header),
		ResponseBody:    string(respBody),
		DurationMs:      durationMs,
	})

	copyResponseHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
}

func (p *Proxy) proxyPassthroughStream(w http.ResponseWriter, r *http.Request, route routeContext, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte, sessionID int64, startTime time.Time) {
	req, err := p.buildPassthroughRequest(r, route, providerCfg, upstream, bodyBytes)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		slog.Error("failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	resp, err := p.client.Do(req)
	if err != nil {
		errMsg := err.Error()
		durationMs := time.Since(startTime).Milliseconds()
		p.db.CompleteSession(sessionID, 0, 0, durationMs, &errMsg)
		captureRequestLog(p.db, db.RequestLog{
			SessionID:       sessionID,
			ProviderName:    route.ProviderName,
			Method:          req.Method,
			URL:             req.URL.String(),
			RequestHeaders:  serializeHeaders(req.Header),
			RequestBody:     string(bodyBytes),
			ResponseStatus:  0,
			ResponseHeaders: "",
			ResponseBody:    "upstream error: " + errMsg,
			DurationMs:      durationMs,
		})
		slog.Error("upstream request failed", "error", err)
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		errMsg := "upstream returned non-200"
		if readErr == nil {
			errMsg = string(body)
		}
		durationMs := time.Since(startTime).Milliseconds()
		p.db.CompleteSession(sessionID, 0, 0, durationMs, &errMsg)
		captureRequestLog(p.db, db.RequestLog{
			SessionID:       sessionID,
			ProviderName:    route.ProviderName,
			Method:          req.Method,
			URL:             req.URL.String(),
			RequestHeaders:  serializeHeaders(req.Header),
			RequestBody:     string(bodyBytes),
			ResponseStatus:  resp.StatusCode,
			ResponseHeaders: serializeHeaders(resp.Header),
			ResponseBody:    string(body),
			DurationMs:      durationMs,
		})
		copyResponseHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	copyResponseHeaders(w.Header(), resp.Header)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		errMsg := "streaming not supported"
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		return
	}

	const bufSize = 4096
	var promptTokens, completionTokens int64
	var leftover string
	var capturedBody strings.Builder
	buf := make([]byte, bufSize)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				break
			}
			flusher.Flush()

			if capturedBody.Len() < maxLoggedBodyBytes {
				remaining := maxLoggedBodyBytes - capturedBody.Len()
				if n <= remaining {
					capturedBody.Write(buf[:n])
				} else {
					capturedBody.Write(buf[:remaining])
				}
			}

			data := leftover + string(buf[:n])
			lines := strings.Split(data, "\n")
			leftover = ""
			for i, line := range lines {
				line = strings.TrimSpace(line)
				if i == len(lines)-1 && line != "" && !strings.HasSuffix(data, "\n") {
					leftover = line
					continue
				}
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				jsonData := strings.TrimPrefix(line, "data: ")
				if jsonData == "[DONE]" {
					continue
				}
				var chunk passthroughStreamChunk
				if err := json.Unmarshal([]byte(jsonData), &chunk); err == nil && chunk.Usage != nil {
					promptTokens = int64(chunk.Usage.PromptTokens)
					completionTokens = int64(chunk.Usage.CompletionTokens)
				}
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				slog.Error("stream read error", "error", readErr)
			}
			break
		}
	}

	durationMs := time.Since(startTime).Milliseconds()

	slog.Info("upstream response",
		"session_id", sessionID,
		"provider", route.ProviderName,
		"stream", true,
		"prompt_tokens", promptTokens,
		"completion_tokens", completionTokens,
		"duration_ms", durationMs,
	)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequestWithMeta(sessionID, promptTokens, completionTokens, durationMs, int64(resp.StatusCode), route.Endpoint, nil)
	captureRequestLog(p.db, db.RequestLog{
		SessionID:       sessionID,
		ProviderName:    route.ProviderName,
		Method:          req.Method,
		URL:             req.URL.String(),
		RequestHeaders:  serializeHeaders(req.Header),
		RequestBody:     string(bodyBytes),
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: serializeHeaders(resp.Header),
		ResponseBody:    capturedBody.String(),
		DurationMs:      durationMs,
	})
}

// copyAllForwardableHeaders copies headers from the inbound request onto the
// outbound upstream request, preserving auth headers (Authorization, X-Api-Key)
// unlike copyForwardableHeaders which strips them for gateway-managed auth.
func copyAllForwardableHeaders(dst, src http.Header) {
	for key, values := range src {
		canon := http.CanonicalHeaderKey(key)
		if _, hop := hopByHopHeaders[canon]; hop {
			continue
		}
		switch canon {
		case "Host", "Content-Length":
			continue
		}
		for _, v := range values {
			dst.Add(canon, v)
		}
	}
}
