package dashboard

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"os/exec"
	"sort"
	"strings"

	"github.com/jachy-h/umbra-gate/codexconfig"
	"github.com/jachy-h/umbra-gate/config"
	"github.com/jachy-h/umbra-gate/db"
	"github.com/jachy-h/umbra-gate/opencodeconfig"
)

//go:embed templates/* static/* static/dashboard/*
var templateFS embed.FS

type pageData struct {
	Active string
	Stats  *db.Stats
}

type Options struct {
	OpencodeConfigPath  string
	CodexConfigPath     string
	ProviderListCommand func() ([]byte, error)
	GatewayBaseURL      string
	GatewayConfig       *config.Config
}

type Handler struct {
	db                  *db.DB
	templates           map[string]*template.Template
	opencode            opencodeconfig.Manager
	codex               codexconfig.Manager
	providerListCommand func() ([]byte, error)
	gatewayBaseURL      string
	gatewayCfg          *config.Config
}

func New(database *db.DB, gatewayCfg *config.Config) *Handler {
	return newWithOptions(database, Options{GatewayConfig: gatewayCfg})
}

func newWithOptions(database *db.DB, options Options) *Handler {
	funcMap := template.FuncMap{
		"formatNum": func(n int64) string {
			if n >= 1000000 {
				v := float64(n) / 1000000
				return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".") + "M"
			}
			if n >= 1000 {
				v := float64(n) / 1000
				return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.1f", v), "0"), ".") + "K"
			}
			return fmt.Sprintf("%d", n)
		},
	}

	templates := map[string]*template.Template{
		"home":           template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/home.html")),
		"sessions":       template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/sessions.html")),
		"session_detail": template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/session_detail.html")),
		"models":         template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/models.html")),
		"agents":         template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/agents.html")),
		"analytics":      template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/analytics.html")),
		"providers":      template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/providers.html")),
		"failures":       template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/layout.html", "templates/failures.html")),
	}
	providerListCommand := options.ProviderListCommand
	if providerListCommand == nil {
		providerListCommand = defaultProviderListCommand
	}
	gatewayBaseURL := options.GatewayBaseURL
	if gatewayBaseURL == "" {
		gatewayBaseURL = "http://127.0.0.1:4141"
	}
	return &Handler{
		db:                  database,
		templates:           templates,
		opencode:            opencodeconfig.Manager{Path: options.OpencodeConfigPath},
		codex:               codexconfig.Manager{Path: options.CodexConfigPath},
		providerListCommand: providerListCommand,
		gatewayBaseURL:      gatewayBaseURL,
		gatewayCfg:          options.GatewayConfig,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/dashboard")
	path = strings.TrimPrefix(path, "/")

	if strings.HasPrefix(path, "static/") {
		http.StripPrefix("/dashboard/", http.FileServer(http.FS(templateFS))).ServeHTTP(w, r)
		return
	}

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

	if path == "agents" {
		h.agents(w, r)
		return
	}

	if path == "analytics" {
		h.analytics(w, r)
		return
	}

	if path == "providers" {
		h.providers(w, r)
		return
	}

	if path == "failures" {
		h.failures(w, r)
		return
	}

	if path == "providers/config" {
		h.providerConfig(w, r)
		return
	}

	if path == "providers/diff" {
		h.providerDiff(w, r)
		return
	}

	if path == "providers/apply" {
		h.providerApply(w, r)
		return
	}

	if path == "providers/gateway" {
		h.providerGateway(w, r)
		return
	}

	if path == "codex/config" {
		h.codexConfig(w, r)
		return
	}

	if path == "codex/diff" {
		h.codexDiff(w, r)
		return
	}

	if path == "codex/apply" {
		h.codexApply(w, r)
		return
	}

	if path == "codex/gateway" {
		h.codexGateway(w, r)
		return
	}

	http.NotFound(w, r)
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetStats()
	if err != nil {
		slog.Error("failed to get stats", "error", err)
	}
	h.render(w, "home", pageData{Active: "home", Stats: stats})
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	h.render(w, "sessions", pageData{Active: "sessions"})
}

