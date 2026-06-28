package codexconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultProviderID   = "openai"
	DefaultProviderName = "Umbragate OpenAI"
	DefaultEnvKey       = "OPENAI_API_KEY"
	DefaultWireAPI      = "responses"
	// CustomProviderID is used as the gateway model_provider when the active
	// provider is a reserved built-in (like "openai"). Codex rejects overriding
	// built-in provider ids, so gateway takeover mirrors the cc-switch pattern
	// and switches routing to a non-reserved "custom" entry.
	CustomProviderID = "custom"
)

// reservedProviderIDs are built-in Codex provider ids that cannot be overridden
// in model_providers.
var reservedProviderIDs = map[string]struct{}{
	"openai": {},
}

const (
	GatewayEnable  = "enable"
	GatewayDisable = "disable"
)

var ErrStaleConfig = errors.New("codex config changed since diff was generated")

type Manager struct {
	Path string
}

type ConfigFile struct {
	Path     string `json:"path"`
	Label    string `json:"label"`
	Exists   bool   `json:"exists"`
	Selected bool   `json:"selected"`
}

type ProviderInput struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	EnvKey         string `json:"env_key"`
	WireAPI        string `json:"wire_api"`
	Gateway        string `json:"gateway"`
	GatewayBaseURL string `json:"gateway_base_url"`
	Delete         bool   `json:"delete"`
}

type Plan struct {
	BaseChecksum string `json:"base_checksum"`
	Diff         string `json:"diff"`
	Current      string `json:"current"`
	ProposedText string `json:"proposed_text"`
}

type ProviderStatus struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	EnvKey         string `json:"env_key"`
	WireAPI        string `json:"wire_api"`
	Configured     bool   `json:"configured"`
	Active         bool   `json:"active"`
	GatewayEnabled bool   `json:"gateway_enabled"`
}

func DefaultHome() string {
	if home := strings.TrimSpace(os.Getenv("CODEX_HOME")); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".codex"
	}
	return filepath.Join(home, ".codex")
}

func DefaultPath() string {
	return filepath.Join(DefaultHome(), "config.toml")
}

func Discover() []ConfigFile {
	path := DefaultPath()
	_, err := os.Stat(path)
	return []ConfigFile{{Path: path, Label: "config.toml", Exists: err == nil, Selected: true}}
}

