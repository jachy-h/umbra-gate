package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
	path string
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
	db := &DB{DB: d, path: path}
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
			endpoints_json TEXT NOT NULL DEFAULT '[]',
			api_key TEXT NOT NULL DEFAULT '',
			models_json TEXT NOT NULL DEFAULT '[]',
			extra_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			builtin INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE providers ADD COLUMN builtin INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE providers ADD COLUMN endpoints_json TEXT NOT NULL DEFAULT '[]'`,
		`CREATE TABLE IF NOT EXISTS proxy_links (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			protocol TEXT NOT NULL DEFAULT '',
			supported_formats_json TEXT NOT NULL DEFAULT '[]',
			attributes_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE proxy_links ADD COLUMN protocol TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE proxy_links ADD COLUMN supported_formats_json TEXT NOT NULL DEFAULT '[]'`,
		`CREATE TABLE IF NOT EXISTS proxy_link_providers (
			link_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			provider_id TEXT NOT NULL,
			protocol TEXT NOT NULL DEFAULT '',
			retry_count INTEGER NOT NULL DEFAULT 0,
			fallback_model TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			rules_json TEXT NOT NULL DEFAULT '{}',
			validation_ok INTEGER,
			validation_error TEXT NOT NULL DEFAULT '',
			validated_at DATETIME,
			supported_formats_json TEXT NOT NULL DEFAULT '[]',
			PRIMARY KEY (link_id, position),
			FOREIGN KEY (link_id) REFERENCES proxy_links(id) ON DELETE CASCADE
		)`,
		`ALTER TABLE proxy_link_providers ADD COLUMN api_key TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE proxy_link_providers ADD COLUMN protocol TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE proxy_link_providers ADD COLUMN validation_ok INTEGER`,
		`ALTER TABLE proxy_link_providers ADD COLUMN validation_error TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE proxy_link_providers ADD COLUMN validated_at DATETIME`,
		`ALTER TABLE proxy_link_providers ADD COLUMN supported_formats_json TEXT NOT NULL DEFAULT '[]'`,
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
			request_url TEXT NOT NULL DEFAULT '',
			request_headers_json TEXT NOT NULL DEFAULT '{}',
			request_body TEXT NOT NULL DEFAULT '',
			upstream_url TEXT NOT NULL DEFAULT '',
			upstream_headers_json TEXT NOT NULL DEFAULT '{}',
			upstream_body TEXT NOT NULL DEFAULT '',
			response_headers_json TEXT NOT NULL DEFAULT '{}',
			response_body TEXT NOT NULL DEFAULT '',
			attributes_json TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_link_created ON request_logs(link_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_logs_created ON request_logs(created_at)`,
		`ALTER TABLE request_logs ADD COLUMN request_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_logs ADD COLUMN request_headers_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE request_logs ADD COLUMN request_body TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_logs ADD COLUMN upstream_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_logs ADD COLUMN upstream_headers_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE request_logs ADD COLUMN upstream_body TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE request_logs ADD COLUMN response_headers_json TEXT NOT NULL DEFAULT '{}'`,
		`ALTER TABLE request_logs ADD COLUMN response_body TEXT NOT NULL DEFAULT ''`,
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

// DatabaseSize returns the on-disk SQLite database and its WAL size.
func (d *DB) DatabaseSize() (int64, error) {
	var total int64
	for _, path := range []string{d.path, d.path + "-wal"} {
		info, err := os.Stat(path)
		if err == nil {
			total += info.Size()
			continue
		}
		if !os.IsNotExist(err) {
			return 0, err
		}
	}
	return total, nil
}

// PruneOldestRequestLogs removes one bounded batch, then compacts the database
// so the configured filesystem limit is actually recovered.
func (d *DB) PruneOldestRequestLogs(limit int) (int64, error) {
	if limit <= 0 {
		return 0, nil
	}
	result, err := d.Exec(`DELETE FROM request_logs WHERE rowid IN (SELECT rowid FROM request_logs ORDER BY created_at, rowid LIMIT ?)`, limit)
	if err != nil {
		return 0, err
	}
	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	if deleted > 0 {
		if _, err := d.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			return deleted, err
		}
		if _, err := d.Exec(`VACUUM`); err != nil {
			return deleted, err
		}
	}
	return deleted, nil
}

func (d *DB) DeleteRequestLogsBefore(cutoff time.Time) error {
	_, err := d.Exec(`DELETE FROM request_logs WHERE created_at < ?`, cutoff.UTC().Format(time.RFC3339Nano))
	return err
}

