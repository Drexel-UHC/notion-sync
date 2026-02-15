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

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Required for FTS5 correctness with INSERT OR REPLACE
	if _, err := db.Exec("PRAGMA recursive_triggers = 1"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set recursive_triggers: %w", err)
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

// SerializeProperties converts a frontmatter map to JSON for storage.
func SerializeProperties(fm map[string]interface{}) (string, error) {
	b, err := json.Marshal(fm)
	if err != nil {
		return "{}", fmt.Errorf("serialize properties: %w", err)
	}
	return string(b), nil
}

func (s *Store) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS _meta (
			key TEXT PRIMARY KEY,
			value TEXT
		);
		INSERT OR IGNORE INTO _meta (key, value) VALUES ('schema_version', '1');

		CREATE TABLE IF NOT EXISTS pages (
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
		);

		CREATE INDEX IF NOT EXISTS idx_pages_database_id ON pages(database_id);
		CREATE INDEX IF NOT EXISTS idx_pages_last_edited ON pages(last_edited_time);

		CREATE VIRTUAL TABLE IF NOT EXISTS pages_fts USING fts5(
			title, body_markdown, content=pages, content_rowid=rowid
		);

		CREATE TRIGGER IF NOT EXISTS pages_ai AFTER INSERT ON pages BEGIN
			INSERT INTO pages_fts(rowid, title, body_markdown)
			VALUES (new.rowid, new.title, new.body_markdown);
		END;

		CREATE TRIGGER IF NOT EXISTS pages_ad AFTER DELETE ON pages BEGIN
			INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
			VALUES('delete', old.rowid, old.title, old.body_markdown);
		END;

		CREATE TRIGGER IF NOT EXISTS pages_au AFTER UPDATE ON pages BEGIN
			INSERT INTO pages_fts(pages_fts, rowid, title, body_markdown)
			VALUES('delete', old.rowid, old.title, old.body_markdown);
			INSERT INTO pages_fts(rowid, title, body_markdown)
			VALUES (new.rowid, new.title, new.body_markdown);
		END;
	`
	_, err := s.db.Exec(schema)
	return err
}
