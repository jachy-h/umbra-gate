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

func TestCreateLinkValidatesModelPrioritiesAndRejectsLinkAPIKey(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "provider", Name: "Provider", Type: "custom", Enabled: true, CreatedAt: time.Now(),
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
			ResponseFormat: models.FormatChatCompletions, BaseURL: "https://provider.test/v1",
		}},
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/links", (&AdminAPI{DB: database}).CreateLink)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"empty", `{"name":"link","path":"empty","chain":[{"provider_id":"provider"}]}`, "model_priorities"},
		{"request model with value", `{"name":"link","path":"request-value","chain":[{"provider_id":"provider","model_priorities":[{"source":"request_model","model":"client"}]}]}`, "must not include model"},
		{"duplicate request model", `{"name":"link","path":"duplicate-request","chain":[{"provider_id":"provider","model_priorities":[{"source":"request_model"},{"source":"request_model"}]}]}`, "may appear only once"},
		{"fixed model missing value", `{"name":"link","path":"fixed-empty","chain":[{"provider_id":"provider","model_priorities":[{"source":"fixed_model","model":" "}]}]}`, "requires model"},
		{"unknown source", `{"name":"link","path":"unknown-source","chain":[{"provider_id":"provider","model_priorities":[{"source":"unknown"}]}]}`, "unsupported source"},
		{"link api key", `{"name":"link","path":"link-api-key","chain":[{"provider_id":"provider","api_key":"forbidden","model_priorities":[{"source":"request_model"}]}]}`, "API key must be configured on the provider"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/links", bytes.NewBufferString(tc.body)))
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), tc.want) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}

	valid := `{"name":"link","path":"valid","chain":[{"provider_id":"provider","model_priorities":[{"source":"request_model"},{"source":"fixed_model","model":" fixed-model "}]}]}`
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/links", bytes.NewBufferString(valid)))
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	link, err := database.GetLinkByPath("valid")
	if err != nil {
		t.Fatal(err)
	}
	if got := link.Chain[0].ModelPriorities; len(got) != 2 || got[1].Model != "fixed-model" {
		t.Fatalf("priorities = %#v", got)
	}
}

func TestRefreshProviderModelsPersistsOpenAICompatibleCatalog(t *testing.T) {
	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("path = %s, want /v1/models", r.URL.Path)
		}
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"model-a"},{"id":" model-b "},{"id":"model-a"},{"id":""}]}`))
	}))
	defer upstream.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "provider", Name: "Provider", Type: "custom", APIKey: "provider-secret", Enabled: true, CreatedAt: time.Now(),
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
			ResponseFormat: models.FormatChatCompletions, BaseURL: upstream.URL + "/v1",
		}},
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/providers/:id/models/refresh", (&AdminAPI{DB: database, HTTPClient: upstream.Client()}).RefreshProviderModels)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/providers/provider/models/refresh", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "provider-secret") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if authorization != "Bearer provider-secret" {
		t.Fatalf("authorization = %q", authorization)
	}
	stored, err := database.GetProvider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(stored.Models, ","), "model-a,model-b"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}
}

func TestRefreshProviderModelsClearsStaleCatalogOnFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer upstream.Close()

	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "provider", Name: "Provider", Type: "custom", Models: []string{"stale-model"}, Enabled: true, CreatedAt: time.Now(),
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
			ResponseFormat: models.FormatChatCompletions, BaseURL: upstream.URL + "/v1",
		}},
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/providers/:id/models/refresh", (&AdminAPI{DB: database, HTTPClient: upstream.Client()}).RefreshProviderModels)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/admin/providers/provider/models/refresh", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	stored, err := database.GetProvider("provider")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Models) != 0 {
		t.Fatalf("stale models were retained: %#v", stored.Models)
	}
}

func TestUpdateProviderAPIKeyReturnsOnlyMaskedProvider(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	provider := models.Provider{
		ID: "provider", Name: "Provider", Type: "custom", Enabled: true, CreatedAt: time.Now(),
		Endpoints: []models.ProviderEndpoint{{
			Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
			ResponseFormat: models.FormatChatCompletions, BaseURL: "https://provider.test/v1",
		}},
	}
	if err := database.UpsertProvider(provider); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	admin := &AdminAPI{DB: database}
	router.PUT("/admin/providers/:id/api-key", admin.UpdateProviderAPIKey)
	router.GET("/admin/providers/:id", admin.GetProvider)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/admin/providers/provider/api-key", bytes.NewBufferString(`{"api_key":"provider-secret"}`)))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "provider-secret") || !strings.Contains(response.Body.String(), `"has_api_key":true`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	stored, err := database.GetProvider("provider")
	if err != nil || stored.APIKey != "provider-secret" {
		t.Fatalf("stored provider = %+v, err = %v", stored, err)
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/admin/providers/provider", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "provider-secret") || !strings.Contains(response.Body.String(), `"has_api_key":true`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/admin/providers/missing/api-key", bytes.NewBufferString(`{"api_key":"secret"}`)))
	if response.Code != http.StatusNotFound {
		t.Fatalf("missing provider status = %d, body = %s", response.Code, response.Body.String())
	}
}