func (d *DB) Path() string { return filepath.Clean(d.path) }

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
	const providerSeedVersion = "2026-07-24-opencode-base-prefix"

	type seedProvider struct {
		id      string
		name    string
		typ     string
		baseURL string
		models  []string
	}
	builtins := []seedProvider{
		{id: "deepseek", name: "DeepSeek", typ: "deepseek", baseURL: "https://api.deepseek.com",
			models: []string{"deepseek-chat", "deepseek-reasoner"}},
		{id: "opencode", name: "OpenCode", typ: "opencode", baseURL: "https://opencode.ai/zen/v1",
			models: []string{}},
		{id: "opencode-go", name: "OpenCode Go", typ: "opencode", baseURL: "https://opencode.ai/zen/go/v1",
			models: []string{}},
	}

	var currentSeedVersion string
	if err := d.QueryRow(`SELECT value FROM stats_meta WHERE key='provider_seed_version'`).Scan(&currentSeedVersion); err != nil && err != sql.ErrNoRows {
		return err
	}
	if currentSeedVersion != providerSeedVersion {
		if _, err := d.Exec(`DELETE FROM providers WHERE builtin=1`); err != nil {
			return err
		}
	}
	for _, p := range builtins {
		provider := models.Provider{ID: p.id, Name: p.name, Type: p.typ, BaseURL: p.baseURL}
		endpoints := normalizeProviderEndpoints(provider)
		if p.id == "deepseek" {
			endpoints = append(endpoints, models.ProviderEndpoint{
				Protocol: models.ProtocolAnthropic, RequestFormat: models.FormatMessages,
				ResponseFormat: models.FormatMessages, BaseURL: "https://api.deepseek.com/anthropic",
			})
		}
		if _, err := d.Exec(`INSERT OR IGNORE INTO providers(id,name,type,base_url,endpoints_json,api_key,models_json,extra_json,enabled,builtin,created_at)
			VALUES(?,?,?,?,?,'',?,?,1,1,?)`,
			p.id, p.name, p.typ, p.baseURL, enc(endpoints), enc(p.models), enc(models.Map{}), time.Now().UTC().Format(time.RFC3339)); err != nil {
			return err
		}
		// Update models if provider already existed but had no models
		d.Exec(`UPDATE providers SET models_json=? WHERE id=? AND (models_json='[]' OR models_json='' OR models_json='{}')`, enc(p.models), p.id)
	}
	if err := d.backfillProviderEndpoints(); err != nil {
		return err
	}
	if _, err := d.Exec(`INSERT INTO stats_meta(key,value) VALUES('provider_seed_version',?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, providerSeedVersion); err != nil {
		return err
	}
	_, _ = d.Exec(`UPDATE proxy_link_providers SET protocol=?
		WHERE protocol IN ('', 'openai_chat_completions', 'openai_responses')`, models.ProtocolOpenAI)
	_, _ = d.Exec(`UPDATE proxy_links SET protocol=?
		WHERE protocol IN ('', 'openai_chat_completions', 'openai_responses')`, models.ProtocolOpenAI)
	_, _ = d.Exec(`UPDATE proxy_link_providers
		SET protocol = COALESCE((
			SELECT json_extract(p.endpoints_json, '$[0].protocol') FROM providers p WHERE p.id=proxy_link_providers.provider_id
		), ?)
		WHERE protocol=''`, models.ProtocolOpenAI)
	_, _ = d.Exec(`UPDATE proxy_links
		SET protocol = COALESCE((
			SELECT plp.protocol FROM proxy_link_providers plp WHERE plp.link_id=proxy_links.id ORDER BY plp.position LIMIT 1
		), ?)
		WHERE protocol=''`, models.ProtocolOpenAI)
	return nil
}

func (d *DB) backfillProviderEndpoints() error {
	rows, err := d.Query(`SELECT id,type,base_url,endpoints_json FROM providers`)
	if err != nil {
		return err
	}
	type row struct {
		id, typ, baseURL, endpointsJSON string
	}
	var providersToUpdate []row
	for rows.Next() {
		var item row
		if err := rows.Scan(&item.id, &item.typ, &item.baseURL, &item.endpointsJSON); err != nil {
			rows.Close()
			return err
		}
		providersToUpdate = append(providersToUpdate, item)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for _, item := range providersToUpdate {
		var endpoints []models.ProviderEndpoint
		_ = json.Unmarshal([]byte(item.endpointsJSON), &endpoints)
		provider := models.Provider{ID: item.id, Type: item.typ, BaseURL: item.baseURL, Endpoints: endpoints}
		endpoints = normalizeProviderEndpoints(provider)
		if item.id == "google" {
			for i := range endpoints {
				if strings.Contains(endpoints[i].BaseURL, "generativelanguage.googleapis.com") &&
					!strings.HasSuffix(strings.TrimRight(endpoints[i].BaseURL, "/"), "/openai") {
					endpoints[i].BaseURL = strings.TrimRight(endpoints[i].BaseURL, "/") + "/openai"
				}
			}
		}
		if item.id == "deepseek" {
			found := false
			for _, endpoint := range endpoints {
				if endpoint.Protocol == models.ProtocolAnthropic && endpoint.ResponseFormat == models.FormatMessages {
					found = true
					break
				}
			}
			if !found {
				endpoints = append(endpoints, models.ProviderEndpoint{
					Protocol: models.ProtocolAnthropic, RequestFormat: models.FormatMessages,
					ResponseFormat: models.FormatMessages, BaseURL: "https://api.deepseek.com/anthropic",
				})
			}
		}
		if _, err := d.Exec(`UPDATE providers SET endpoints_json=? WHERE id=?`, enc(endpoints), item.id); err != nil {
			return err
		}
	}
	return nil
}
