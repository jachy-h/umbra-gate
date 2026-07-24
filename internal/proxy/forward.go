package proxy

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

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	conversion "github.com/jachy-h/llm-gateway-lite/internal/protocol"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

type Forwarder struct {
	DB    *db.DB
	Stats *stats.Service
}

// Handle implements the OpenAI Chat Completions compatible proxy for a link.
func (f *Forwarder) Handle(w http.ResponseWriter, r *http.Request, link models.ProxyLink) {
	f.HandleRequest(w, r, link, models.ProtocolOpenAI, models.FormatChatCompletions)
}

// HandleRequest forwards one API format within the style fixed by the first
// chain node. API formats (Chat Completions, Responses, Messages) are endpoint
// capabilities, not link protocols.
func (f *Forwarder) HandleRequest(w http.ResponseWriter, r *http.Request, link models.ProxyLink, requestProtocol, requestFormat string) {
	linkProtocol := link.Protocol
	if linkProtocol == "" && len(link.Chain) > 0 {
		linkProtocol = link.Chain[0].Protocol
	}
	if linkProtocol != "" && linkProtocol != requestProtocol {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("link protocol is %s, request protocol is %s", linkProtocol, requestProtocol))
		return
	}
	// The request URL chooses the client wire format. A Link is protocol-level
	// routing only, so one OpenAI Link can serve both /chat/completions and
	// /responses to the same provider chain.
	linkRequestFormat := requestFormat
	linkResponseFormat := requestFormat
	if len(link.SupportedFormats) > 0 && !hasFormat(link.SupportedFormats, requestFormat) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("link does not support %s", requestFormat))
		return
	}
	body, _ := io.ReadAll(r.Body)
	origModel := extractModel(body)
	// Statistics dimensions are owned by the Link configuration. Do not accept
	// caller-supplied dimensions: they make cardinality and storage unbounded.
	attributes := link.Attributes
	requestURL := inboundRequestURL(r)
	requestHeaders := redactHTTPHeaders(r.Header)

	var lastErr error
	var lastStatus int
	var lastBody []byte
	chain := link.Chain

	for i, entry := range chain {
		if entry.Protocol != "" && linkProtocol != "" && entry.Protocol != linkProtocol {
			lastErr = fmt.Errorf("chain entry %d protocol %s does not match link protocol %s", i, entry.Protocol, linkProtocol)
			continue
		}
		if len(entry.SupportedFormats) > 0 && !hasFormat(entry.SupportedFormats, requestFormat) {
			lastErr = fmt.Errorf("provider %s does not support %s", entry.ProviderID, requestFormat)
			continue
		}
		provider, err := f.DB.GetProvider(entry.ProviderID)
		if err != nil || !provider.Enabled {
			lastErr = fmt.Errorf("provider %s unavailable", entry.ProviderID)
			continue
		}
		// Use chain-entry API key override if set, otherwise fall back to global provider key.
		if entry.ApiKey != "" {
			provider.APIKey = entry.ApiKey
		}
		protocol := entry.Protocol
		if protocol == "" {
			protocol = linkProtocol
		}
		adapter, ok := providers.AdapterForProtocol(provider.Type, protocol)
		if !ok {
			lastErr = fmt.Errorf("no adapter for type %s and protocol %s", provider.Type, protocol)
			continue
		}
		endpoint, ok := endpointForEntry(provider, entry, protocol, linkRequestFormat, linkResponseFormat)
		if !ok {
			lastErr = fmt.Errorf("provider %s has no endpoint convertible from %s to %s", provider.Name, linkRequestFormat, linkResponseFormat)
			continue
		}
		provider.BaseURL = endpointURL(endpoint)

		attempts := entry.RetryCount + 1
		for attempt := 0; attempt < attempts; attempt++ {
			modelOverride := origModel
			// Use fallback model only when escalating past the first attempt or
			// past the first chain entry (i.e. actually falling back).
			if (attempt > 0 || i > 0) && entry.FallbackModel != "" {
				modelOverride = entry.FallbackModel
			}
			upstreamBody, prepareErr := conversion.ConvertRequest(body, linkRequestFormat, endpoint.RequestFormat)
			if prepareErr != nil {
				lastErr = fmt.Errorf("provider %s request adaptation failed: %w", provider.Name, prepareErr)
				break
			}
			req := openAIRequest(upstreamBody, modelOverride)
			start := time.Now()
			res := adapter.Forward(r.Context(), providers.FromModel(provider), req, modelOverride, endpoint.RequestFormat)
			latency := time.Since(start).Milliseconds()
			usedModel := modelOverride

			validationErr := validateResult(res, endpoint.ResponseFormat)
			clientBody := res.Body
			if validationErr == nil {
				clientBody, validationErr = conversion.ConvertResponse(res.Body, endpoint.ResponseFormat, linkResponseFormat)
				if validationErr == nil {
					validationErr = validateResult(providers.Result{StatusCode: res.StatusCode, Body: clientBody}, linkResponseFormat)
				}
			}
			success := validationErr == nil
			f.Stats.Record(models.RequestLog{
				LinkID: link.ID, Path: link.Path, ProviderID: provider.ID,
				ProviderName: provider.Name, Model: usedModel,
				StatusCode: res.StatusCode, LatencyMS: latency,
				Success: success, Attributes: attributes,
				ErrorMessage: errStr(validationErr),
				RequestURL:   requestURL, RequestHeaders: requestHeaders, RequestBody: logBody(body),
				UpstreamURL: redactURL(res.RequestURL), UpstreamHeaders: redactStringHeaders(res.RequestHeaders), UpstreamBody: logBody(res.RequestBody),
				ResponseHeaders: redactStringHeaders(res.ResponseHeaders), ResponseBody: logBody(res.Body), CreatedAt: time.Now(),
			})

			if success {
				writeJSON(w, res.StatusCode, clientBody)
				return
			}
			lastErr = validationErr
			lastStatus = res.StatusCode
			lastBody = res.Body

			// Every failed or malformed upstream response is handled inside the
			// gateway. Continue to the next attempt/provider instead of asking the
			// Agent to retry the request itself.
		}
	}

	if lastStatus > 0 || lastErr != nil {
		if lastBody != nil && (lastStatus < 200 || lastStatus >= 300) {
			writeJSON(w, pickStatus(lastStatus), lastBody)
			return
		}
		msg := "all providers failed"
		if lastErr != nil {
			msg = lastErr.Error()
		}
		status := pickStatus(lastStatus)
		if status >= 200 && status < 300 {
			status = http.StatusBadGateway
		}
		writeError(w, status, msg)
		return
	}
	writeError(w, http.StatusServiceUnavailable, "no available providers")
}