func (h *Handler) sessionDetail(w http.ResponseWriter, r *http.Request) {
	h.render(w, "session_detail", pageData{Active: "sessions"})
}

func (h *Handler) models(w http.ResponseWriter, r *http.Request) {
	h.render(w, "models", pageData{Active: "models"})
}

func (h *Handler) agents(w http.ResponseWriter, r *http.Request) {
	h.render(w, "agents", pageData{Active: "agents"})
}

func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) {
	h.render(w, "analytics", pageData{Active: "analytics"})
}

func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	h.render(w, "providers", pageData{Active: "providers"})
}

func (h *Handler) failures(w http.ResponseWriter, r *http.Request) {
	h.render(w, "failures", pageData{Active: "failures"})
}

func (h *Handler) render(w http.ResponseWriter, name string, data pageData) {
	tmpl, ok := h.templates[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout.html", data); err != nil {
		slog.Error("failed to render template", "error", err)
	}
}

type providerListEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type providerConfigResponse struct {
	Files     []opencodeconfig.ConfigFile `json:"files"`
	Providers []providerStatus            `json:"providers"`
	Config    map[string]any              `json:"config"`
}

type providerStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BuiltIn        bool   `json:"built_in"`
	Configured     bool   `json:"configured"`
	HasAPIKey      bool   `json:"has_api_key"`
	GatewayEnabled bool   `json:"gateway_enabled"`
}

type providerPlanRequest struct {
	opencodeconfig.ProviderInput
	Path string `json:"path"`
}

type providerApplyRequest struct {
	opencodeconfig.ProviderInput
	Path         string `json:"path"`
	BaseChecksum string `json:"base_checksum"`
}

type codexConfigResponse struct {
	Files     []codexconfig.ConfigFile     `json:"files"`
	Providers []codexconfig.ProviderStatus `json:"providers"`
}

type codexPlanRequest struct {
	codexconfig.ProviderInput
	Path string `json:"path"`
}

type codexApplyRequest struct {
	codexconfig.ProviderInput
	Path         string `json:"path"`
	BaseChecksum string `json:"base_checksum"`
}