func (m Manager) Load() ([]byte, error) {
	path := m.path()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func (m Manager) Plan(input ProviderInput) (*Plan, error) {
	raw, err := m.Load()
	if err != nil {
		return nil, err
	}
	proposed, err := ApplyToText(raw, input)
	if err != nil {
		return nil, err
	}
	current := normalizeTrailingNewline(raw)
	return &Plan{
		BaseChecksum: checksum(raw),
		Diff:         unifiedDiff(current, proposed),
		Current:      string(current),
		ProposedText: string(proposed),
	}, nil
}

func (m Manager) Apply(input ProviderInput, baseChecksum string) error {
	raw, err := m.Load()
	if err != nil {
		return err
	}
	if checksum(raw) != baseChecksum {
		return ErrStaleConfig
	}
	proposed, err := ApplyToText(raw, input)
	if err != nil {
		return err
	}
	path := m.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".codex-config-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(proposed); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (m Manager) Statuses(ids []string, gatewayBaseURL string) ([]ProviderStatus, error) {
	raw, err := m.Load()
	if err != nil {
		return nil, err
	}
	cfg := parse(raw)
	if len(ids) == 0 {
		ids = []string{DefaultProviderID}
	}
	seen := map[string]struct{}{}
	out := make([]ProviderStatus, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		table := cfg.ProviderTable(id)
		status := ProviderStatus{
			ID:      id,
			Name:    firstNonEmpty(table["name"], "Umbragate "+id),
			BaseURL: table["base_url"],
			EnvKey:  firstNonEmpty(table["env_key"], DefaultEnvKey),
			WireAPI: firstNonEmpty(table["wire_api"], DefaultWireAPI),
		}
		status.Configured = len(table) > 0
		status.Active = cfg.Top["model_provider"] == id
		status.GatewayEnabled = gatewayURLMatches(status.BaseURL, gatewayBaseURL, id)
		if !status.GatewayEnabled {
			// When gateway takeover routes through the managed "custom" provider,
			// the requested id (e.g. openai) may not have its own table. Detect
			// gateway state by checking the active custom entry instead.
			customTable := cfg.ProviderTable(CustomProviderID)
			if cfg.Top["model_provider"] == CustomProviderID && len(customTable) > 0 && gatewayURLMatches(customTable["base_url"], gatewayBaseURL, id) {
				status.GatewayEnabled = true
				if status.BaseURL == "" {
					status.BaseURL = customTable["base_url"]
				}
			}
		}
		out = append(out, status)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func ApplyToText(raw []byte, input ProviderInput) ([]byte, error) {
	input = input.withDefaults()
	if err := validateID(input.ID); err != nil {
		return nil, err
	}
	text := string(normalizeTrailingNewline(raw))
	if input.Gateway == GatewayEnable {
		text = enableCodexGateway(text, input)
		return []byte(strings.TrimRight(text, "\n") + "\n"), nil
	}
	if input.Gateway == GatewayDisable {
		text = disableCodexGateway(text, input)
		return []byte(strings.TrimRight(text, "\n") + "\n"), nil
	}
	text = removeProviderTable(text, input.ID)
	if input.Delete {
		cfg := parse([]byte(text))
		if cfg.Top["model_provider"] == input.ID {
			text = upsertTopLevelString(text, "model_provider", "")
		}
		return []byte(strings.TrimRight(text, "\n") + "\n"), nil
	}
	text = upsertTopLevelString(text, "model_provider", input.ID)
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	text += fmt.Sprintf("[model_providers.%s]\n", input.ID)
	text += fmt.Sprintf("name = %q\n", input.Name)
	text += fmt.Sprintf("base_url = %q\n", input.BaseURL)
	text += fmt.Sprintf("env_key = %q\n", input.EnvKey)
	text += fmt.Sprintf("wire_api = %q\n", input.WireAPI)
	return []byte(text), nil
}

func GatewayURL(baseURL, id string) string {
	return gatewayURL(baseURL, id)
}

func (m Manager) path() string {
	if m.Path != "" {
		return m.Path
	}
	return DefaultPath()
}

func (in ProviderInput) withDefaults() ProviderInput {
	in.ID = strings.TrimSpace(in.ID)
	if in.ID == "" {
		in.ID = DefaultProviderID
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		if in.ID == DefaultProviderID {
			in.Name = DefaultProviderName
		} else {
			in.Name = "Umbragate " + in.ID
		}
	}
	in.EnvKey = strings.TrimSpace(in.EnvKey)
	if in.EnvKey == "" {
		in.EnvKey = DefaultEnvKey
	}
	in.WireAPI = strings.TrimSpace(in.WireAPI)
	if in.WireAPI == "" {
		in.WireAPI = DefaultWireAPI
	}
	in.BaseURL = strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	if in.BaseURL == "" {
		in.BaseURL = gatewayURL(in.GatewayBaseURL, in.ID)
	}
	return in
}

var providerIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func validateID(id string) error {
	if id == "" {
		return errors.New("provider id is required")
	}
	if !providerIDPattern.MatchString(id) {
		return fmt.Errorf("provider id %q must use only letters, numbers, underscore, or hyphen", id)
	}
	return nil
}

func gatewayURL(baseURL, id string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://127.0.0.1:4141"
	}
	return base + "/a/codex/" + url.PathEscape(id) + "/v1"
}

func gatewayURLMatches(baseURL, gatewayBaseURL, id string) bool {
	current := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	base := strings.TrimRight(strings.TrimSpace(gatewayBaseURL), "/")
	return current == gatewayURL(gatewayBaseURL, id) ||
		current == base+"/v1" ||
		current == base+"/a/codex/"+id+"/v1" ||
		current == base+"/"+id+"/v1"
}

type parsedConfig struct {
	Top    map[string]string
	Tables map[string]map[string]string
}

func (c parsedConfig) ProviderTable(id string) map[string]string {
	if c.Tables == nil {
		return nil
	}
	return c.Tables["model_providers."+id]
}

func parse(raw []byte) parsedConfig {
	cfg := parsedConfig{Top: map[string]string{}, Tables: map[string]map[string]string{}}
	current := ""
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			current = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			if _, ok := cfg.Tables[current]; !ok {
				cfg.Tables[current] = map[string]string{}
			}
			continue
		}
		key, value, ok := parseStringAssignment(trimmed)
		if !ok {
			continue
		}
		if current == "" {
			cfg.Top[key] = value
		} else {
			cfg.Tables[current][key] = value
		}
	}
	return cfg
}

