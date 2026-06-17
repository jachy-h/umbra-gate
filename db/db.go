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