func (h *Handler) providerConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	files := opencodeconfig.Discover(opencodeconfig.DefaultDir())
	manager := h.opencode
	if manager.Path != "" {
		files = []opencodeconfig.ConfigFile{{Path: manager.Path, Label: manager.Path, Exists: true, Selected: true}}
	} else if len(files) > 0 {
		manager.Path = files[0].Path
	}
	cfg, err := manager.MaskedConfig()
	if err != nil {
		http.Error(w, `{"error":"failed to read opencode config"}`, http.StatusInternalServerError)
		return
	}
	providerList, err := h.opencodeProviders()
	if err != nil {
		http.Error(w, `{"error":"failed to list opencode providers"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, providerConfigResponse{Files: files, Providers: providerStatuses(cfg, providerList, h.gatewayBaseURL), Config: cfg})
}

func (h *Handler) codexConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	files := codexconfig.Discover()
	manager := h.codex
	if manager.Path != "" {
		files = []codexconfig.ConfigFile{{Path: manager.Path, Label: manager.Path, Exists: true, Selected: true}}
	} else if len(files) > 0 {
		manager.Path = files[0].Path
	}
	statuses, err := manager.Statuses(h.codexProviderIDs(), h.gatewayBaseURL)
	if err != nil {
		http.Error(w, `{"error":"failed to read codex config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, codexConfigResponse{Files: files, Providers: statuses})
}

func (h *Handler) codexDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input codexPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	manager := h.codexManagerForPath(input.Path)
	input.ProviderInput = h.withCodexGatewayBaseURL(input.ProviderInput)
	plan, err := manager.Plan(input.ProviderInput)
	if err != nil {
		http.Error(w, `{"error":"failed to plan codex config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, plan)
}

func (h *Handler) codexApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input codexApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	manager := h.codexManagerForPath(input.Path)
	input.ProviderInput = h.withCodexGatewayBaseURL(input.ProviderInput)
	if err := manager.Apply(input.ProviderInput, input.BaseChecksum); err != nil {
		if errors.Is(err, codexconfig.ErrStaleConfig) {
			http.Error(w, `{"error":"stale codex config"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to apply codex config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) providerDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input providerPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	manager := h.managerForPath(input.Path)
	input.ProviderInput = h.withGatewayBaseURL(input.ProviderInput)
	plan, err := manager.Plan(input.ProviderInput)
	if err != nil {
		http.Error(w, `{"error":"failed to plan opencode config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, plan)
}

func (h *Handler) providerApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input providerApplyRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	manager := h.managerForPath(input.Path)
	input.ProviderInput = h.withGatewayBaseURL(input.ProviderInput)
	if err := manager.Apply(input.ProviderInput, input.BaseChecksum); err != nil {
		if errors.Is(err, opencodeconfig.ErrStaleConfig) {
			http.Error(w, `{"error":"stale opencode config"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to apply opencode config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) syncGatewayConfig(manager opencodeconfig.Manager, id string, enabled bool) {
	if h.gatewayCfg == nil {
		return
	}
	if !enabled {
		_ = h.gatewayCfg.DeleteProvider(id)
		_ = h.gatewayCfg.Save()
		return
	}
	ocCfg, _, err := manager.Load()
	if err != nil {
		slog.Warn("failed to load opencode config for gateway sync", "error", err)
		return
	}
	providers, _ := ocCfg["provider"].(map[string]any)
	if providers == nil {
		return
	}
	provider, _ := providers[id].(map[string]any)
	if provider == nil {
		return
	}
	options, _ := provider["options"].(map[string]any)
	origBaseURL := ""
	if options != nil {
		if bu, ok := options["baseURL"].(string); ok {
			origBaseURL = bu
		}
	}
	if origBaseURL == "" || gatewayURLMatches(origBaseURL, h.gatewayBaseURL, id) {
		if _, exists := h.gatewayCfg.Provider(id); exists {
			return
		}
		slog.Warn("cannot determine upstream URL for gateway provider; add it to config.yaml manually", "provider", id)
		return
	}
	if err := h.gatewayCfg.UpsertProvider(id, config.ProviderConfig{
		BaseURL: origBaseURL,
	}); err != nil {
		slog.Warn("failed to register gateway provider", "error", err)
		return
	}
	if err := h.gatewayCfg.Save(); err != nil {
		slog.Warn("failed to save gateway config", "error", err)
	}
}

type gatewayToggleRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

func (h *Handler) providerGateway(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input gatewayToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}
	manager := h.managerForPath(input.Path)
	h.syncGatewayConfig(manager, id, input.Enabled)
	plan, err := manager.Plan(opencodeconfig.ProviderInput{ID: id})
	if err != nil {
		http.Error(w, `{"error":"failed to read opencode config"}`, http.StatusInternalServerError)
		return
	}
	gateway := opencodeconfig.GatewayDisable
	if input.Enabled {
		gateway = opencodeconfig.GatewayEnable
	}
	provider := opencodeconfig.ProviderInput{
		ID:             id,
		Gateway:        gateway,
		GatewayBaseURL: h.gatewayBaseURL,
	}
	if err := manager.Apply(provider, plan.BaseChecksum); err != nil {
		http.Error(w, `{"error":"failed to apply opencode config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) codexGateway(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	var input gatewayToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		http.Error(w, `{"error":"id is required"}`, http.StatusBadRequest)
		return
	}
	if input.Enabled {
		h.ensureCodexGatewayProvider(id)
	}
	manager := h.codexManagerForPath(input.Path)
	plan, err := manager.Plan(codexconfig.ProviderInput{ID: id})
	if err != nil {
		http.Error(w, `{"error":"failed to read codex config"}`, http.StatusInternalServerError)
		return
	}
	gateway := codexconfig.GatewayDisable
	if input.Enabled {
		gateway = codexconfig.GatewayEnable
	}
	provider := codexconfig.ProviderInput{
		ID:             id,
		Gateway:        gateway,
		GatewayBaseURL: h.gatewayBaseURL,
	}
	if err := manager.Apply(provider, plan.BaseChecksum); err != nil {
		http.Error(w, `{"error":"failed to apply codex config"}`, http.StatusInternalServerError)
		return
	}
	writeDashboardJSON(w, map[string]bool{"ok": true})
}

func (h *Handler) ensureCodexGatewayProvider(id string) {
	if h.gatewayCfg == nil {
		return
	}
	if _, exists := h.gatewayCfg.Provider(id); exists {
		return
	}
	baseURL := defaultCodexUpstreamURL(id)
	if baseURL == "" {
		slog.Warn("cannot determine upstream URL for codex gateway provider; add it to config.yaml manually", "provider", id)
		return
	}
	if err := h.gatewayCfg.UpsertProvider(id, config.ProviderConfig{BaseURL: baseURL}); err != nil {
		slog.Warn("failed to register codex gateway provider", "error", err)
		return
	}
	if err := h.gatewayCfg.Save(); err != nil {
		slog.Warn("failed to save gateway config", "error", err)
	}
}

func defaultCodexUpstreamURL(id string) string {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "openai":
		return "https://api.openai.com"
	default:
		return ""
	}
}

func providerStatuses(cfg map[string]any, providerList []providerListEntry, gatewayBaseURL string) []providerStatus {
	statuses := map[string]providerStatus{}
	for _, entry := range providerList {
		statuses[entry.ID] = providerStatus{ID: entry.ID, Name: normalizeProviderDisplayName(entry.Name), BuiltIn: true}
	}
	// Build lookup: lowercase key -> id, slug -> id, lowercase name -> id, slug name -> id
	providers, _ := cfg["provider"].(map[string]any)
	configLookup := map[string]string{}
	for id, raw := range providers {
		configLookup[strings.ToLower(id)] = id
		configLookup[slugify(id)] = id
		p, _ := raw.(map[string]any)
		if name, ok := p["name"].(string); ok && name != "" {
			configLookup[strings.ToLower(name)] = id
			configLookup[slugify(name)] = id
		}
	}
	for _, entry := range providerList {
		cfgID, ok := configLookup[strings.ToLower(entry.ID)]
		if !ok {
			cfgID, ok = configLookup[slugify(entry.ID)]
		}
		if !ok {
			cfgID, ok = configLookup[strings.ToLower(entry.Name)]
		}
		if !ok {
			cfgID, ok = configLookup[slugify(entry.Name)]
		}
		if !ok {
			cfgID, ok = configLookup[canonicalProviderID(entry.Name)]
		}
		if !ok {
			continue
		}
		raw := providers[cfgID]
		status := statuses[entry.ID]
		status.ID = cfgID
		status.Configured = true
		provider, _ := raw.(map[string]any)
		if name, ok := provider["name"].(string); ok && name != "" {
			status.Name = normalizeProviderDisplayName(name)
		}
		if status.Name == "" {
			status.Name = cfgID
		}
		options, _ := provider["options"].(map[string]any)
		if apiKey, ok := options["apiKey"].(string); ok && apiKey != "" {
			status.HasAPIKey = true
		}
		if baseURL, ok := options["baseURL"].(string); ok && gatewayURLMatches(baseURL, gatewayBaseURL, cfgID) {
			status.GatewayEnabled = true
		}
		statuses[entry.ID] = status
	}
	ids := make([]string, 0, len(statuses))
	for id := range statuses {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]providerStatus, 0, len(ids))
	for _, id := range ids {
		result = append(result, statuses[id])
	}
	return result
}

func gatewayURLMatches(baseURL, gatewayBaseURL, cfgID string) bool {
	current := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	prefix := strings.TrimRight(strings.TrimSpace(gatewayBaseURL), "/") + "/"
	escapedID := url.PathEscape(cfgID)
	return current == prefix+"a/opencode/"+escapedID ||
		current == prefix+cfgID ||
		current == prefix+escapedID
}

func (h *Handler) withGatewayBaseURL(input opencodeconfig.ProviderInput) opencodeconfig.ProviderInput {
	if input.GatewayBaseURL == "" {
		input.GatewayBaseURL = h.gatewayBaseURL
	}
	return input
}

func (h *Handler) withCodexGatewayBaseURL(input codexconfig.ProviderInput) codexconfig.ProviderInput {
	if input.GatewayBaseURL == "" {
		input.GatewayBaseURL = h.gatewayBaseURL
	}
	return input
}

func (h *Handler) managerForPath(path string) opencodeconfig.Manager {
	if path != "" {
		return opencodeconfig.Manager{Path: path}
	}
	return h.opencode
}

func (h *Handler) codexManagerForPath(path string) codexconfig.Manager {
	if path != "" {
		return codexconfig.Manager{Path: path}
	}
	return h.codex
}

func (h *Handler) codexProviderIDs() []string {
	if h.gatewayCfg == nil {
		return []string{codexconfig.DefaultProviderID}
	}
	ids := h.gatewayCfg.ProviderIDs()
	if len(ids) == 0 {
		return []string{codexconfig.DefaultProviderID}
	}
	return ids
}

func (h *Handler) opencodeProviders() ([]providerListEntry, error) {
	output, err := h.providerListCommand()
	if err != nil {
		return nil, err
	}
	return parseProviderList(output), nil
}

func defaultProviderListCommand() ([]byte, error) {
	return exec.Command("opencode", "providers", "list").Output()
}

func parseProviderList(output []byte) []providerListEntry {
	var jsonRows []providerListEntry
	if err := json.Unmarshal(output, &jsonRows); err == nil {
		return normalizeProviderList(jsonRows)
	}
	text := stripANSICodes(string(output))
	rows := []providerListEntry{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip box-drawing and separator lines
		if strings.HasPrefix(line, "┌") || strings.HasPrefix(line, "└") || strings.HasPrefix(line, "│") || strings.HasPrefix(line, "├") || strings.HasPrefix(line, "┤") || strings.HasPrefix(line, "┴") || strings.HasPrefix(line, "┬") || strings.HasPrefix(line, "┼") {
			continue
		}
		// Parse provider bullet lines: ●  Name type
		if strings.HasPrefix(line, "●") {
			rest := strings.TrimSpace(strings.TrimPrefix(line, "●"))
			fields := strings.Fields(rest)
			if len(fields) == 0 {
				continue
			}
			// Last field is the type (api/oauth), everything before is the name
			name := strings.Join(fields[:len(fields)-1], " ")
			if name == "" {
				name = fields[0]
			}
			id := canonicalProviderID(name)
			rows = append(rows, providerListEntry{ID: id, Name: name})
		}
	}
	return normalizeProviderList(rows)
}

func slugify(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "-"))
}

// providerDisplayAliases maps human-readable provider names emitted by
// `opencode providers list` to the canonical provider id used in the opencode
// config (and gateway config.yaml). The TUI prints display names like
// "OpenCode Zen" or "GitHub Copilot" while the underlying config keys are
// "opencode" / "github-copilot". Without this mapping the dashboard cannot
// match list entries with configured providers, so the gateway-enabled badge
// stays off even when forwarding is correctly configured.
var providerDisplayAliases = map[string]string{
	"opencode zen":   "opencode",
	"github copilot": "github-copilot",
	"velcengine":     "volcengine",
}

func canonicalProviderID(name string) string {
	key := strings.ToLower(strings.TrimSpace(name))
	if id, ok := providerDisplayAliases[key]; ok {
		return id
	}
	return slugify(name)
}

func normalizeProviderDisplayName(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "Velcengine") {
		return "Volcengine"
	}
	return name
}

func stripANSICodes(s string) string {
	var buf bytes.Buffer
	for i := 0; i < len(s); i++ {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && !(s[i] >= 'A' && s[i] <= 'Z') && !(s[i] >= 'a' && s[i] <= 'z') {
				i++
			}
			continue
		}
		buf.WriteByte(s[i])
	}
	return buf.String()
}

func normalizeProviderList(rows []providerListEntry) []providerListEntry {
	seen := map[string]providerListEntry{}
	for _, row := range rows {
		row.ID = strings.TrimSpace(row.ID)
		row.Name = strings.TrimSpace(row.Name)
		if row.ID == "" {
			continue
		}
		if row.Name == "" {
			row.Name = row.ID
		}
		seen[row.ID] = row
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]providerListEntry, 0, len(ids))
	for _, id := range ids {
		result = append(result, seen[id])
	}
	return result
}

func writeDashboardJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}
