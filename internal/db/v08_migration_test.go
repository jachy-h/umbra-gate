package db

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/jachy-h/llm-gateway-lite/internal/models"
	_ "modernc.org/sqlite"
)

func TestMigrateLinkModelPrioritiesRemovesLegacyCredentials(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacySchema := []string{
		`CREATE TABLE proxy_links (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, path TEXT NOT NULL UNIQUE,
			protocol TEXT NOT NULL DEFAULT '', supported_formats_json TEXT NOT NULL DEFAULT '[]',
			attributes_json TEXT NOT NULL DEFAULT '{}', enabled INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE proxy_link_providers (
			link_id TEXT NOT NULL, position INTEGER NOT NULL, provider_id TEXT NOT NULL,
			protocol TEXT NOT NULL DEFAULT '', retry_count INTEGER NOT NULL DEFAULT 0,
			fallback_model TEXT NOT NULL DEFAULT '', api_key TEXT NOT NULL DEFAULT '',
			rules_json TEXT NOT NULL DEFAULT '{}', validation_ok INTEGER,
			validation_error TEXT NOT NULL DEFAULT '', validated_at DATETIME,
			supported_formats_json TEXT NOT NULL DEFAULT '[]',
			PRIMARY KEY (link_id, position),
			FOREIGN KEY (link_id) REFERENCES proxy_links(id) ON DELETE CASCADE
		)`,
	}
	for _, statement := range legacySchema {
		if _, err := legacy.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := legacy.Exec(`INSERT INTO proxy_links(id,name,path) VALUES('link','Legacy Link','legacy')`); err != nil {
		t.Fatal(err)
	}
	if _, err := legacy.Exec(`INSERT INTO proxy_link_providers(link_id,position,provider_id,protocol,retry_count,fallback_model,api_key) VALUES('link',0,'provider','openai',2,' fallback-model ','link-secret')`); err != nil {
		t.Fatal(err)
	}
	if err := legacy.Close(); err != nil {
		t.Fatal(err)
	}

	database, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	var prioritiesJSON string
	if err := database.QueryRow(`SELECT model_priorities_json FROM proxy_link_providers WHERE link_id='link'`).Scan(&prioritiesJSON); err != nil {
		t.Fatal(err)
	}
	var priorities []models.ModelPriority
	if err := json.Unmarshal([]byte(prioritiesJSON), &priorities); err != nil {
		t.Fatal(err)
	}
	want := []models.ModelPriority{
		{Source: models.ModelPriorityRequestModel},
		{Source: models.ModelPriorityFixedModel, Model: "fallback-model"},
	}
	if len(priorities) != len(want) || priorities[0] != want[0] || priorities[1] != want[1] {
		t.Fatalf("priorities = %#v, want %#v", priorities, want)
	}

	columns := map[string]bool{}
	rows, err := database.Query(`PRAGMA table_info(proxy_link_providers)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typ string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatal(err)
		}
		columns[name] = true
	}
	if columns["fallback_model"] || columns["api_key"] || !columns["model_priorities_json"] {
		t.Fatalf("unexpected v0.8 Link-node columns: %#v", columns)
	}
}
