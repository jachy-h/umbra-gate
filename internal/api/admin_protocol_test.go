package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
)

func TestCreateLinkRejectsMixedProtocolsAndPersistsFirstNodeProtocol(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	now := time.Now()
	providers := []models.Provider{
		{ID: "multi", Name: "Multi", Type: "custom", Endpoints: []models.ProviderEndpoint{
			{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "https://multi.test/v1"},
			{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatResponses, ResponseFormat: models.FormatResponses, BaseURL: "https://multi.test/v1/responses"},
		}, Enabled: true, CreatedAt: now},
		{ID: "anthropic-test", Name: "Anthropic Test", Type: "anthropic", Endpoints: []models.ProviderEndpoint{
			{Protocol: models.ProtocolAnthropic, RequestFormat: models.FormatMessages, ResponseFormat: models.FormatMessages, BaseURL: "https://anthropic.test"},
		}, Enabled: true, CreatedAt: now},
	}
	for _, provider := range providers {
		if err := database.UpsertProvider(provider); err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := &AdminAPI{DB: database}
	router.POST("/admin/links", admin.CreateLink)

	mixed := []byte(`{"name":"mixed","path":"mixed","chain":[{"provider_id":"multi","protocol":"openai"},{"provider_id":"anthropic-test","protocol":"anthropic"}]}`)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/links", bytes.NewReader(mixed)))
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "protocol mismatch") {
		t.Fatalf("mixed protocol status = %d, body = %s", response.Code, response.Body.String())
	}

	matching := []byte(`{"name":"openai","path":"openai","chain":[{"provider_id":"multi"}]}`)
	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/links", bytes.NewReader(matching)))
	if response.Code != http.StatusCreated {
		t.Fatalf("matching protocol status = %d, body = %s", response.Code, response.Body.String())
	}
	links, err := database.ListLinks()
	if err != nil {
		t.Fatal(err)
	}
	if len(links) != 1 || links[0].Protocol != models.ProtocolOpenAI || links[0].Chain[0].Protocol != models.ProtocolOpenAI {
		t.Fatalf("protocol was not persisted from first node: %+v", links)
	}
}

func TestUpdatingProviderEndpointsWithoutAPIKeyPreservesExistingKey(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "preserve-key", Name: "Preserve Key", Type: "custom", APIKey: "secret-key",
		Endpoints: []models.ProviderEndpoint{{Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions, ResponseFormat: models.FormatChatCompletions, BaseURL: "https://old.test/v1"}},
		Enabled:   true, CreatedAt: time.Now(),
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := &AdminAPI{DB: database}
	router.POST("/admin/providers", admin.CreateProvider)
	body := []byte(`{"id":"preserve-key","name":"Preserve Key","type":"custom","enabled":true,"endpoints":[{"protocol":"openai","request_format":"chat_completions","response_format":"chat_completions","base_url":"https://new.test/v1"}]}`)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/providers", bytes.NewReader(body)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	updated, err := database.GetProvider(provider.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.APIKey != "secret-key" || updated.Endpoints[0].BaseURL != "https://new.test/v1" {
		t.Fatalf("provider update lost credentials or endpoint: %+v", updated)
	}
}
