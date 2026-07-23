package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
)

var httpClient = &http.Client{Timeout: 120 * time.Second}

func doRequest(ctx context.Context, method, url string, headers map[string]string, body []byte) ([]byte, int, map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	responseHeaders := make(map[string]string, len(resp.Header))
	for key, values := range resp.Header {
		responseHeaders[key] = strings.Join(values, ", ")
	}
	return b, resp.StatusCode, responseHeaders, err
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
}

func newOpenAI(name string) OpenAIAdapter { return OpenAIAdapter{typeName: name} }

func (a OpenAIAdapter) Type() string { return a.typeName }

func (a OpenAIAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride, _ string) Result {
	requestURL := p.BaseURL
	body := req.Raw
	if modelOverride != "" {
		b, err := setOpenAIModel(body, modelOverride)
		if err == nil {
			body = b
		}
	} else if req.Model != "" {
		// ensure model set
	}
	baseURL, path, err := splitSDKTarget(requestURL)
	if err != nil {
		return Result{Err: fmt.Errorf("openai upstream: %w", err), RequestURL: requestURL, RequestBody: body}
	}
	var rawResponse *http.Response
	var responseBody []byte
	captured := requestSnapshot{URL: requestURL, Body: body}
	client := openai.NewClient(
		openaioption.WithAPIKey(p.APIKey),
		openaioption.WithBaseURL(baseURL),
		openaioption.WithHTTPClient(httpClient),
		openaioption.WithMaxRetries(0),
		openaioption.WithMiddleware(func(request *http.Request, next openaioption.MiddlewareNext) (*http.Response, error) {
			captured = snapshotRequest(request, body)
			return next(request)
		}),
	)
	err = client.Post(ctx, path, body, &responseBody, openaioption.WithResponseInto(&rawResponse))
	return sdkResult("openai", captured, responseBody, rawResponse, err)
}

// Anthropic expects /v1/messages with x-api-key and anthropic-version header.
type AnthropicAdapter struct{}

func (AnthropicAdapter) Type() string { return "anthropic" }

func (AnthropicAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride, _ string) Result {
	body := req.Raw
	if modelOverride != "" {
		if changed, err := setOpenAIModel(body, modelOverride); err == nil {
			body = changed
		}
	}
	requestURL := p.BaseURL
	baseURL, path, err := splitSDKTarget(requestURL)
	if err != nil {
		return Result{Err: fmt.Errorf("anthropic upstream: %w", err), RequestURL: requestURL, RequestBody: body}
	}
	var rawResponse *http.Response
	var responseBody []byte
	captured := requestSnapshot{URL: requestURL, Body: body}
	client := anthropic.NewClient(
		anthropicoption.WithAPIKey(p.APIKey),
		anthropicoption.WithBaseURL(baseURL),
		anthropicoption.WithHTTPClient(httpClient),
		anthropicoption.WithMaxRetries(0),
		anthropicoption.WithHeader("anthropic-version", strFromExtra(p.Extra, "anthropic_version", "2023-06-01")),
		anthropicoption.WithMiddleware(func(request *http.Request, next anthropicoption.MiddlewareNext) (*http.Response, error) {
			captured = snapshotRequest(request, body)
			return next(request)
		}),
	)
	err = client.Post(ctx, path, body, &responseBody, anthropicoption.WithResponseInto(&rawResponse))
	return sdkResult("anthropic", captured, responseBody, rawResponse, err)
}

type requestSnapshot struct {
	URL     string
	Headers map[string]string
	Body    []byte
}

func splitSDKTarget(rawURL string) (string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		if err == nil {
			err = fmt.Errorf("invalid URL %q", rawURL)
		}
		return "", "", err
	}
	baseURL := parsed.Scheme + "://" + parsed.Host + "/"
	path := strings.TrimPrefix(parsed.EscapedPath(), "/")
	if parsed.RawQuery != "" {
		path += "?" + parsed.RawQuery
	}
	return baseURL, path, nil
}

func snapshotRequest(request *http.Request, fallbackBody []byte) requestSnapshot {
	headers := make(map[string]string, len(request.Header))
	for key, values := range request.Header {
		headers[key] = strings.Join(values, ", ")
	}
	body := fallbackBody
	if request.GetBody != nil {
		if reader, err := request.GetBody(); err == nil {
			if content, readErr := io.ReadAll(reader); readErr == nil {
				body = content
			}
			_ = reader.Close()
		}
	}
	return requestSnapshot{URL: request.URL.String(), Headers: headers, Body: body}
}

func sdkResult(providerName string, request requestSnapshot, body []byte, response *http.Response, requestErr error) Result {
	result := Result{
		Body: body, RequestURL: request.URL, RequestHeaders: request.Headers, RequestBody: request.Body,
	}
	if response != nil {
		result.StatusCode = response.StatusCode
		result.ResponseHeaders = make(map[string]string, len(response.Header))
		for key, values := range response.Header {
			result.ResponseHeaders[key] = strings.Join(values, ", ")
		}
		if len(result.Body) == 0 && response.Body != nil {
			if contents, err := io.ReadAll(response.Body); err == nil {
				result.Body = contents
			}
			_ = response.Body.Close()
		}
	}
	if requestErr != nil && result.StatusCode == 0 {
		result.Err = fmt.Errorf("%s upstream: %w", providerName, requestErr)
	}
	return result
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

func (GeminiAdapter) Forward(ctx context.Context, p Provider, req OpenAIReq, modelOverride, _ string) Result {
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
	if req.MaxTokens > 0 {
		bodyMap["generationConfig"] = map[string]any{"maxOutputTokens": req.MaxTokens}
	}
	if len(sysParts) > 0 {
		bodyMap["systemInstruction"] = map[string]any{"parts": []map[string]any{{"text": strings.Join(sysParts, "\n")}}}
	}
	body, _ := json.Marshal(bodyMap)
	baseURL := strings.TrimRight(p.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/v1beta") {
		baseURL += "/v1beta"
	}
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", baseURL, model, p.APIKey)
	headers := map[string]string{"Content-Type": "application/json"}
	b, code, responseHeaders, err := doRequest(ctx, http.MethodPost, url, headers, body)
	if err != nil {
		return Result{Err: fmt.Errorf("gemini upstream: %w", err), RequestURL: url, RequestHeaders: headers, RequestBody: body, ResponseHeaders: responseHeaders}
	}
	if code >= 200 && code < 300 {
		if conv, cerr := geminiToOpenAI(b); cerr == nil {
			b = conv
		}
	}
	return Result{Body: b, StatusCode: code, RequestURL: url, RequestHeaders: headers, RequestBody: body, ResponseHeaders: responseHeaders}
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