func endpointForRequest(provider models.Provider, protocol, responseFormat string) (models.ProviderEndpoint, bool) {
	for _, endpoint := range provider.Endpoints {
		if endpoint.Protocol == protocol && endpoint.ResponseFormat == responseFormat && endpoint.BaseURL != "" {
			return endpoint, true
		}
	}
	return models.ProviderEndpoint{}, false
}

func endpointForFormats(provider models.Provider, protocol, requestFormat, responseFormat string) (models.ProviderEndpoint, bool) {
	for _, endpoint := range provider.Endpoints {
		if endpoint.Protocol == protocol && endpoint.BaseURL != "" && conversion.CanConvert(requestFormat, endpoint.RequestFormat) && conversion.CanConvert(endpoint.ResponseFormat, responseFormat) {
			return endpoint, true
		}
	}
	return models.ProviderEndpoint{}, false
}

func endpointForEntry(provider models.Provider, entry models.ChainEntry, protocol, requestFormat, responseFormat string) (models.ProviderEndpoint, bool) {
	if len(entry.SupportedFormats) == 0 {
		return endpointForFormats(provider, protocol, requestFormat, responseFormat)
	}
	base, ok := firstEndpointForProtocol(provider, protocol)
	if !ok {
		return models.ProviderEndpoint{}, false
	}
	base.BaseURL = operationBaseURL(base.BaseURL)
	base.RequestFormat = requestFormat
	base.ResponseFormat = responseFormat
	return base, true
}

func operationBaseURL(raw string) string {
	base := strings.TrimRight(raw, "/")
	for _, suffix := range []string{"/chat/completions", "/responses", "/messages"} {
		if strings.HasSuffix(base, suffix) {
			return strings.TrimSuffix(base, suffix)
		}
	}
	return base
}

func hasFormat(formats []string, format string) bool {
	for _, candidate := range formats {
		if candidate == format {
			return true
		}
	}
	return false
}

