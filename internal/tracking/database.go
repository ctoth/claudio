package tracking

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // SQLite driver
)

// schemaUserVersion is the current schema version. Incremented when the
// schema changes; migrate() walks any older DB up to this version.
const schemaUserVersion = 2

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

	// Apply pragmas first (user_version handled by migrate() below)
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 10000",
		"PRAGMA temp_store = MEMORY",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to set pragma: %w", err)
		}
	}

	// Ensure schema exists (fresh DBs get the current shape directly)
	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ensure schema: %w", err)
	}

	// Apply any pending migrations (existing DBs get rewritten to current
	// shape; fresh DBs just have user_version stamped).
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate schema: %w", err)
	}

	return db, nil
}

// ensureSchema creates the database schema if it doesn't exist
func ensureSchema(db *sql.DB) error {
	// Fresh-database schema is the current shape:
	//   - no fallback_level (conflated path-index across three chain shapes;
	//     see review finding #20)
	//   - chain_type TEXT nullable so the column round-trips through the
	//     migration on existing DBs that have no value to backfill
	schema := `
-- Main events table
CREATE TABLE IF NOT EXISTS hook_events (
    id             INTEGER PRIMARY KEY,
    timestamp      INTEGER NOT NULL,
    session_id     TEXT    NOT NULL,
    tool_name      TEXT,
    selected_path  TEXT    NOT NULL,
    chain_type     TEXT,
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

// migrate brings a database up to schemaUserVersion. Idempotent: fresh
// databases just have user_version stamped; existing databases get any
// missing migrations applied in sequence.
func migrate(db *sql.DB) error {
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	if v < 2 {
		if err := migrateToV2(db); err != nil {
			return err
		}
	}

	return nil
}

// migrateToV2 drops the conflated fallback_level column and adds the
// chain_type column. Idempotent: detects columns via PRAGMA table_info
// so it can also stamp user_version on a fresh DB that already has the
// target shape.
func migrateToV2(db *sql.DB) error {
	hasFallback, hasChainType, err := hookEventsColumns(db)
	if err != nil {
		return fmt.Errorf("inspect hook_events columns: %w", err)
	}

	if hasFallback {
		if _, err := db.Exec("ALTER TABLE hook_events DROP COLUMN fallback_level"); err != nil {
			return fmt.Errorf("drop fallback_level: %w", err)
		}
	}
	if !hasChainType {
		if _, err := db.Exec("ALTER TABLE hook_events ADD COLUMN chain_type TEXT"); err != nil {
			return fmt.Errorf("add chain_type: %w", err)
		}
	}
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}

// hookEventsColumns reports which of the schema-migration-relevant columns
// exist on hook_events today.
func hookEventsColumns(db *sql.DB) (hasFallback, hasChainType bool, err error) {
	rows, err := db.Query("PRAGMA table_info(hook_events)")
	if err != nil {
		return false, false, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cid     int
			name    string
			ctype   string
			notnull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, false, err
		}
		switch name {
		case "fallback_level":
			hasFallback = true
		case "chain_type":
			hasChainType = true
		}
	}
	if err := rows.Err(); err != nil {
		return false, false, err
	}
	return hasFallback, hasChainType, nil
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
