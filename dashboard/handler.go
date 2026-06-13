package dashboard

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/anomalyco/llm-gateway/db"
)

//go:embed templates/*
var templateFS embed.FS

type Handler struct {
	db   *db.DB
	tmpl *template.Template
}

func New(database *db.DB) *Handler {
	funcMap := template.FuncMap{
		"formatNum": func(n int64) string {
			if n >= 1000000 {
				return fmt.Sprintf("%.1fM", float64(n)/1000000)
			}
			if n >= 1000 {
				return fmt.Sprintf("%.1fK", float64(n)/1000)
			}
			return fmt.Sprintf("%d", n)
		},
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	return &Handler{db: database, tmpl: tmpl}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/dashboard")
	path = strings.TrimPrefix(path, "/")

	if path == "" || path == "/" {
		h.home(w, r)
		return
	}

	if path == "sessions" {
		h.sessions(w, r)
		return
	}

	if strings.HasPrefix(path, "sessions/") {
		h.sessionDetail(w, r)
		return
	}

	if path == "models" {
		h.models(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	stats, _ := h.db.GetStats()
	data := map[string]interface{}{
		"Active": "home",
		"Stats":  stats,
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "sessions",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) sessionDetail(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "sessions",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Active": "models",
	}
	h.tmpl.ExecuteTemplate(w, "layout.html", data)
}
