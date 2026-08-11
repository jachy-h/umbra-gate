package proxy

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/db"
	"github.com/jachy-h/llm-gateway-lite/internal/models"
	"github.com/jachy-h/llm-gateway-lite/internal/providers"
	"github.com/jachy-h/llm-gateway-lite/internal/stats"
)

type modelPriorityAdapter struct {
	attempts  *[]string
	succeedOn string
}

func (modelPriorityAdapter) Type() string { return "model-priority-test" }

func (a modelPriorityAdapter) Forward(_ context.Context, provider providers.Provider, _ providers.OpenAIReq, model, _ string) providers.Result {
	*a.attempts = append(*a.attempts, provider.ID+"/"+model)
	if model == a.succeedOn {
		return providers.Result{StatusCode: http.StatusOK, Body: []byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)}
	}
	return providers.Result{StatusCode: http.StatusBadRequest, Body: []byte(`{"error":{"message":"model unavailable"}}`)}
}

func TestHandleUsesModelPrioritiesInConfiguredOrder(t *testing.T) {
	attempts := []string{}
	providers.Register(modelPriorityAdapter{attempts: &attempts, succeedOn: "provider-2-a"})
	database := modelPriorityTestDatabase(t, "provider-1", "provider-2")
	defer database.Close()

	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{
		ID: "link", Path: "token", Protocol: models.ProtocolOpenAI,
		Chain: []models.ChainEntry{
			{ProviderID: "provider-1", Protocol: models.ProtocolOpenAI, ModelPriorities: []models.ModelPriority{
				{Source: models.ModelPriorityFixedModel, Model: "provider-1-a"},
				{Source: models.ModelPriorityRequestModel},
				{Source: models.ModelPriorityFixedModel, Model: "provider-1-b"},
			}},
			{ProviderID: "provider-2", Protocol: models.ProtocolOpenAI, ModelPriorities: []models.ModelPriority{
				{Source: models.ModelPriorityFixedModel, Model: "provider-2-a"},
				{Source: models.ModelPriorityRequestModel},
			}},
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/llm-gateway-lite/token/v1/chat/completions", bytes.NewBufferString(`{"model":"client-model","messages":[{"role":"user","content":"hello"}]}`))
	response := httptest.NewRecorder()

	forwarder.Handle(response, request, link)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	want := []string{"provider-1/provider-1-a", "provider-1/client-model", "provider-1/provider-1-b", "provider-2/provider-2-a"}
	if !reflect.DeepEqual(attempts, want) {
		t.Fatalf("attempts = %v, want %v", attempts, want)
	}
}

func TestHandleDeduplicatesModelsAndRetriesBeforeNextPriority(t *testing.T) {
	attempts := []string{}
	providers.Register(modelPriorityAdapter{attempts: &attempts, succeedOn: "next-model"})
	database := modelPriorityTestDatabase(t, "provider")
	defer database.Close()

	forwarder := &Forwarder{DB: database, Stats: stats.New(database)}
	link := models.ProxyLink{
		ID: "link", Path: "token", Protocol: models.ProtocolOpenAI,
		Chain: []models.ChainEntry{{
			ProviderID: "provider", Protocol: models.ProtocolOpenAI, RetryCount: 1,
			ModelPriorities: []models.ModelPriority{
				{Source: models.ModelPriorityRequestModel},
				{Source: models.ModelPriorityFixedModel, Model: "client-model"},
				{Source: models.ModelPriorityFixedModel, Model: "next-model"},
			},
		}},
	}
	request := httptest.NewRequest(http.MethodPost, "/llm-gateway-lite/token/v1/chat/completions", bytes.NewBufferString(`{"model":"client-model","messages":[{"role":"user","content":"hello"}]}`))
	response := httptest.NewRecorder()

	forwarder.Handle(response, request, link)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	want := []string{"provider/client-model", "provider/client-model", "provider/next-model"}
	if !reflect.DeepEqual(attempts, want) {
		t.Fatalf("attempts = %v, want %v", attempts, want)
	}
}

func modelPriorityTestDatabase(t *testing.T, ids ...string) *db.DB {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "gateway.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		provider := models.Provider{
			ID: id, Name: id, Type: "model-priority-test", Enabled: true, CreatedAt: time.Now(),
			Endpoints: []models.ProviderEndpoint{{
				Protocol: models.ProtocolOpenAI, RequestFormat: models.FormatChatCompletions,
				ResponseFormat: models.FormatChatCompletions, BaseURL: "https://provider.test/v1",
			}},
		}
		if err := database.UpsertProvider(provider); err != nil {
			database.Close()
			t.Fatal(err)
		}
	}
	return database
}
