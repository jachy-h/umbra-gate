package proxy

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

func TestHandleFallsBackOnAnyFailedResponse(t *testing.T) {
	providers.Register(fallbackTestAdapter{})

	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Now()
	for _, provider := range []models.Provider{
		{ID: "failed", Name: "failed-test", Type: "fallback-test", BaseURL: "http://unused", Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "http://unused"}}, Enabled: true, CreatedAt: now},
		{ID: "succeeded", Name: "succeeded-test", Type: "fallback-test", BaseURL: "http://unused", Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "http://unused"}}, Enabled: true, CreatedAt: now},
	} {
		if err := database.UpsertProvider(provider); err != nil {
			t.Fatal(err)
		}
	}

	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{ID: "link", Path: "token", Protocol: models.ProtocolOpenAI, Chain: []models.ChainEntry{{ProviderID: "failed", Protocol: models.ProtocolOpenAI}, {ProviderID: "succeeded", Protocol: models.ProtocolOpenAI}}}
	body := []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/llm-gateway-lite/token/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer agent-secret")
	req.Header.Set("X-Trace-ID", "trace-123")
	response := httptest.NewRecorder()

	forwarder.Handle(response, req, link)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"content":"ok"`)) {
		t.Fatalf("unexpected response: %s", response.Body.String())
	}
	logs, err := database.ListRecentLogs(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 2 || !logs[0].Success || logs[1].Success {
		t.Fatalf("unexpected fallback logs: %+v", logs)
	}
	if !bytes.Contains([]byte(logs[0].RequestBody), []byte(`"content":"hello"`)) || !bytes.Contains([]byte(logs[0].ResponseBody), []byte(`"content":"ok"`)) {
		t.Fatalf("request/response detail was not recorded: %+v", logs[0])
	}
	if logs[0].RequestURL == "" || logs[0].RequestHeaders["Authorization"] != "[REDACTED]" || logs[0].RequestHeaders["X-Trace-Id"] != "trace-123" {
		t.Fatalf("gateway request metadata was not recorded safely: %+v", logs[0])
	}
	if strings.Contains(logs[0].UpstreamURL, "provider-secret") || logs[0].UpstreamHeaders["Authorization"] != "[REDACTED]" {
		t.Fatalf("upstream request metadata leaked a secret: %+v", logs[0])
	}
	if logs[0].ResponseHeaders["X-Request-Id"] != "request-123" || logs[0].ResponseHeaders["Set-Cookie"] != "[REDACTED]" {
		t.Fatalf("response headers were not recorded safely: %+v", logs[0])
	}
}

func TestHandleUsesOnlyLinkConfiguredAttributes(t *testing.T) {
	providers.Register(fallbackTestAdapter{})
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider := models.Provider{ID: "succeeded", Name: "succeeded-test", Type: "fallback-test", BaseURL: "http://unused", Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "http://unused"}}, Enabled: true, CreatedAt: time.Now()}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}
	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{ID: "link", Path: "token", Protocol: models.ProtocolOpenAI, Attributes: models.Map{"environment": "production"}, Chain: []models.ChainEntry{{ProviderID: provider.ID, Protocol: models.ProtocolOpenAI}}}
	req := httptest.NewRequest(http.MethodPost, "/llm-gateway-lite/token/v1/chat/completions", strings.NewReader(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`))
	req.Header.Set("X-Gateway-Attributes", `{"untrusted":"random-value","environment":"overridden"}`)

	forwarder.Handle(httptest.NewRecorder(), req, link)

	logs, err := database.ListRecentLogs(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].Attributes["environment"] != "production" {
		t.Fatalf("configured attributes were not retained: %+v", logs)
	}
	if _, exists := logs[0].Attributes["untrusted"]; exists {
		t.Fatalf("caller-supplied attributes were stored: %+v", logs[0].Attributes)
	}
}

type fallbackTestAdapter struct{}

func (fallbackTestAdapter) Type() string { return "fallback-test" }

