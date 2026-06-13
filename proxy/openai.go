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

	"github.com/anomalyco/llm-gateway/config"
)

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openAIResponse struct {
	Model string      `json:"model"`
	Usage openAIUsage `json:"usage"`
}

type openAIStreamChunk struct {
	Model string       `json:"model"`
	Usage *openAIUsage `json:"usage"`
}

func (p *Proxy) handleOpenAI(w http.ResponseWriter, r *http.Request, providerName string, providerCfg *config.ProviderConfig, target *url.URL, path string) {
	startTime := time.Now()

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	r.Body.Close()
	if err != nil {
		slog.Error("failed to read request body", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	model := extractModel(bodyBytes)
	slog.Info("request", "provider", providerName, "model", model, "path", path)

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

	isStreaming := isStreamRequest(bodyBytes)

	if isStreaming {
		p.proxyOpenAIStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	} else {
		p.proxyOpenAINonStream(w, r, providerCfg, target, path, bodyBytes, sessionID, startTime)
	}
}

func (p *Proxy) proxyOpenAINonStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		slog.Error("failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
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

	var oaiResp openAIResponse
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		slog.Warn("failed to parse upstream response", "error", err)
	}

	durationMs := time.Since(startTime).Milliseconds()
	promptTokens := int64(oaiResp.Usage.PromptTokens)
	completionTokens := int64(oaiResp.Usage.CompletionTokens)

	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody)
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

func (p *Proxy) proxyOpenAIStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	upstreamURL := target.String() + path
	req, err := http.NewRequestWithContext(r.Context(), "POST", upstreamURL, bytes.NewReader(bodyBytes))
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, 0, &errMsg)
		slog.Error("failed to create upstream request", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+providerCfg.APIKey)

	resp, err := p.client.Do(req)
	if err != nil {
		errMsg := err.Error()
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
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
		p.db.CompleteSession(sessionID, 0, 0, time.Since(startTime).Milliseconds(), &errMsg)
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
		return
	}

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
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
	buf := make([]byte, bufSize)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := w.Write(buf[:n]); writeErr != nil {
				break
			}
			flusher.Flush()

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
				var chunk openAIStreamChunk
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
	p.db.CompleteSession(sessionID, promptTokens, completionTokens, durationMs, nil)
	p.db.CreateRequest(sessionID, promptTokens, completionTokens, durationMs, nil)
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