func firstEndpointForProtocol(provider models.Provider, protocol string) (models.ProviderEndpoint, bool) {
	for _, endpoint := range provider.Endpoints {
		if endpoint.Protocol == protocol && endpoint.BaseURL != "" {
			return endpoint, true
		}
	}
	return models.ProviderEndpoint{}, false
}

func endpointURL(endpoint models.ProviderEndpoint) string {
	baseURL := strings.TrimRight(endpoint.BaseURL, "/")
	parsed, _ := url.Parse(baseURL)
	hasPathPrefix := parsed != nil && strings.Trim(parsed.Path, "/") != ""
	switch endpoint.RequestFormat {
	case models.FormatChatCompletions:
		// Some compatible providers intentionally accept Chat payloads at a
		// /responses URL. Treat an explicit operation URL as authoritative.
		if strings.HasSuffix(baseURL, "/chat/completions") || strings.HasSuffix(baseURL, "/responses") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/v1") || hasPathPrefix {
			return baseURL + "/chat/completions"
		}
		return baseURL + "/v1/chat/completions"
	case models.FormatResponses:
		if strings.HasSuffix(baseURL, "/responses") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/v1") || hasPathPrefix {
			return baseURL + "/responses"
		}
		return baseURL + "/v1/responses"
	case models.FormatMessages:
		if strings.HasSuffix(baseURL, "/messages") {
			return baseURL
		}
		if strings.HasSuffix(baseURL, "/v1") {
			return baseURL + "/messages"
		}
		return baseURL + "/v1/messages"
	default:
		return baseURL
	}
}

// ValidateChain actively probes every wire format rather than trusting a
// provider type or endpoint declaration. The link's capability is the
// intersection of all node capabilities.
func (f *Forwarder) ValidateChain(ctx context.Context, link models.ProxyLink) models.ProxyLink {
	link.SupportedFormats = nil
	for i := range link.Chain {
		entry := &link.Chain[i]
		entry.ValidatedAt = time.Now()
		entry.SupportedFormats = nil
		entry.ValidationError = ""
		ok := false
		entry.ValidationOK = &ok
		if entry.Protocol != "" && link.Protocol != "" && entry.Protocol != link.Protocol {
			entry.ValidationError = fmt.Sprintf("protocol mismatch: node uses %s, link requires %s", entry.Protocol, link.Protocol)
			continue
		}
		provider, err := f.DB.GetProvider(entry.ProviderID)
		if err != nil || !provider.Enabled {
			entry.ValidationError = "provider unavailable"
			continue
		}
		if entry.ApiKey != "" {
			provider.APIKey = entry.ApiKey
		}
		protocol := entry.Protocol
		if protocol == "" {
			protocol = link.Protocol
		}
		adapter, adapterExists := providers.AdapterForProtocol(provider.Type, protocol)
		base, endpointExists := firstEndpointForProtocol(provider, protocol)
		if !adapterExists || !endpointExists {
			entry.ValidationError = "provider adapter or endpoint unavailable"
			continue
		}
		model := entry.FallbackModel
		if model == "" && len(provider.Models) > 0 {
			model = provider.Models[0]
		}
		if model == "" {
			entry.ValidationError = "no model configured for validation"
			continue
		}
		formats := []string{models.FormatChatCompletions, models.FormatResponses}
		if protocol == models.ProtocolAnthropic {
			formats = []string{models.FormatMessages}
		}
		errs := []string{}
		for _, format := range formats {
			endpoint := base
			endpoint.BaseURL = operationBaseURL(base.BaseURL)
			endpoint.RequestFormat, endpoint.ResponseFormat = format, format
			provider.BaseURL = endpointURL(endpoint)
			body := validationBody(model, format)
			probeCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			res := adapter.Forward(probeCtx, providers.FromModel(provider), openAIRequest(body, model), model, format)
			cancel()
			err := validateResult(res, format)
			f.recordValidationAttempt(link, *entry, i, provider.Name, model, format, body, res, err)
			if err == nil {
				entry.SupportedFormats = append(entry.SupportedFormats, format)
			} else {
				errs = append(errs, format+": "+err.Error())
			}
		}
		if len(entry.SupportedFormats) > 0 {
			ok = true
			entry.ValidationOK = &ok
		} else {
			entry.ValidationError = strings.Join(errs, "; ")
		}
		if i == 0 {
			link.SupportedFormats = append([]string(nil), entry.SupportedFormats...)
		} else {
			link.SupportedFormats = intersectFormats(link.SupportedFormats, entry.SupportedFormats)
		}
	}
	return link
}

