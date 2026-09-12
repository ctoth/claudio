package tracking

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"modernc.org/sqlite"
)

// schemaUserVersion is the current schema version. Incremented when the
// schema changes; migrate() walks any older DB up to this version.
const schemaUserVersion = 2

const schemaConnectionRetryTimeout = 10 * time.Second

// NewDatabase creates a new SQLite database with the specified path and applies the schema
func NewDatabase(dbPath string) (*sql.DB, error) {
	// Ensure directory exists if not in-memory
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Build a DSN that pushes connection-scoped pragmas onto every pool
	// connection rather than priming only the first one with db.Exec.
	// Chunk 13's fix-up surfaced this: db.Exec("PRAGMA busy_timeout = ...")
	// runs against whichever single connection database/sql happened to
	// hand it; later pool connections opened under concurrent load inherit
	// the 0-ms default and fail fast with SQLITE_BUSY. modernc.org/sqlite
	// honors the _pragma=...(value) query parameter on EVERY connection
	// it opens, which is what we need. user_version is intentionally NOT
	// part of the DSN; migrate() owns that column.
	//
	// `:memory:` is a special case: every connection in the pool would get
	// its own private in-memory database, and the DSN's `file:` prefix
	// would change the URI shape. We keep the bare path for :memory: and
	// fall back to db.Exec to prime its single connection.
	dsn := buildDSN(dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	if dbPath == ":memory:" {
		// Each SQLite :memory: connection owns a separate database. Pin the
		// pool so schema creation and every later query use the same one.
		db.SetMaxOpenConns(1)
	}

	// Belt-and-suspenders for `:memory:` and for any consumer relying on
	// PRAGMA visibility on the very first connection: re-exec the same
	// pragmas via db.Exec. For file-backed DBs the DSN already applied
	// them; for `:memory:` this is the only path. user_version is owned
	// by migrate() and intentionally omitted here.
	if dbPath == ":memory:" {
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
	}

	// Schema creation and migrations share one connection and transaction.
	// BEGIN IMMEDIATE reserves the writer before inspecting user_version, so
	// concurrent openers serialize instead of running the same DDL together.
	if err := initializeSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

type schemaConnection interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func initializeSchema(db *sql.DB) error {
	ctx := context.Background()
	conn, err := acquireSchemaConnection(ctx, db)
	if err != nil {
		return fmt.Errorf("acquire schema connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
		}
	}()

	if err := ensureSchema(ctx, conn); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	if err := migrate(ctx, conn); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		return fmt.Errorf("commit schema transaction: %w", err)
	}
	committed = true
	return nil
}

func acquireSchemaConnection(ctx context.Context, db *sql.DB) (*sql.Conn, error) {
	deadline := time.Now().Add(schemaConnectionRetryTimeout)
	for {
		conn, err := db.Conn(ctx)
		if err == nil {
			return conn, nil
		}
		if !isSQLiteBusy(err) || time.Now().After(deadline) {
			return nil, err
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func isSQLiteBusy(err error) bool {
	var sqliteErr *sqlite.Error
	return errors.As(err, &sqliteErr) && sqliteErr.Code()&0xff == 5
}

// ensureSchema creates the database schema if it doesn't exist
func ensureSchema(ctx context.Context, db schemaConnection) error {
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
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// migrate brings a database up to schemaUserVersion. Idempotent: fresh
// databases just have user_version stamped; existing databases get any
// missing migrations applied in sequence.
func migrate(ctx context.Context, db schemaConnection) error {
	var v int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&v); err != nil {
		return fmt.Errorf("read user_version: %w", err)
	}

	if v < schemaUserVersion {
		if err := migrateToV2(ctx, db); err != nil {
			return err
		}
	}

	return nil
}

// migrateToV2 drops the conflated fallback_level column and adds the
// chain_type column. Idempotent: detects columns via PRAGMA table_info
// so it can also stamp user_version on a fresh DB that already has the
// target shape.
func migrateToV2(ctx context.Context, db schemaConnection) error {
	hasFallback, hasChainType, err := hookEventsColumnsContext(ctx, db)
	if err != nil {
		return fmt.Errorf("inspect hook_events columns: %w", err)
	}

	if hasFallback {
		if _, err := db.ExecContext(ctx, "ALTER TABLE hook_events DROP COLUMN fallback_level"); err != nil {
			return fmt.Errorf("drop fallback_level: %w", err)
		}
	}
	if !hasChainType {
		if _, err := db.ExecContext(ctx, "ALTER TABLE hook_events ADD COLUMN chain_type TEXT"); err != nil {
			return fmt.Errorf("add chain_type: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}
	return nil
}

// hookEventsColumns reports which of the schema-migration-relevant columns
// exist on hook_events today.
func hookEventsColumns(db *sql.DB) (hasFallback, hasChainType bool, err error) {
	return hookEventsColumnsContext(context.Background(), db)
}

func hookEventsColumnsContext(ctx context.Context, db schemaConnection) (hasFallback, hasChainType bool, err error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info(hook_events)")
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

// buildDSN constructs a modernc.org/sqlite DSN that embeds the
// connection-scoped pragmas the schema relies on. The driver applies these
// to every pool connection it opens, closing the chunk-13 finding that
// db.Exec("PRAGMA busy_timeout = ...") only primed one connection.
//
// `:memory:` is preserved verbatim — each connection to `:memory:` gets a
// private database by SQLite design, so the production pragmas are
// irrelevant there and tests rely on the bare form.
func buildDSN(dbPath string) string {
	if dbPath == ":memory:" {
		return dbPath
	}
	const pragmas = "_pragma=busy_timeout(10000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=foreign_keys(ON)"
	return "file:" + dbPath + "?" + pragmas
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
