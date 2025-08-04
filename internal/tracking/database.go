package tracking

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite driver
)

// Database represents a SQLite database connection for sound tracking
type Database struct {
	*sql.DB
	path string
}

// NewDatabase creates a new SQLite database with the specified path and applies the schema
func NewDatabase(dbPath string) (*sql.DB, error) {
	// Ensure directory exists if not in-memory
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply pragmas first
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 10000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA user_version = 1",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Ensure schema exists
	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	return db, nil
}

// ensureSchema creates the database schema if it doesn't exist
func ensureSchema(db *sql.DB) error {
	// Create tables with exact schema from db.md
	schema := `
-- Main events table
CREATE TABLE IF NOT EXISTS hook_events (
    id             INTEGER PRIMARY KEY,
    timestamp      INTEGER NOT NULL,
    session_id     TEXT    NOT NULL,
    tool_name      TEXT,
    selected_path  TEXT    NOT NULL,
    fallback_level INTEGER NOT NULL CHECK (fallback_level > 0),
    context        JSON    NOT NULL
);

-- Individual path lookups
CREATE TABLE IF NOT EXISTS path_lookups (
    id       INTEGER PRIMARY KEY,
    event_id INTEGER NOT NULL REFERENCES hook_events(id) ON DELETE CASCADE,
    path     TEXT    NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    found    INTEGER NOT NULL CHECK (found IN (0,1)),
    UNIQUE(event_id, sequence),
    UNIQUE(event_id, path)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_events_timestamp ON hook_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_events_tool ON hook_events(tool_name);
CREATE INDEX IF NOT EXISTS idx_events_session ON hook_events(session_id);
CREATE INDEX IF NOT EXISTS idx_lookups_event ON path_lookups(event_id);
CREATE INDEX IF NOT EXISTS idx_lookups_missing ON path_lookups(path) WHERE found = 0;
`

	// Execute schema creation
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// GetDatabasePath returns the XDG-compliant path for the sounds database
func GetDatabasePath() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		// Fallback to current directory if XDG cache dir is not available
		cacheDir = "."
	}

	dbDir := filepath.Join(cacheDir, "claudio")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create database directory: %w", err)
	}

	return filepath.Join(dbDir, "sounds.db"), nil
}