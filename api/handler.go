package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/anomalyco/llm-gateway/db"
)

type Handler struct {
	db *db.DB
}

func New(database *db.DB) *Handler {
	return &Handler{db: database}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/stats":
		h.handleStats(w, r)
	case "/sessions":
		h.handleSessions(w, r)
	case "/models":
		h.handleModels(w, r)
	default:
		if len(r.URL.Path) > 10 && r.URL.Path[:10] == "/sessions/" {
			h.handleSessionDetail(w, r)
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
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		offset, _ = strconv.Atoi(o)
	}

	sessions, err := h.db.ListSessions(limit, offset)
	if err != nil {
		http.Error(w, `{"error":"internal error"}`, http.StatusInternalServerError)
		return
	}
	if sessions == nil {
		sessions = []db.Session{}
	}
	json.NewEncoder(w).Encode(sessions)
}

func (h *Handler) handleSessionDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[10:]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}

	session, err := h.db.GetSession(id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(session)
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
	json.NewEncoder(w).Encode(stats)
}
