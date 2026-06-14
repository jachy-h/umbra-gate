package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/anomalyco/llm-gateway/config"
	"github.com/anomalyco/llm-gateway/db"
)

type Handler struct {
	db  *db.DB
	cfg *config.Config
}

// New builds a handler with full gateway provider management.
func New(database *db.DB, cfg *config.Config) *Handler {
	return &Handler{db: database, cfg: cfg}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/gateway/providers") {
		w.Header().Set("Content-Type", "application/json")
		h.handleGatewayProviders(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/stats":
		h.handleStats(w, r)
	case "/sessions":
		h.handleSessions(w, r)
	case "/models":
		h.handleModels(w, r)
	case "/providers":
		h.handleProviders(w, r)
	case "/timeseries":
		h.handleTimeSeries(w, r)
	default:
		idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
		if idStr != r.URL.Path {
			h.handleSessionDetail(w, r, idStr)
			return
		}
		http.NotFound(w, r)
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, stats)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil {
			offset = v
		}
	}

	sessions, err := h.db.ListSessions(limit, offset)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []db.Session{}
	}
	writeJSON(w, sessions)
}

func (h *Handler) handleSessionDetail(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	session, err := h.db.GetSession(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, session)
}

func (h *Handler) handleModels(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetModelStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.ModelStats{}
	}
	writeJSON(w, stats)
}

func (h *Handler) handleProviders(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetProviderStats()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.ProviderStats{}
	}
	writeJSON(w, stats)
}

func (h *Handler) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			days = parsed
		}
	}
	stats, err := h.db.GetTimeSeriesStats(days)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.TimeSeriesStats{}
	}
	writeJSON(w, stats)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
