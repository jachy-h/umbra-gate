package opencodeconfig

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const SchemaURL = "https://opencode.ai/config.json"

const (
	GatewayEnable  = "enable"
	GatewayDisable = "disable"
)

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
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	API            string   `json:"api"`
	APIKey         string   `json:"api_key"`
	BaseURL        string   `json:"base_url"`
	Gateway        string   `json:"gateway"`
	GatewayBaseURL string   `json:"gateway_base_url"`
	Models         []string `json:"models"`
	DefaultModel   string   `json:"default_model"`
	SmallModel     string   `json:"small_model"`
	Delete         bool     `json:"delete"`
}

type Plan struct {
	BaseChecksum string         `json:"base_checksum"`
	Diff         string         `json:"diff"`
	Current      string         `json:"current"`
	ProposedText string         `json:"proposed_text"`
	Proposed     map[string]any `json:"proposed"`
}

var ErrStaleConfig = errors.New("opencode config changed since diff was generated")

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "opencode")
	}
	return filepath.Join(home, ".config", "opencode")
}

func DefaultPath() string {
	return filepath.Join(DefaultDir(), "opencode.json")
}

func Discover(baseDir string) []ConfigFile {
	if baseDir == "" {
		baseDir = DefaultDir()
	}
	candidates := []string{
		"opencode.json",
		"opencode.jsonc",
		filepath.Join(".opencode", "opencode.json"),
		filepath.Join(".opencode", "opencode.jsonc"),
	}
	files := []ConfigFile{}
	for _, candidate := range candidates {
		path := filepath.Join(baseDir, candidate)
		if _, err := os.Stat(path); err == nil {
			files = append(files, ConfigFile{Path: path, Label: candidate, Exists: true, Selected: len(files) == 0})
		}
	}
	if len(files) == 0 {
		files = append(files, ConfigFile{Path: filepath.Join(baseDir, "opencode.json"), Label: "opencode.json", Exists: false, Selected: true})
	}
	return files
}

func (m Manager) Load() (map[string]any, []byte, error) {
	path := m.path()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{"$schema": SchemaURL}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var cfg map[string]any
	if err := json.Unmarshal(stripJSONC(raw), &cfg); err != nil {
		return nil, nil, err
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	return cfg, raw, nil
}

func (m Manager) Plan(input ProviderInput) (*Plan, error) {
	cfg, raw, err := m.Load()
	if err != nil {
		return nil, err
	}
	baseChecksum := checksum(raw)
	proposed := cloneMap(cfg)
	applyInput(proposed, input)
	currentJSON, err := marshalMasked(cfg)
	if err != nil {
		return nil, err
	}
	proposedJSON, err := marshalMasked(proposed)
	if err != nil {
		return nil, err
	}
	return &Plan{BaseChecksum: baseChecksum, Diff: unifiedDiff(currentJSON, proposedJSON), Current: string(currentJSON), ProposedText: string(proposedJSON), Proposed: proposed}, nil
}

func (m Manager) Apply(input ProviderInput, baseChecksum string) error {
	_, raw, err := m.Load()
	if err != nil {
		return err
	}
	if checksum(raw) != baseChecksum {
		return ErrStaleConfig
	}
	plan, err := m.Plan(input)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(plan.Proposed, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	path := m.path()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		backup := fmt.Sprintf("%s.%s.bak", path, time.Now().Format("20060102-150405"))
		original, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(backup, original, 0o600); err != nil {
			return err
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencode-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func (m Manager) MaskedConfig() (map[string]any, error) {
	cfg, _, err := m.Load()
	if err != nil {
		return nil, err
	}
	return maskSecrets(cloneMap(cfg)).(map[string]any), nil
}

func (m Manager) path() string {
	if m.Path != "" {
		return m.Path
	}
	return DefaultPath()
}

func applyInput(cfg map[string]any, input ProviderInput) {
	if cfg["$schema"] == nil {
		cfg["$schema"] = SchemaURL
	}
	providers := objectAt(cfg, "provider")
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return
	}
	if input.Delete {
		delete(providers, id)
		return
	}
	provider := objectAt(providers, id)
	if input.Name != "" {
		provider["name"] = input.Name
	}
	if input.API != "" {
		provider["api"] = input.API
	}
	options := objectAt(provider, "options")
	if input.APIKey != "" {
		options["apiKey"] = input.APIKey
	}
	if input.BaseURL != "" {
		options["baseURL"] = input.BaseURL
	}
	gatewayBase := strings.TrimRight(input.GatewayBaseURL, "/")
	if input.Gateway == GatewayEnable && gatewayBase != "" {
		options["baseURL"] = gatewayBase + "/" + url.PathEscape(id)
	}
	if input.Gateway == GatewayDisable && gatewayBase != "" {
		esc := url.PathEscape(id)
		if current, ok := options["baseURL"].(string); ok && (current == gatewayBase+"/"+id || current == gatewayBase+"/"+esc) {
			delete(options, "baseURL")
		}
	}
}

func stripJSONC(raw []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == '/' && i+1 < len(raw) && raw[i+1] == '/' {
			i += 2
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			if i < len(raw) {
				out.WriteByte(raw[i])
			}
			continue
		}
		if ch == '/' && i+1 < len(raw) && raw[i+1] == '*' {
			i += 2
			for i+1 < len(raw) && !(raw[i] == '*' && raw[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		out.WriteByte(ch)
	}
	withoutComments := out.Bytes()
	out.Reset()
	inString = false
	escaped = false
	for i := 0; i < len(withoutComments); i++ {
		ch := withoutComments[i]
		if inString {
			out.WriteByte(ch)
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			out.WriteByte(ch)
			continue
		}
		if ch == ',' {
			j := i + 1
			for j < len(withoutComments) && (withoutComments[j] == ' ' || withoutComments[j] == '\n' || withoutComments[j] == '\r' || withoutComments[j] == '\t') {
				j++
			}
			if j < len(withoutComments) && (withoutComments[j] == '}' || withoutComments[j] == ']') {
				continue
			}
		}
		out.WriteByte(ch)
	}
	return out.Bytes()
}

func objectAt(parent map[string]any, key string) map[string]any {
	if existing, ok := parent[key].(map[string]any); ok {
		return existing
	}
	created := map[string]any{}
	parent[key] = created
	return created
}

func cloneMap(src map[string]any) map[string]any {
	data, _ := json.Marshal(src)
	var dst map[string]any
	_ = json.Unmarshal(data, &dst)
	if dst == nil {
		dst = map[string]any{}
	}
	return dst
}

func checksum(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func marshalMasked(v any) ([]byte, error) {
	masked := maskSecrets(v)
	return json.MarshalIndent(masked, "", "  ")
}

func maskSecrets(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		masked := map[string]any{}
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.EqualFold(key, "apiKey") {
				masked[key] = "********"
				continue
			}
			masked[key] = maskSecrets(typed[key])
		}
		return masked
	case []any:
		masked := make([]any, len(typed))
		for i, item := range typed {
			masked[i] = maskSecrets(item)
		}
		return masked
	default:
		return typed
	}
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
