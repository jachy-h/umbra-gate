package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/anomalyco/llm-gateway/config"
)

// gatewayProviderView is the JSON shape returned to clients. The plaintext
// api key is never serialized; api_key_source shows the literal stored in
// config.yaml (e.g. ${VOLC_KEY}) for round-trip awareness.
type gatewayProviderView struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	BaseURL      string `json:"base_url"`
	APIKey       string `json:"api_key"`
	APIKeySource string `json:"api_key_source"`
	HasAPIKey    bool   `json:"has_api_key"`
}

// gatewayProviderInput accepts both create and update payloads. For updates
// an empty api_key signals "leave unchanged".
type gatewayProviderInput struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
}

func (h *Handler) handleGatewayProviders(w http.ResponseWriter, r *http.Request) {
	if h.cfg == nil {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/gateway/providers")
	rest = strings.TrimPrefix(rest, "/")

	switch {
	case rest == "" && r.Method == http.MethodGet:
		h.listGatewayProviders(w, r)
	case rest == "" && r.Method == http.MethodPost:
		h.createGatewayProvider(w, r)
	case rest != "" && r.Method == http.MethodPut:
		h.updateGatewayProvider(w, r, rest)
	case rest != "" && r.Method == http.MethodDelete:
		h.deleteGatewayProvider(w, r, rest)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

func (h *Handler) listGatewayProviders(w http.ResponseWriter, _ *http.Request) {
	ids := h.cfg.ProviderIDs()
	out := make([]gatewayProviderView, 0, len(ids))
	for _, id := range ids {
		p, ok := h.cfg.Provider(id)
		if !ok {
			continue
		}
		out = append(out, toView(id, p))
	}
	writeJSON(w, out)
}

func (h *Handler) createGatewayProvider(w http.ResponseWriter, r *http.Request) {
	var in gatewayProviderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}
	if _, exists := h.cfg.Provider(id); exists {
		http.Error(w, `{"error":"provider already exists"}`, http.StatusConflict)
		return
	}
	if strings.TrimSpace(in.APIKey) == "" {
		http.Error(w, `{"error":"api_key is required"}`, http.StatusBadRequest)
		return
	}
	if err := upsertFromInput(h.cfg, id, in); err != nil {
		writeUpsertError(w, err)
		return
	}
	if err := h.cfg.Save(); err != nil {
		http.Error(w, `{"error":"failed to persist config"}`, http.StatusInternalServerError)
		return
	}
	p, _ := h.cfg.Provider(id)
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, toView(id, p))
}

func (h *Handler) updateGatewayProvider(w http.ResponseWriter, r *http.Request, id string) {
	current, ok := h.cfg.Provider(id)
	if !ok {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	var in gatewayProviderInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	// Empty api_key means "keep existing"
	if strings.TrimSpace(in.APIKey) == "" {
		in.APIKey = current.APIKeyRaw
	}
	if err := upsertFromInput(h.cfg, id, in); err != nil {
		writeUpsertError(w, err)
		return
	}
	if err := h.cfg.Save(); err != nil {
		http.Error(w, `{"error":"failed to persist config"}`, http.StatusInternalServerError)
		return
	}
	p, _ := h.cfg.Provider(id)
	writeJSON(w, toView(id, p))
}

func (h *Handler) deleteGatewayProvider(w http.ResponseWriter, _ *http.Request, id string) {
	if err := h.cfg.DeleteProvider(id); err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err := h.cfg.Save(); err != nil {
		http.Error(w, `{"error":"failed to persist config"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func upsertFromInput(cfg *config.Config, id string, in gatewayProviderInput) error {
	return cfg.UpsertProvider(id, config.ProviderConfig{
		Type:      config.ProviderType(strings.TrimSpace(in.Type)),
		BaseURL:   strings.TrimSpace(in.BaseURL),
		APIKey:    in.APIKey,
		APIKeyRaw: in.APIKey,
	})
}

func writeUpsertError(w http.ResponseWriter, err error) {
	if err == nil {
		return
	}
	payload := map[string]string{"error": err.Error()}
	data, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	w.Write(data)
}

func toView(id string, p config.ProviderConfig) gatewayProviderView {
	return gatewayProviderView{
		ID:           id,
		Type:         string(p.Type),
		BaseURL:      p.BaseURL,
		APIKey:       "",
		APIKeySource: p.APIKeyRaw,
		HasAPIKey:    strings.TrimSpace(p.APIKey) != "",
	}
}
