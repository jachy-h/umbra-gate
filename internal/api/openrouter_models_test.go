package api

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func TestProviderModelsURLUsesOpenRouterLimit(t *testing.T) {
	openRouterURL, err := providerModelsURL("https://openrouter.ai/api/v1", "openrouter")
	if err != nil {
		t.Fatal(err)
	}
	if openRouterURL != "https://openrouter.ai/api/v1/models?limit=500" {
		t.Fatalf("OpenRouter models URL = %q", openRouterURL)
	}
	genericURL, err := providerModelsURL("https://api.deepseek.com", "deepseek")
	if err != nil {
		t.Fatal(err)
	}
	if genericURL != "https://api.deepseek.com/models" {
		t.Fatalf("generic models URL = %q", genericURL)
	}
}

func TestRefreshOpenRouterModelsRequestsLimitParam(t *testing.T) {
	var requestQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":[{"id":"openai/gpt-4o"},{"id":"anthropic/claude-3.5-sonnet"}]}`))
	}))
	defer upstream.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "openrouter", Name: "OpenRouter", Type: "openrouter", APIKey: "sk-or-v1-test",
		Enabled: true, CreatedAt: time.Now(),
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
			ResponseFormat: models.FormatChatCompletions, BaseURL: upstream.URL + "/api/v1",
		}},
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/providers/:id/models/refresh", (&AdminAPI{DB: database, HTTPClient: upstream.Client()}).RefreshProviderModels)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/providers/openrouter/models/refresh", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if requestQuery != "limit=500" {
		t.Fatalf("upstream query = %q, want limit=500", requestQuery)
	}
	stored, err := database.GetProvider("openrouter")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Models) != 2 || stored.Models[0] != "openai/gpt-4o" || stored.Models[1] != "anthropic/claude-3.5-sonnet" {
		t.Fatalf("models = %v", stored.Models)
	}
}