func (fallbackTestAdapter) Forward(_ context.Context, provider providers.Provider, _ providers.OpenAIReq, _, _ string) providers.Result {
	if provider.ID == "failed" {
		return providers.Result{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":{"message":"bad model"}}`), RequestURL: "https://provider.test/v1/chat/completions?token=provider-secret", RequestHeaders: map[string]string{"Authorization": "Bearer provider-secret", "Content-Type": "application/json"}, RequestBody: []byte(`{"model":"test"}`)}
	}
	return providers.Result{StatusCode: http.StatusOK, Body: []byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`), RequestURL: "https://provider.test/v1/chat/completions?token=provider-secret", RequestHeaders: map[string]string{"Authorization": "Bearer provider-secret", "Content-Type": "application/json"}, RequestBody: []byte(`{"model":"test"}`), ResponseHeaders: map[string]string{"Content-Type": "application/json", "X-Request-Id": "request-123", "Set-Cookie": "session=secret"}}
}

func TestValidateResultAcceptsOpenAIStream(t *testing.T) {
	result := validateResult(structResult(http.StatusOK, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n"), models.FormatChatCompletions)
	if result != nil {
		t.Fatal(result)
	}
}

func TestValidateResultAcceptsResponsesAPI(t *testing.T) {
	result := validateResult(structResult(http.StatusOK, `{"id":"resp_1","object":"response","output":[{"type":"message"}]}`), models.FormatResponses)
	if result != nil {
		t.Fatal(result)
	}
}

func TestPrepareRequestBodyAdaptsResponsesToHybridChatEndpoint(t *testing.T) {
	body, err := prepareRequestBody(
		[]byte(`{"model":"test","instructions":"Be brief.","input":"hello","max_output_tokens":32,"private_field":"drop-me"}`),
		models.FormatResponses,
		models.FormatChatCompletions,
	)
	if err != nil {
		t.Fatal(err)
	}
	value := string(body)
	if !strings.Contains(value, `"role":"system"`) || !strings.Contains(value, `"role":"user"`) ||
		!strings.Contains(value, `"max_tokens":32`) || strings.Contains(value, `"input"`) {
		t.Fatalf("unexpected adapted body: %s", value)
	}
}

func TestValidateResultAcceptsAnthropicMessages(t *testing.T) {
	result := validateResult(structResult(http.StatusOK, `{"id":"msg_1","type":"message","content":[{"type":"text","text":"ok"}]}`), models.FormatMessages)
	if result != nil {
		t.Fatal(result)
	}
}

func TestHandleResponsesUsesProtocolEndpointWithoutConversion(t *testing.T) {
	providers.Register(responsesTestAdapter{})
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider := models.Provider{
		ID: "responses-native", Name: "responses-native", Type: "responses-test",
		Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatResponses, ResponseFormat: models.FormatResponses, BaseURL: "https://provider.test/v1/responses"}},
		Enabled:   true, CreatedAt: time.Now(),
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}
	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{
		ID: "responses-link", Path: "responses", Protocol: models.ProtocolOpenAI,
		Chain: []models.ChainEntry{{ProviderID: provider.ID, Protocol: models.ProtocolOpenAI}},
	}
	body := `{"model":"test","input":"hello","max_output_tokens":32}`
	request := httptest.NewRequest(http.MethodPost, "/llm-gateway-lite/responses/v1/responses", strings.NewReader(body))
	response := httptest.NewRecorder()
	forwarder.HandleRequest(response, request, link, models.ProtocolOpenAI, models.FormatResponses)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"output"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	logs, err := database.ListRecentLogs(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || logs[0].UpstreamURL != "https://provider.test/v1/responses" || !strings.Contains(logs[0].UpstreamBody, `"input":"hello"`) {
		t.Fatalf("Responses request was not passed through natively: %+v", logs)
	}
}

type responsesTestAdapter struct{}

func (responsesTestAdapter) Type() string { return "responses-test" }

func (responsesTestAdapter) Forward(_ context.Context, provider providers.Provider, req providers.OpenAIReq, _, protocol string) providers.Result {
	if protocol != models.FormatResponses {
		return providers.Result{Err: fmt.Errorf("unexpected protocol %s", protocol)}
	}
	return providers.Result{
		StatusCode:  http.StatusOK,
		Body:        []byte(`{"id":"resp_1","object":"response","output":[{"type":"message"}]}`),
		RequestURL:  provider.BaseURL,
		RequestBody: req.Raw,
	}
}

func TestValidateChainRecordsLinkTestRequest(t *testing.T) {
	providers.Register(fallbackTestAdapter{})
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider := models.Provider{
		ID: "succeeded", Name: "validation-provider", Type: "fallback-test",
		BaseURL: "http://unused", Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "http://unused"}}, Models: []string{"test-model"}, Enabled: true, CreatedAt: time.Now(),
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}
	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{
		ID: "validation-link", Path: "validation-token", Attributes: models.Map{"env": "test"},
		Protocol: models.ProtocolOpenAI,
		Chain:    []models.ChainEntry{{ProviderID: provider.ID, Protocol: models.ProtocolOpenAI}},
	}

	validated := forwarder.ValidateChain(context.Background(), link)
	if validated.Chain[0].ValidationOK == nil || !*validated.Chain[0].ValidationOK {
		t.Fatalf("expected validation success: %+v", validated.Chain[0])
	}
	logs, err := database.ListRecentLogs(100)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 || !logs[0].Success || logs[0].Attributes["_request_type"] != "link_validation" {
		t.Fatalf("validation request was not recorded: %+v", logs)
	}
	if logs[0].RequestBody == "" || logs[0].ResponseBody == "" {
		t.Fatalf("validation request detail was not recorded: %+v", logs[0])
	}
	if !strings.Contains(logs[0].RequestBody, `"max_tokens":100`) {
		t.Fatalf("link validation output limit was not recorded: %s", logs[0].RequestBody)
	}
	latest, err := database.ListLatestValidationLogs()
	if err != nil {
		t.Fatal(err)
	}
	if len(latest) != 1 || intValue(latest[0].Attributes["_chain_position"]) != 0 {
		t.Fatalf("latest validation request was not addressable by chain node: %+v", latest)
	}
}

func intValue(value any) int {
	if number, ok := value.(float64); ok {
		return int(number)
	}
	return -1
}

func structResult(status int, body string) providers.Result {
	return providers.Result{StatusCode: status, Body: []byte(body)}
}
