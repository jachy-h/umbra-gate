package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/config"
	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
)

func TestResponsesOperationURLCanBeUsedAsSDKBaseURL(t *testing.T) {
	providers.Register(responsesBaseURLTestAdapter{})

	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	provider := models.Provider{
		ID: "responses-upstream", Name: "Responses Upstream", Type: "responses-base-url-test",
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatResponses,
			ResponseFormat: models.FormatResponses, BaseURL: "https://provider.test/v1/responses",
		}},
		APIKey: "test-key", Enabled: true, CreatedAt: time.Now(),
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}
	link := models.ProxyLink{
		ID: "responses-link", Name: "Responses Link", Path: "token",
		Protocol: models.ProtocolOpenAI, Enabled: true, CreatedAt: time.Now(),
		Chain: []models.ChainEntry{{ProviderID: provider.ID, Protocol: models.ProtocolOpenAI}},
	}
	if err := database.SaveLink(link); err != nil {
		t.Fatal(err)
	}

	engine, _ := New(config.Config{}, database)
	request := httptest.NewRequest(
		http.MethodPost,
		"/llm-gateway-lite/token/v1/responses/responses",
		strings.NewReader(`{"model":"test","input":"hello"}`),
	)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	logs, err := database.ListRecentLogs(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(logs) != 1 ||
		logs[0].UpstreamURL != "https://provider.test/v1/responses" ||
		strings.Contains(logs[0].RequestURL, "/responses/responses") {
		t.Fatalf("request URL was not canonicalized: %+v", logs)
	}

	probeRequest := httptest.NewRequest(http.MethodGet, "/llm-gateway-lite/token/v1/responses", nil)
	probeResponse := httptest.NewRecorder()
	engine.ServeHTTP(probeResponse, probeRequest)
	if probeResponse.Code != http.StatusOK ||
		!strings.Contains(probeResponse.Body.String(), `"/llm-gateway-lite/token/v1/responses"`) {
		t.Fatalf("responses URL probe: status = %d, body = %s", probeResponse.Code, probeResponse.Body.String())
	}
}

type responsesBaseURLTestAdapter struct{}

func (responsesBaseURLTestAdapter) Type() string { return "responses-base-url-test" }

func (responsesBaseURLTestAdapter) Forward(
	_ context.Context,
	provider providers.Provider,
	request providers.OpenAIReq,
	_, _ string,
) providers.Result {
	return providers.Result{
		StatusCode:  http.StatusOK,
		Body:        []byte(`{"id":"resp_1","object":"response","output":[{"type":"message"}]}`),
		RequestURL:  provider.BaseURL,
		RequestBody: request.Raw,
	}
}