func parseStringAssignment(line string) (string, string, bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || len(value) < 2 || value[0] != '"' {
		return "", "", false
	}
	var b strings.Builder
	escaped := false
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if escaped {
			b.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' {
			escaped = true
			continue
		}
		if ch == '"' {
			return key, b.String(), true
		}
		b.WriteByte(ch)
	}
	return "", "", false
}

func removeProviderTable(text, id string) string {
	target := "model_providers." + id
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	skip := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			table := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			skip = table == target
			if skip {
				continue
			}
		}
		if !skip {
			out = append(out, line)
		}
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

// enableCodexGateway routes Codex traffic through the gateway by rewriting the
// active provider's base_url. When the active provider is a reserved built-in
// (like "openai") or unset, it switches routing to a non-reserved "custom"
// entry — Codex rejects overriding built-in ids in model_providers. For an
// already-active non-reserved provider, only base_url is rewritten and all
// other keys (name, env_key, wire_api, ...) are preserved.
func enableCodexGateway(text string, input ProviderInput) string {
	cfg := parse([]byte(text))
	active := cfg.Top["model_provider"]
	_, reserved := reservedProviderIDs[active]
	if active != "" && !reserved {
		return rewriteProviderBaseURL(text, active, input.BaseURL)
	}
	// Active is empty or reserved: route through custom provider table.
	text = upsertTopLevelString(text, "model_provider", CustomProviderID)
	return upsertCustomProviderTable(text, input.BaseURL, input.WireAPI)
}

// disableCodexGateway reverses the gateway takeover. It strips a
// gateway-matching base_url from the active provider table (or top-level),
// preserving all other keys. Only the managed "custom" table created solely for
// gateway routing is removed wholesale.
func disableCodexGateway(text string, input ProviderInput) string {
	cfg := parse([]byte(text))
	active := cfg.Top["model_provider"]
	if active == CustomProviderID {
		table := cfg.ProviderTable(CustomProviderID)
		baseURL := table["base_url"]
		if baseURL != "" && gatewayURLMatches(baseURL, input.GatewayBaseURL, input.ID) {
			if isManagedCustomProvider(table) {
				text = removeProviderTable(text, CustomProviderID)
				text = upsertTopLevelString(text, "model_provider", "")
				return text
			}
			return stripGatewayBaseURL(text, active, input.GatewayBaseURL, input.ID)
		}
	}
	return stripGatewayBaseURL(text, active, input.GatewayBaseURL, input.ID)
}

// isManagedCustomProvider returns true when the custom table was created solely
// by gateway enable. Tables that carry user credentials or a non-default auth
// shape are left in place and only their base_url is stripped on disable.
func isManagedCustomProvider(table map[string]string) bool {
	if len(table) == 0 {
		return false
	}
	if name, ok := table["name"]; !ok || name != "Umbragate" {
		return false
	}
	if envKey := strings.TrimSpace(table["env_key"]); envKey != "" && envKey != DefaultEnvKey {
		return false
	}
	for _, reserved := range []string{"experimental_bearer_token", "api_key"} {
		if v, ok := table[reserved]; ok && v != "" {
			return false
		}
	}
	return true
}

