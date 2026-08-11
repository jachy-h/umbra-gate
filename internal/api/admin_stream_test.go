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
	"github.com/jachy-h/llm-gateway-lite/internal/proxy"
)

func TestCreateLinkStreamEmitsProviderProgressInChainOrder(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	for _, id := range []string{"first", "second"} {
		provider := models.Provider{
			ID: id, Name: id, Type: "custom", Enabled: true, CreatedAt: time.Now(),
			Endpoints: []models.ProviderEndpoint{{
				Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
				ResponseFormat: models.FormatChatCompletions, BaseURL: "https://provider.test/v1",
			}},
		}
		if err := database.UpsertProvider(provider); err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/admin/links/stream", (&AdminAPI{
		DB: database, Forwarder: &proxy.Forwarder{DB: database},
	}).CreateLinkStream)

	body := []byte(`{"name":"stream","path":"stream","chain":[{"provider_id":"first","protocol":"openai","model_priorities":[{"source":"request_model"}]},{"provider_id":"second","protocol":"openai","model_priorities":[{"source":"request_model"}]}]}`)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/admin/links/stream", bytes.NewReader(body))
	request.Header.Set("Accept", "text/event-stream")
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("content type = %q", contentType)
	}
	stream := response.Body.String()
	first := strings.Index(stream, `event:provider`)
	second := strings.Index(stream[first+1:], `event:provider`)
	if first < 0 || second < 0 || !strings.Contains(stream, `"chain_index":0`) || !strings.Contains(stream, `"chain_index":1`) {
		t.Fatalf("missing ordered provider progress events: %s", stream)
	}
	if !strings.Contains(stream, "event:complete") {
		t.Fatalf("missing completion event: %s", stream)
	}
}
