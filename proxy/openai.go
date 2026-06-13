package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
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

	bodyBytes, err := io.ReadAll(r.Body)
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

	resp, err := http.DefaultClient.Do(req)
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
	json.Unmarshal(respBody, &oaiResp)

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
	json.Unmarshal(body, &req)
	if req.Model == "" {
		return "unknown"
	}
	return req.Model
}

func (p *Proxy) proxyOpenAIStream(w http.ResponseWriter, r *http.Request, providerCfg *config.ProviderConfig, target *url.URL, path string, bodyBytes []byte, sessionID int64, startTime time.Time) {
	http.Error(w, "streaming not yet implemented", http.StatusNotImplemented)
}

func isStreamRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}