func validationBody(model, format string) []byte {
	payload := map[string]any{"model": model}
	if format == models.FormatResponses {
		payload["input"] = "Reply with OK."
		payload["max_output_tokens"] = 100
	} else {
		payload["messages"] = []map[string]string{{"role": "user", "content": "Reply with OK."}}
		payload["max_tokens"] = 100
	}
	body, _ := json.Marshal(payload)
	return body
}

func intersectFormats(left, right []string) []string {
	out := []string{}
	for _, format := range left {
		if hasFormat(right, format) {
			out = append(out, format)
		}
	}
	return out
}

func (f *Forwarder) recordValidationAttempt(link models.ProxyLink, entry models.ChainEntry, position int, providerName, model, format string, body []byte, res providers.Result, validationErr error) {
	if f.Stats == nil {
		return
	}
	attributes := models.Map{"_request_type": "link_validation", "_chain_position": position, "_format": format}
	for key, value := range link.Attributes {
		attributes[key] = value
	}
	f.Stats.Record(models.RequestLog{LinkID: link.ID, Path: link.Path, ProviderID: entry.ProviderID, ProviderName: providerName, Model: model, StatusCode: res.StatusCode, Success: validationErr == nil, ErrorMessage: errStr(validationErr), RequestBody: logBody(body), UpstreamURL: redactURL(res.RequestURL), UpstreamHeaders: redactStringHeaders(res.RequestHeaders), UpstreamBody: logBody(res.RequestBody), ResponseHeaders: redactStringHeaders(res.ResponseHeaders), ResponseBody: logBody(res.Body), Attributes: attributes, CreatedAt: time.Now()})
}

const maxLoggedBodyBytes = 1024 * 1024

func logBody(body []byte) string {
	if len(body) <= maxLoggedBodyBytes {
		return string(body)
	}
	return string(body[:maxLoggedBodyBytes]) + "\n...[truncated]"
}

func inboundRequestURL(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = r.Header.Get("X-Forwarded-Proto")
	}
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	if r.Host == "" {
		return redactURL(r.URL.String())
	}
	return redactURL(scheme + "://" + r.Host + r.URL.RequestURI())
}

func redactHTTPHeaders(headers http.Header) models.Map {
	out := models.Map{}
	for key, values := range headers {
		if sensitiveHeader(key) {
			out[key] = "[REDACTED]"
		} else {
			out[key] = strings.Join(values, ", ")
		}
	}
	return out
}

func redactStringHeaders(headers map[string]string) models.Map {
	out := models.Map{}
	for key, value := range headers {
		if sensitiveHeader(key) {
			out[key] = "[REDACTED]"
		} else {
			out[key] = value
		}
	}
	return out
}

func sensitiveHeader(key string) bool {
	switch strings.ToLower(key) {
	case "authorization", "proxy-authorization", "x-api-key", "api-key", "x-admin-token", "cookie", "set-cookie":
		return true
	default:
		return false
	}
}

func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	query := parsed.Query()
	for key := range query {
		switch strings.ToLower(key) {
		case "key", "api_key", "token", "access_token":
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func openAIRequest(body []byte, model string) providers.OpenAIReq {
	req := providers.OpenAIReq{Raw: body, Model: model}
	_ = json.Unmarshal(body, &req)
	req.Raw = body
	if model != "" {
		req.Model = model
	}
	return req
}

func prepareRequestBody(body []byte, inboundFormat, upstreamFormat string) ([]byte, error) {
	if inboundFormat == upstreamFormat {
		return body, nil
	}
	if inboundFormat != models.FormatResponses || upstreamFormat != models.FormatChatCompletions {
		return nil, fmt.Errorf("unsupported format conversion %s -> %s", inboundFormat, upstreamFormat)
	}
	var source map[string]any
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("invalid Responses request: %w", err)
	}
	target := map[string]any{}
	for _, key := range []string{"model", "stream", "temperature", "top_p", "metadata"} {
		if value, ok := source[key]; ok {
			target[key] = value
		}
	}
	if value, ok := source["max_output_tokens"]; ok {
		target["max_tokens"] = value
	}
	messages := make([]map[string]any, 0)
	if instructions, ok := source["instructions"].(string); ok && instructions != "" {
		messages = append(messages, map[string]any{"role": "system", "content": instructions})
	}
	switch input := source["input"].(type) {
	case string:
		messages = append(messages, map[string]any{"role": "user", "content": input})
	case []any:
		for _, item := range input {
			message, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("unsupported Responses input item")
			}
			role, _ := message["role"].(string)
			if role == "" {
				role = "user"
			}
			content, err := responsesInputContentToChat(message["content"])
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{"role": role, "content": content})
		}
	default:
		return nil, fmt.Errorf("Responses input must be a string or message array")
	}
	target["messages"] = messages
	if tools, exists := source["tools"]; exists {
		converted, err := responsesToolsToChat(tools)
		if err != nil {
			return nil, err
		}
		target["tools"] = converted
	}
	return json.Marshal(target)
}

