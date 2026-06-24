package db

import (
	"database/sql"
	"sort"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, err
	}
	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return err
	}

	var currentVersion int
	err = d.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return err
	}

	migrations := []struct {
		version int
		sql     string
	}{
		{1, `CREATE TABLE IF NOT EXISTS providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
		{2, `CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider_id INTEGER NOT NULL REFERENCES providers(id),
			model TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT NOT NULL DEFAULT (datetime('now')),
			ended_at TEXT,
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT
		)`},
		{3, `CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL REFERENCES sessions(id),
			prompt_tokens INTEGER DEFAULT 0,
			completion_tokens INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
		{4, `CREATE TABLE IF NOT EXISTS request_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL REFERENCES sessions(id),
			provider_name TEXT NOT NULL DEFAULT '',
			method TEXT NOT NULL DEFAULT '',
			url TEXT NOT NULL DEFAULT '',
			request_headers TEXT NOT NULL DEFAULT '',
			request_body TEXT NOT NULL DEFAULT '',
			response_status INTEGER NOT NULL DEFAULT 0,
			response_headers TEXT NOT NULL DEFAULT '',
			response_body TEXT NOT NULL DEFAULT '',
			duration_ms INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
		{5, `CREATE INDEX IF NOT EXISTS idx_request_logs_session ON request_logs(session_id)`},
		{6, `CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			display_name TEXT NOT NULL,
			kind TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`},
		{7, `CREATE TABLE IF NOT EXISTS agent_provider_bindings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			agent_id TEXT NOT NULL,
			provider_id INTEGER NOT NULL REFERENCES providers(id),
			enabled INTEGER NOT NULL DEFAULT 1,
			gateway_enabled INTEGER NOT NULL DEFAULT 0,
			config_path TEXT NOT NULL DEFAULT '',
			project_id TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(agent_id, provider_id, config_path)
		)`},
		{8, `ALTER TABLE sessions ADD COLUMN agent_id TEXT NOT NULL DEFAULT 'unknown'`},
		{9, `ALTER TABLE sessions ADD COLUMN project_id TEXT NOT NULL DEFAULT ''`},
		{10, `ALTER TABLE sessions ADD COLUMN endpoint TEXT NOT NULL DEFAULT ''`},
		{11, `ALTER TABLE sessions ADD COLUMN protocol TEXT NOT NULL DEFAULT ''`},
		{12, `ALTER TABLE sessions ADD COLUMN stream INTEGER NOT NULL DEFAULT 0`},
		{13, `ALTER TABLE requests ADD COLUMN response_status INTEGER NOT NULL DEFAULT 0`},
		{14, `ALTER TABLE requests ADD COLUMN endpoint TEXT NOT NULL DEFAULT ''`},
		{15, `CREATE INDEX IF NOT EXISTS idx_sessions_agent_started ON sessions(agent_id, started_at)`},
		{16, `CREATE INDEX IF NOT EXISTS idx_sessions_provider_model_started ON sessions(provider_id, model, started_at)`},
		{17, `CREATE INDEX IF NOT EXISTS idx_sessions_project_started ON sessions(project_id, started_at)`},
		{18, `CREATE INDEX IF NOT EXISTS idx_sessions_endpoint_started ON sessions(endpoint, started_at)`},
		{19, `ALTER TABLE sessions DROP COLUMN protocol`},
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].version < migrations[j].version
	})

	for _, m := range migrations {
		if m.version > currentVersion {
			tx, err := d.conn.Begin()
			if err != nil {
				return err
			}
			if _, err := tx.Exec(m.sql); err != nil {
				tx.Rollback()
				return err
			}
			if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", m.version); err != nil {
				tx.Rollback()
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}
		}
	}

	return nil
}
