package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIAdapterUsesOfficialSDKAndPreservesRawBody(t *testing.T) {
	var requestBody, authorization string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBodyBytes, _ := io.ReadAll(r.Body)
		requestBody = string(requestBodyBytes)
		authorization = r.Header.Get("Authorization")
		w.Header().Set("X-Request-Id", "openai-request")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}]}`))
	}))
	defer server.Close()

	adapter := newOpenAI("test-openai")
	body := []byte(`{"model":"original","messages":[{"role":"user","content":"hello"}],"private_field":true}`)
	result := adapter.Forward(context.Background(), Provider{
		BaseURL: server.URL + "/v1/chat/completions", APIKey: "openai-secret",
	}, OpenAIReq{Raw: body, Model: "original"}, "override", "chat_completions")

	if result.Err != nil || result.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v", result)
	}
	if authorization != "Bearer openai-secret" || !strings.Contains(requestBody, `"private_field":true`) || !strings.Contains(requestBody, `"model":"override"`) {
		t.Fatalf("SDK request did not preserve auth/body: authorization=%q body=%s", authorization, requestBody)
	}
	if result.ResponseHeaders["X-Request-Id"] != "openai-request" || result.RequestHeaders["Authorization"] == "" {
		t.Fatalf("SDK request/response metadata was not captured: %+v", result)
	}
}

func TestAnthropicAdapterUsesOfficialSDKWithoutOpenAIConversion(t *testing.T) {
	var requestBody, apiKey, version string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBodyBytes, _ := io.ReadAll(r.Body)
		requestBody = string(requestBodyBytes)
		apiKey = r.Header.Get("X-Api-Key")
		version = r.Header.Get("Anthropic-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","content":[{"type":"text","text":"ok"}]}`))
	}))
	defer server.Close()

	body := []byte(`{"model":"claude-test","max_tokens":32,"messages":[{"role":"user","content":"hello"}]}`)
	result := (AnthropicAdapter{}).Forward(context.Background(), Provider{
		BaseURL: server.URL + "/v1/messages", APIKey: "anthropic-secret",
	}, OpenAIReq{Raw: body, Model: "claude-test"}, "", "messages")

	if result.Err != nil || result.StatusCode != http.StatusOK {
		t.Fatalf("result = %+v", result)
	}
	if apiKey != "anthropic-secret" || version != "2023-06-01" || requestBody != string(body) {
		t.Fatalf("Anthropic SDK request changed the native contract: key=%q version=%q body=%s", apiKey, version, requestBody)
	}
	if !strings.Contains(string(result.Body), `"type":"message"`) {
		t.Fatalf("Anthropic response was converted unexpectedly: %s", result.Body)
	}
}