func responsesInputContentToChat(value any) (any, error) {
	if text, ok := value.(string); ok {
		return text, nil
	}
	parts, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported Responses message content")
	}
	chatParts := make([]map[string]any, 0, len(parts))
	for _, rawPart := range parts {
		part, ok := rawPart.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported Responses content part")
		}
		switch partType, _ := part["type"].(string); partType {
		case "input_text":
			chatParts = append(chatParts, map[string]any{"type": "text", "text": part["text"]})
		case "input_image":
			chatParts = append(chatParts, map[string]any{"type": "image_url", "image_url": map[string]any{"url": part["image_url"]}})
		default:
			return nil, fmt.Errorf("unsupported Responses content type %q", partType)
		}
	}
	return chatParts, nil
}

func responsesToolsToChat(value any) ([]map[string]any, error) {
	tools, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("Responses tools must be an array")
	}
	converted := make([]map[string]any, 0, len(tools))
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok || tool["type"] != "function" {
			return nil, fmt.Errorf("only function tools can be adapted from Responses to Chat Completions")
		}
		function := map[string]any{"name": tool["name"]}
		if description, exists := tool["description"]; exists {
			function["description"] = description
		}
		if parameters, exists := tool["parameters"]; exists {
			function["parameters"] = parameters
		}
		converted = append(converted, map[string]any{"type": "function", "function": function})
	}
	return converted, nil
}

func validateResult(res providers.Result, responseFormat string) error {
	if res.Err != nil {
		return res.Err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("upstream returned HTTP %d", res.StatusCode)
	}
	trimmed := bytes.TrimSpace(res.Body)
	if bytes.HasPrefix(trimmed, []byte("data:")) {
		for _, line := range bytes.Split(trimmed, []byte("\n")) {
			line = bytes.TrimSpace(line)
			if !bytes.HasPrefix(line, []byte("data:")) {
				continue
			}
			data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
			if bytes.Equal(data, []byte("[DONE]")) {
				continue
			}
			var event map[string]any
			if json.Unmarshal(data, &event) == nil {
				switch responseFormat {
				case models.FormatResponses:
					if eventType, _ := event["type"].(string); strings.HasPrefix(eventType, "response.") && !strings.Contains(eventType, "error") {
						return nil
					}
				case models.FormatMessages:
					if eventType, _ := event["type"].(string); eventType == "message_start" || eventType == "content_block_delta" || eventType == "message_stop" {
						return nil
					}
				default:
					if choices, ok := event["choices"].([]any); ok && len(choices) > 0 {
						return nil
					}
				}
			}
		}
		return fmt.Errorf("upstream stream has no valid choices")
	}
	var payload map[string]any
	if err := json.Unmarshal(res.Body, &payload); err != nil {
		return fmt.Errorf("invalid upstream JSON: %w", err)
	}
	if upstreamErr, exists := payload["error"]; exists && upstreamErr != nil {
		return fmt.Errorf("upstream response contains an error")
	}
	switch responseFormat {
	case models.FormatResponses:
		output, ok := payload["output"].([]any)
		if !ok || len(output) == 0 {
			return fmt.Errorf("upstream Responses API response has no output")
		}
		return nil
	case models.FormatMessages:
		content, ok := payload["content"].([]any)
		if !ok || len(content) == 0 {
			return fmt.Errorf("upstream Anthropic Messages response has no content")
		}
		return nil
	}
	choices, ok := payload["choices"].([]any)
	if !ok || len(choices) == 0 {
		return fmt.Errorf("upstream Chat Completions response has no choices")
	}
	return nil
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
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("data:")) {
		w.Header().Set("Content-Type", "text/event-stream")
	} else {
		w.Header().Set("Content-Type", "application/json")
	}
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
