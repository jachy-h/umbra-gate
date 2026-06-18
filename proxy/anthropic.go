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

const defaultAnthropicVersion = "2023-06-01"

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicResponse struct {
	Model string         `json:"model"`
	Usage anthropicUsage `json:"usage"`
}

type anthropicStreamEvent struct {
	Type  string          `json:"type"`
	Usage *anthropicUsage `json:"usage"`
}

type anthropicMessage struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream,omitempty"`
}

func (p *Proxy) handleAnthropic(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, upstream *url.URL) {
	startTime := time.Now()

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	r.Body.Close()
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var msg anthropicMessage
	if err := json.Unmarshal(bodyBytes, &msg); err != nil {
		slog.Warn("failed to parse anthropic request", "error", err)
	}
	model := msg.Model
	if model == "" {
		model = "unknown"
	}

	slog.Info("request", "provider", providerName, "model", model, "path", upstream.Path)

	providerID, err := p.db.EnsureProvider(providerName)
	if err != nil {
		slog.Error("failed to ensure provider", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sessionID, err := p.db.CreateSession(providerID, model)
	if err != nil {
		slog.Error("failed to create session", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if msg.Stream {
		p.proxyAnthropicStream(w, r, providerName, providerCfg, upstream, bodyBytes, sessionID, startTime)
	} else {
		p.proxyAnthropicNonStream(w, r, providerName, providerCfg, upstream, bodyBytes, sessionID, startTime)
	}
}

func (p *Proxy) buildAnthropicRequest(r *http.Request, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, upstream.String(), bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}
	copyForwardableHeaders(req.Header, r.Header)
	if providerCfg.APIKey != "" {
		req.Header.Set("x-api-key", providerCfg.APIKey)
	}
	if req.Header.Get("anthropic-version") == "" {
		req.Header.Set("anthropic-version", defaultAnthropicVersion)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (p *Proxy) proxyAnthropicNonStream(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte, sessionID int64, startTime time.Time) {
	req, err := p.buildAnthropicRequest(r, providerCfg, upstream, bodyBytes)
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
			SessionID:      sessionID,
			ProviderName:   providerName,
			Method:         req.Method,
			URL:            req.URL.String(),
			RequestHeaders: serializeHeaders(req.Header),
			RequestBody:    string(bodyBytes),
			ResponseBody:   "upstream error: " + errMsg,
			DurationMs:     durationMs,
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
			ProviderName:    providerName,
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

	var antResp anthropicResponse
	if err := json.Unmarshal(respBody, &antResp); err != nil {
		slog.Warn("failed to parse upstream response", "error", err)
	}

	promptTokens := int64(antResp.Usage.InputTokens)
	completionTokens := int64(antResp.Usage.OutputTokens)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)
	captureRequestLog(p.db, db.RequestLog{
		SessionID:       sessionID,
		ProviderName:    providerName,
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

func (p *Proxy) proxyAnthropicStream(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, upstream *url.URL, bodyBytes []byte, sessionID int64, startTime time.Time) {
	req, err := p.buildAnthropicRequest(r, providerCfg, upstream, bodyBytes)
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
			SessionID:      sessionID,
			ProviderName:   providerName,
			Method:         req.Method,
			URL:            req.URL.String(),
			RequestHeaders: serializeHeaders(req.Header),
			RequestBody:    string(bodyBytes),
			ResponseBody:   "upstream error: " + errMsg,
			DurationMs:     durationMs,
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
			ProviderName:    providerName,
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
	var inputTokens, outputTokens int64
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
				var event anthropicStreamEvent
				if err := json.Unmarshal([]byte(jsonData), &event); err == nil && event.Usage != nil {
					inputTokens = int64(event.Usage.InputTokens)
					outputTokens = int64(event.Usage.OutputTokens)
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
	p.db.CompleteSession(sessionID, inputTokens, outputTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, inputTokens, outputTokens, durationMs, nil)
	captureRequestLog(p.db, db.RequestLog{
		SessionID:       sessionID,
		ProviderName:    providerName,
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