// rewriteProviderBaseURL replaces base_url inside [model_providers.<id>],
// preserving all other keys.
func rewriteProviderBaseURL(text, providerID, baseURL string) string {
	tableID := "model_providers." + providerID
	lines := strings.Split(text, "\n")
	var out []string
	insideTarget := false
	written := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			table := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			insideTarget = table == tableID
		}
		if insideTarget && isAssignmentFor(trimmed, "base_url") {
			if !written {
				out = append(out, fmt.Sprintf("base_url = %q", baseURL))
				written = true
			}
			continue
		}
		out = append(out, line)
	}
	if !written {
		out = ensureProviderTable(out, tableID, baseURL)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

// ensureProviderTable appends a [model_providers.<id>] table with base_url when
// the active provider table was missing entirely.
func ensureProviderTable(lines []string, tableID, baseURL string) []string {
	hasTable := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "["+tableID+"]" {
			hasTable = true
			break
		}
	}
	if hasTable {
		return lines
	}
	out := lines
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, "["+tableID+"]")
	out = append(out, fmt.Sprintf("base_url = %q", baseURL))
	return out
}

// upsertCustomProviderTable creates or replaces [model_providers.custom] with
// the gateway base_url and wire_api, keeping unrelated top-level keys intact.
// Codex auth remains enabled so Codex can use its own auth.json login state and
// pass Authorization through the local gateway.
func upsertCustomProviderTable(text, baseURL, wireAPI string) string {
	text = removeProviderTable(text, CustomProviderID)
	text = strings.TrimRight(text, "\n")
	if text != "" {
		text += "\n\n"
	}
	text += fmt.Sprintf("[model_providers.%s]\n", CustomProviderID)
	text += fmt.Sprintf("name = \"Umbragate\"\n")
	text += fmt.Sprintf("base_url = %q\n", baseURL)
	text += fmt.Sprintf("wire_api = %q\n", wireAPI)
	text += "requires_openai_auth = true\n"
	return text
}

// stripGatewayBaseURL removes a gateway-matching base_url from the active
// provider table (or top-level), leaving unrelated config intact.
func stripGatewayBaseURL(text, active, gatewayBaseURL, id string) string {
	lines := strings.Split(text, "\n")
	var out []string
	insideTarget := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			table := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "["), "]"))
			insideTarget = active != "" && table == "model_providers."+active
		}
		if (insideTarget || active == "") && isAssignmentFor(trimmed, "base_url") {
			_, value, ok := parseStringAssignment(trimmed)
			if ok && gatewayURLMatches(value, gatewayBaseURL, id) {
				continue
			}
		}
		out = append(out, line)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func upsertTopLevelString(text, key, value string) string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)+1)
	inserted := false
	replaced := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inserted && strings.HasPrefix(trimmed, "[") {
			if value != "" && !replaced {
				out = append(out, fmt.Sprintf("%s = %q", key, value))
				if len(out) > 0 && strings.TrimSpace(line) != "" {
					out = append(out, "")
				}
			}
			inserted = true
		}
		if isAssignmentFor(trimmed, key) && !inserted {
			replaced = true
			if value != "" {
				out = append(out, fmt.Sprintf("%s = %q", key, value))
			}
			continue
		}
		out = append(out, line)
	}
	if !inserted && !replaced && value != "" {
		prefix := []string{fmt.Sprintf("%s = %q", key, value)}
		if strings.TrimSpace(text) != "" {
			prefix = append(prefix, "")
		}
		out = append(prefix, out...)
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
}

func isAssignmentFor(line, key string) bool {
	if !strings.HasPrefix(line, key) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(line, key))
	return strings.HasPrefix(rest, "=")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeTrailingNewline(raw []byte) []byte {
	out := bytes.TrimRight(raw, "\n")
	if len(out) == 0 {
		return nil
	}
	return append(out, '\n')
}

func checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func unifiedDiff(current, proposed []byte) string {
	var out bytes.Buffer
	out.WriteString("--- current\n")
	out.WriteString("+++ proposed\n")
	if reflect.DeepEqual(current, proposed) {
		return out.String()
	}
	currentLines := strings.Split(string(current), "\n")
	proposedLines := strings.Split(string(proposed), "\n")
	for _, line := range currentLines {
		if line != "" {
			out.WriteString("-")
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	for _, line := range proposedLines {
		if line != "" {
			out.WriteString("+")
			out.WriteString(line)
			out.WriteByte('\n')
		}
	}
	return out.String()
}
