package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1) // avoid sqlite write contention
	if err := d.Ping(); err != nil {
		return nil, err
	}
	db := &DB{d}
	if err := db.migrate(); err != nil {
		return nil, err
	}
	if err := db.seed(); err != nil {
		return nil, err
	}
	return db, nil
}

func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS providers (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL,
			base_url TEXT NOT NULL,
			api_key TEXT NOT NULL DEFAULT '',
			models_json TEXT NOT NULL DEFAULT '[]',
			extra_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			builtin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE providers ADD COLUMN builtin INTEGER NOT NULL DEFAULT 0`,
		`CREATE TABLE IF NOT EXISTS proxy_links (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			attributes_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS proxy_link_providers (
			link_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			provider_id TEXT NOT NULL,
			retry_count INTEGER NOT NULL DEFAULT 0,
			fallback_model TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			rules_json TEXT NOT NULL DEFAULT '{}',
			PRIMARY KEY (link_id, position),
			FOREIGN KEY (link_id) REFERENCES proxy_links(id) ON DELETE CASCADE
		)`,
		`ALTER TABLE proxy_link_providers ADD COLUMN api_key TEXT NOT NULL DEFAULT ''`,
		`CREATE TABLE IF NOT EXISTS request_logs (
			id TEXT PRIMARY KEY,
			link_id TEXT NOT NULL,
			path TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			provider_name TEXT NOT NULL,
			model TEXT NOT NULL DEFAULT '',
			status_code INTEGER NOT NULL DEFAULT 0,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 0,
			error_message TEXT NOT NULL DEFAULT '',
			attributes_json TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_link_created ON request_logs(link_id, created_at)`,
		`CREATE TABLE IF NOT EXISTS stats_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS stats_hourly (
			link_id TEXT NOT NULL,
			provider_id TEXT NOT NULL,
			attr_key TEXT NOT NULL DEFAULT '',
			attr_value TEXT NOT NULL DEFAULT '',
			period TEXT NOT NULL,
			total INTEGER NOT NULL DEFAULT 0,
			success INTEGER NOT NULL DEFAULT 0,
			failure INTEGER NOT NULL DEFAULT 0,
			total_latency_ms INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (link_id, provider_id, attr_key, attr_value, period)
		)`,
	}
	for _, s := range stmts {
		_, err := d.Exec(s)
		if err != nil && strings.HasPrefix(s, "ALTER TABLE") {
			continue // column already exists
		}
		if err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func enc(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
func decModels(s string) []string {
	var v []string
	_ = json.Unmarshal([]byte(s), &v)
	return v
}
func decMap(s string) models.Map {
	var v models.Map
	if s == "" {
		s = "{}"
	}
	_ = json.Unmarshal([]byte(s), &v)
	return v
}

func (d *DB) seed() error {
	type seedProvider struct {
		id      string
		name    string
		typ     string
		baseURL string
		models  []string
	}
	builtins := []seedProvider{
		{id: "openai", name: "OpenAI", typ: "openai", baseURL: "https://api.openai.com/v1",
			models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "o4-mini", "o3-mini"}},
		{id: "anthropic", name: "Anthropic", typ: "anthropic", baseURL: "https://api.anthropic.com",
			models: []string{"claude-opus-4-20250514", "claude-sonnet-4-20250514", "claude-3.5-haiku"}},
		{id: "google", name: "Gemini", typ: "gemini", baseURL: "https://generativelanguage.googleapis.com/v1beta",
			models: []string{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"}},
		{id: "deepseek", name: "DeepSeek", typ: "deepseek", baseURL: "https://api.deepseek.com/v1",
			models: []string{"deepseek-chat", "deepseek-reasoner"}},
		{id: "qwen", name: "Qwen", typ: "qwen", baseURL: "https://dashscope-intl.aliyuncs.com/compatible-mode/v1",
			models: []string{"qwen-plus", "qwen-max", "qwen-turbo"}},
		{id: "xai", name: "xAI", typ: "custom", baseURL: "https://api.x.ai/v1",
			models: []string{"grok-4", "grok-3", "grok-3-mini"}},
		{id: "groq", name: "Groq", typ: "custom", baseURL: "https://api.groq.com/openai/v1",
			models: []string{"llama-4-scout-17b-16e-instruct"}},
		{id: "cerebras", name: "Cerebras", typ: "custom", baseURL: "https://api.cerebras.ai/v1",
			models: []string{"llama-4-scout-17b-16e-instruct", "llama-3.3-70b"}},
		{id: "deepinfra", name: "DeepInfra", typ: "custom", baseURL: "https://api.deepinfra.com/v1/openai",
			models: []string{"meta-llama/Meta-Llama-3.1-405B-Instruct"}},
		{id: "together", name: "Together AI", typ: "custom", baseURL: "https://api.together.xyz/v1",
			models: []string{"meta-llama/Llama-4-Maverick-17B-128E-Instruct-FP8"}},
		{id: "fireworks", name: "Fireworks AI", typ: "custom", baseURL: "https://api.fireworks.ai/inference/v1",
			models: []string{"accounts/fireworks/models/llama4-maverick-instruct-basic"}},
		{id: "openrouter", name: "OpenRouter", typ: "custom", baseURL: "https://openrouter.ai/api/v1",
			models: []string{"openai/gpt-4o", "anthropic/claude-sonnet-4"}},
		{id: "github-copilot", name: "GitHub Copilot", typ: "custom", baseURL: "https://api.githubcopilot.com",
			models: []string{"gpt-4o", "claude-sonnet-4"}},
		{id: "moonshot", name: "Moonshot", typ: "custom", baseURL: "https://api.moonshot.cn/v1",
			models: []string{"moonshot-v1-8k", "moonshot-v1-32k", "moonshot-v1-128k"}},
		{id: "minimax", name: "MiniMax", typ: "custom", baseURL: "https://api.minimax.chat/v1",
			models: []string{"abab6.5s-chat", "abab7-chat"}},
		{id: "vercel", name: "Vercel AI", typ: "custom", baseURL: "https://api.ai.vercel.com/v1",
			models: []string{"gpt-4o", "claude-sonnet-4"}},
		{id: "volcengine", name: "Volcengine", typ: "custom", baseURL: "https://ark.cn-beijing.volces.com/api/coding/v3",
			models: []string{"deepseek-r1-250528", "deepseek-v3-250324"}},
		{id: "helicone", name: "Helicone", typ: "custom", baseURL: "https://ai-gateway.helicone.ai",
			models: []string{}},
		{id: "huggingface", name: "HuggingFace", typ: "custom", baseURL: "https://api-inference.huggingface.co/v1",
			models: []string{}},
		{id: "cloudflare-ai-gateway", name: "Cloudflare AI", typ: "custom", baseURL: "https://gateway.ai.cloudflare.com/v1/{ACCOUNT_ID}/{GATEWAY_ID}",
			models: []string{}},
		{id: "opencode", name: "OpenCode", typ: "opencode", baseURL: "https://opencode.ai/zen/v1/responses",
			models: []string{}},
		{id: "opencode-go", name: "OpenCode Go", typ: "opencode", baseURL: "https://opencode.ai/zen/go/v1/responses",
			models: []string{}},
	}
	for _, p := range builtins {
		if _, err := d.Exec(`INSERT OR IGNORE INTO providers(id,name,type,base_url,api_key,models_json,extra_json,enabled,builtin,created_at)
			VALUES(?,?,?,?,'',?,?,1,1,?)`,
			p.id, p.name, p.typ, p.baseURL, enc(p.models), enc(models.Map{}), time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
		// Update models if provider already existed but had no models
		d.Exec(`UPDATE providers SET models_json=? WHERE id=? AND (models_json='[]' OR models_json='' OR models_json='{}')`, enc(p.models), p.id)
	}
	return nil
}
