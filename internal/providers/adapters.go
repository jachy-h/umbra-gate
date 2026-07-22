package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

func doRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

// setOpenAIModel overrides the model in the raw OpenAI request body.
func setOpenAIModel(raw []byte, model string) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw, err
	}
	m["model"] = model
	return json.Marshal(m)
}

// OpenAIAdapter serves OpenAI itself and OpenAI-compatible providers
// (DeepSeek, Qwen, and any user-added "custom" provider that speaks the
// OpenAI Chat Completions protocol).
type OpenAIAdapter struct {
	typeName string
	// pathSuffix is appended to base url, e.g. "/v1/chat/completions".
	pathSuffix string
}

func newOpenAI(name, suffix string) OpenAIAdapter { return OpenAIAdapter{name, suffix} }

func (a OpenAIAdapter) Type() string { return a.typeName }

func (a OpenAIAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride string) Result {
	url := strings.TrimRight(p.BaseURL, "/") + a.pathSuffix
	body := req.Raw
	if modelOverride != "" {
		b, err := setOpenAIModel(body, modelOverride)
		if err == nil {
			body = b
		}
	} else if req.Model != "" {
		// ensure model set
	}
	headers := map[string]string{"Authorization": "Bearer " + p.APIKey}
	b, code, err := doRequest(ctx, http.MethodPost, url, headers, body)
	if err != nil {
		return Result{Err: fmt.Errorf("openai upstream: %w", err)}
	}
	return Result{Body: b, StatusCode: code}
}

// Anthropic expects /v1/messages with x-api-key and anthropic-version header.
type AnthropicAdapter struct{}

func (AnthropicAdapter) Type() string { return "anthropic" }

func (AnthropicAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride string) Result {
	model := req.Model
	if modelOverride != "" {
		model = modelOverride
	}
	var sysParts []string
	var msgs []map[string]any
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if s, ok := m.Content.(string); ok {
				sysParts = append(sysParts, s)
			} else if b, err := json.Marshal(m.Content); err == nil {
				sysParts = append(sysParts, string(b))
			}
		default:
			msgs = append(msgs, map[string]any{"role": m.Role, "content": m.Content})
		}
	}
	bodyMap := map[string]any{
		"model":      model,
		"messages":   msgs,
		"max_tokens": 4096,
	}
	if len(sysParts) > 0 {
		bodyMap["system"] = strings.Join(sysParts, "\n")
	}
	body, _ := json.Marshal(bodyMap)
	headers := map[string]string{
		"x-api-key":         p.APIKey,
		"anthropic-version": strFromExtra(p.Extra, "anthropic_version", "2023-06-01"),
	}
	url := strings.TrimRight(p.BaseURL, "/") + "/v1/messages"
	b, code, err := doRequest(ctx, http.MethodPost, url, headers, body)
	if err != nil {
		return Result{Err: fmt.Errorf("anthropic upstream: %w", err)}
	}
	// convert anthropic response back to OpenAI format
	if code >= 200 && code < 300 {
		if conv, cerr := anthropicToOpenAI(b); cerr == nil {
			b = conv
		}
	}
	return Result{Body: b, StatusCode: code}
}

func strFromExtra(m map[string]any, key, def string) string {
	if m == nil {
		return def
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return def
}

// GeminiAdapter calls generateContent and converts to OpenAI format.
type GeminiAdapter struct{}

func (GeminiAdapter) Type() string { return "gemini" }

func (GeminiAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride string) Result {
	model := req.Model
	if modelOverride != "" {
		model = modelOverride
	}
	var sysParts []string
	var contents []map[string]any
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			if s, ok := m.Content.(string); ok {
				sysParts = append(sysParts, s)
			}
		case "user":
			contents = append(contents, map[string]any{"role": "user", "parts": textParts(m.Content)})
		case "assistant":
			contents = append(contents, map[string]any{"role": "model", "parts": textParts(m.Content)})
		}
	}
	bodyMap := map[string]any{"contents": contents}
	if len(sysParts) > 0 {
		bodyMap["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": strings.Join(sysParts, "\n")}}}
	}
	body, _ := json.Marshal(bodyMap)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		strings.TrimRight(p.BaseURL, "/"), model, p.APIKey)
	headers := map[string]string{"Content-Type": "application/json"}
	b, code, err := doRequest(ctx, http.MethodPost, url, headers, body)
	if err != nil {
		return Result{Err: fmt.Errorf("gemini upstream: %w", err)}
	}
	if code >= 200 && code < 300 {
		if conv, cerr := geminiToOpenAI(b); cerr == nil {
			b = conv
		}
	}
	return Result{Body: b, StatusCode: code}
}

func textParts(content any) []map[string]any {
	if s, ok := content.(string); ok {
		return []map[string]any{{"text": s}}
	}
	// fallback: stringify
	if b, err := json.Marshal(content); err == nil {
		return []map[string]any{{"text": string(b)}}
	}
	return []map[string]any{{"text": ""}}
}
