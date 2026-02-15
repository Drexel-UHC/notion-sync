// Package store provides a SQLite-backed store for synced Notion pages.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const dbFileName = "_notion_sync.db"

// Store wraps a SQLite database for storing synced page data.
type Store struct {
	db   *sql.DB
	path string
}

// PageData holds the data for a single page to be stored.
type PageData struct {
	ID             string
	Title          string
	URL            string
	FilePath       string
	BodyMarkdown   string
	PropertiesJSON string
	CreatedTime    string
	LastEditedTime string
	FrozenAt       string
	DatabaseID     string
}

// OpenStore opens or creates a SQLite database at workspacePath/_notion_sync.db.
func OpenStore(workspacePath string) (*Store, error) {
	dbPath := filepath.Join(workspacePath, dbFileName)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Force single connection — PRAGMAs like recursive_triggers are per-connection,
	// and the connection pool would create new connections without them.
	// Fine for a CLI tool; we don't need concurrent DB writers.
	db.SetMaxOpenConns(1)

	// Enable WAL mode for concurrent reads (persists in DB file)
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Required for FTS5 correctness with INSERT OR REPLACE —
	// without this, the implicit DELETE in REPLACE doesn't fire AFTER DELETE triggers.
	if _, err := db.Exec("PRAGMA recursive_triggers = 1"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set recursive_triggers: %w", err)
	}

	// Wait up to 5s if another process holds a lock (e.g. sqlite3 CLI during debugging)
	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}

	s := &Store{db: db, path: dbPath}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}

	return s, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// UpsertPage inserts or replaces a page in the store.
func (s *Store) UpsertPage(data PageData) error {
	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO pages (id, title, url, file_path, body_markdown, properties_json, created_time, last_edited_time, frozen_at, deleted, database_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		data.ID, data.Title, data.URL, data.FilePath, data.BodyMarkdown,
		data.PropertiesJSON, data.CreatedTime, data.LastEditedTime,
		data.FrozenAt, data.DatabaseID,
	)
	if err != nil {
		return fmt.Errorf("upsert page %s: %w", data.ID, err)
	}
	return nil
}

// MarkDeleted sets the deleted flag on a page.
func (s *Store) MarkDeleted(pageID string) error {
	_, err := s.db.Exec("UPDATE pages SET deleted = 1 WHERE id = ?", pageID)
	if err != nil {
		return fmt.Errorf("mark deleted %s: %w", pageID, err)
	}
	return nil
}

// PageSyncInfo holds minimal info for incremental sync checks.
type PageSyncInfo struct {
	ID             string
	LastEditedTime string
}

// GetPagesByDatabase returns sync info for all non-deleted pages in a database.
func (s *Store) GetPagesByDatabase(databaseID string) ([]PageSyncInfo, error) {
	rows, err := s.db.Query(
		"SELECT id, last_edited_time FROM pages WHERE database_id = ? AND deleted = 0",
		databaseID,
	)
	if err != nil {
		return nil, fmt.Errorf("query pages by database: %w", err)
	}
	defer rows.Close()

	var result []PageSyncInfo
	for rows.Next() {
		var p PageSyncInfo
		if err := rows.Scan(&p.ID, &p.LastEditedTime); err != nil {
			return nil, fmt.Errorf("scan page: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// GetPageLastEdited returns the last_edited_time for a specific page, or empty string if not found.
func (s *Store) GetPageLastEdited(pageID string) string {
	var lastEdited string
	err := s.db.QueryRow("SELECT last_edited_time FROM pages WHERE id = ? AND deleted = 0", pageID).Scan(&lastEdited)
	if err != nil {
		return ""
	}
	return lastEdited
}

// SerializeProperties converts a frontmatter map to JSON for storage.
func SerializeProperties(fm map[string]interface{}) (string, error) {
	b, err := json.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("serialize properties: %w", err)
	}
	return string(b), nil
}

func (s *Store) initSchema() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema tx: %w", err)
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS _meta (
			key TEXT PRIMARY KEY,
			value TEXT
		)`,
		`INSERT OR IGNORE INTO _meta (key, value) VALUES ('schema_version', '1')`,
		`CREATE TABLE IF NOT EXISTS pages (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			file_path TEXT,
			body_markdown TEXT NOT NULL DEFAULT '',
			properties_json TEXT NOT NULL DEFAULT '{}',
			created_time TEXT,
			last_edited_time TEXT NOT NULL,
			frozen_at TEXT NOT NULL,
			deleted INTEGER DEFAULT 0,
			database_id TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pages_database_id ON pages(database_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pages_last_edited ON pages(last_edited_time)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("schema stmt: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema: %w", err)
	}

	// FTS5 virtual tables and triggers cannot be created inside a transaction
	// in some SQLite builds, so run them outside the transaction.
	fts := []string{
		`CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
			title, body_markdown, content=pages, content_rowid=rowid
		)`,
		// AFTER INSERT — index new pages
		`CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
			INSERT INTO pages_fts(rowid, title, body_markdown)
			VALUES (new.rowid, new.title, new.body_markdown);
		END`,
		// AFTER DELETE — remove from FTS index
		`CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
			INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
			VALUES('delete', old.rowid, old.title, old.body_markdown);
		END`,
		// AFTER UPDATE — re-index (covers MarkDeleted and any future direct UPDATEs)
		`CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
			INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
			VALUES('delete', old.rowid, old.title, old.body_markdown);
			INSERT INTO pages_fts(rowid, title, body_markdown)
			VALUES (new.rowid, new.title, new.body_markdown);
		END`,
	}

	for _, stmt := range fts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("fts stmt: %w", err)
		}
	}

	return nil
}
