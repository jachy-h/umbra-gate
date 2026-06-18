package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
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
	case "/overview":
		h.handleOverview(w, r)
	case "/sessions":
		h.handleSessions(w, r)
	case "/models":
		h.handleModels(w, r)
	case "/models/analytics":
		h.handleModelAnalytics(w, r)
	case "/providers":
		h.handleProviders(w, r)
	case "/providers/analytics":
		h.handleProviderAnalytics(w, r)
	case "/timeseries":
		h.handleTimeSeries(w, r)
	case "/latency":
		h.handleLatency(w, r)
	case "/failures":
		h.handleFailures(w, r)
	case "/logs":
		h.handleRecentLogs(w, r)
	default:
		if idStr := strings.TrimPrefix(r.URL.Path, "/sessions/"); idStr != r.URL.Path {
			if strings.HasSuffix(idStr, "/log") {
				h.handleSessionLog(w, r, strings.TrimSuffix(idStr, "/log"))
				return
			}
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

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := h.db.GetOverviewStats(r.URL.Query().Get("range"))
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, overview)
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

func (h *Handler) handleProviderAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.db.GetProviderAnalytics(r.URL.Query().Get("range"))
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if analytics == nil {
		analytics = []db.ProviderAnalytics{}
	}
	writeJSON(w, analytics)
}

func (h *Handler) handleModelAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.db.GetModelAnalytics(r.URL.Query().Get("range"))
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if analytics == nil {
		analytics = []db.ModelAnalytics{}
	}
	writeJSON(w, analytics)
}

func (h *Handler) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	var stats []db.TimeSeriesStats
	var err error
	if rawRange := r.URL.Query().Get("range"); rawRange != "" {
		stats, err = h.db.GetTimeSeriesStatsForRange(rawRange)
	} else {
		days := 7
		if raw := r.URL.Query().Get("days"); raw != "" {
			if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
				days = parsed
			}
		}
		stats, err = h.db.GetTimeSeriesStats(days)
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if stats == nil {
		stats = []db.TimeSeriesStats{}
	}
	writeJSON(w, stats)
}

func (h *Handler) handleLatency(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.db.GetLatencyAnalytics(r.URL.Query().Get("range"), r.URL.Query().Get("by"))
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if analytics == nil {
		analytics = []db.LatencyAnalytics{}
	}
	writeJSON(w, analytics)
}

func (h *Handler) handleFailures(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.db.GetFailureAnalytics(r.URL.Query().Get("range"))
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, analytics)
}

func (h *Handler) handleSessionLog(w http.ResponseWriter, r *http.Request, idStr string) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	log, err := h.db.GetRequestLogBySession(id)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	writeJSON(w, log)
}

func (h *Handler) handleRecentLogs(w http.ResponseWriter, r *http.Request) {
	logs, err := h.db.ListRecentRequestLogs()
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if logs == nil {
		logs = []db.RequestLog{}
	}
	writeJSON(w, logs)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
