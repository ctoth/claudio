package tracking

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("NewDatabase returned nil")
	}

	// Test that database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestDatabaseSchemaExists(t *testing.T) {
	db := setupTestDB(t)

	// Test that both tables exist by querying them
	tables := []string{"hook_events", "path_lookups"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("Table %s does not exist or is not queryable: %v", table, err)
		}
	}
}

func TestDatabaseIndexesExist(t *testing.T) {
	db := setupTestDB(t)

	// Query sqlite_master to check all 5 indexes exist
	expectedIndexes := []string{
		"idx_events_timestamp",
		"idx_events_tool",
		"idx_events_session",
		"idx_lookups_event",
		"idx_lookups_missing",
	}

	for _, indexName := range expectedIndexes {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&count)
		if err != nil {
			t.Errorf("Failed to query for index %s: %v", indexName, err)
		}
		if count != 1 {
			t.Errorf("Index %s does not exist (found %d entries)", indexName, count)
		}
	}
}

func TestGetDatabasePath(t *testing.T) {
	path, err := GetDatabasePath()
	if err != nil {
		t.Fatalf("GetDatabasePath failed: %v", err)
	}

	if path == "" {
		t.Error("GetDatabasePath returned empty string")
	}

	// Should end with claudio/sounds.db
	if !strings.HasSuffix(path, filepath.Join("claudio", "sounds.db")) {
		t.Errorf("Database path doesn't end with expected suffix: %s", path)
	}

	// Should be an absolute path
	if !filepath.IsAbs(path) {
		t.Errorf("Database path is not absolute: %s", path)
	}
}

func TestDatabasePragmasApplied(t *testing.T) {
	db := setupTestDB(t)

	// Test that key pragmas were applied
	pragmaTests := []struct {
		pragma   string
		expected string
	}{
		{"PRAGMA user_version", "2"},
		{"PRAGMA busy_timeout", "10000"},
		{"PRAGMA synchronous", "1"}, // NORMAL = 1
		{"PRAGMA temp_store", "2"},  // MEMORY = 2
	}

	for _, test := range pragmaTests {
		var value string
		err := db.QueryRow(test.pragma).Scan(&value)
		if err != nil {
			t.Errorf("Failed to query %s: %v", test.pragma, err)
		}
		if value != test.expected {
			t.Errorf("%s: expected %s, got %s", test.pragma, test.expected, value)
		}
	}
}

func TestDatabaseConstraints(t *testing.T) {
	db := setupTestDB(t)

	// Test foreign key constraint exists
	_, err := db.Exec("INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (99999, 'test.wav', 1, 0)")
	if err == nil {
		t.Error("Expected foreign key constraint violation, but insert succeeded")
	}

	// Test CHECK constraints
	// Test sequence > 0 constraint
	_, err = db.Exec("INSERT INTO hook_events (timestamp, session_id, selected_path, chain_type, context) VALUES (1234567890, 'test', 'test.wav', 'enhanced', '{}')")
	if err != nil {
		t.Fatalf("Failed to insert test event: %v", err)
	}

	var eventID int64
	err = db.QueryRow("SELECT id FROM hook_events WHERE session_id = 'test'").Scan(&eventID)
	if err != nil {
		t.Fatalf("Failed to get event ID: %v", err)
	}

	_, err = db.Exec("INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (?, 'test.wav', 0, 0)", eventID)
	if err == nil {
		t.Error("Expected CHECK constraint violation for sequence <= 0, but insert succeeded")
	}
}

// TestMigration_DropsFallbackLevel verifies the v2 migration drops the
// pre-existing fallback_level column from a legacy database and adds the
// new chain_type column.
func TestMigration_DropsFallbackLevel(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "legacy_v1.db")

	// Build a database matching the v1 schema directly (no NewDatabase
	// shortcut — we need to simulate an existing user's DB).
	rawDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	legacySchema := `
CREATE TABLE hook_events (
    id             INTEGER PRIMARY KEY,
    timestamp      INTEGER NOT NULL,
    session_id     TEXT    NOT NULL,
    tool_name      TEXT,
    selected_path  TEXT    NOT NULL,
    fallback_level INTEGER NOT NULL CHECK (fallback_level > 0),
    context        JSON    NOT NULL
);
CREATE TABLE path_lookups (
    id       INTEGER PRIMARY KEY,
    event_id INTEGER NOT NULL REFERENCES hook_events(id) ON DELETE CASCADE,
    path     TEXT    NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    found    INTEGER NOT NULL CHECK (found IN (0,1)),
    UNIQUE(event_id, sequence),
    UNIQUE(event_id, path)
);
PRAGMA user_version = 1;
`
	if _, err := rawDB.Exec(legacySchema); err != nil {
		rawDB.Close()
		t.Fatalf("seed legacy schema: %v", err)
	}

	// Seed one row so we can confirm data survives the column drop.
	if _, err := rawDB.Exec(`INSERT INTO hook_events
		(timestamp, session_id, tool_name, selected_path, fallback_level, context)
		VALUES (1700000000, 'legacy-session', 'Edit', 'success/edit.wav', 3, '{"Category":1,"ToolName":"Edit"}')`); err != nil {
		rawDB.Close()
		t.Fatalf("seed legacy row: %v", err)
	}
	if err := rawDB.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	// Reopen via the production path — migrations should run.
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase on legacy db: %v", err)
	}
	defer db.Close()

	// PRAGMA user_version bumped to 2.
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 2 {
		t.Errorf("expected user_version 2 after migration, got %d", version)
	}

	// fallback_level column is gone, chain_type column is present.
	hasFallback, hasChainType, err := hookEventsColumns(db)
	if err != nil {
		t.Fatalf("inspect columns: %v", err)
	}
	if hasFallback {
		t.Error("expected fallback_level column dropped after migration")
	}
	if !hasChainType {
		t.Error("expected chain_type column present after migration")
	}

	// Querying the dropped column should fail.
	_, err = db.Query("SELECT fallback_level FROM hook_events")
	if err == nil {
		t.Error("expected SELECT fallback_level to fail after column drop")
	}

	// Pre-existing row still exists.
	var sessionID, toolName string
	if err := db.QueryRow("SELECT session_id, tool_name FROM hook_events WHERE id = 1").Scan(&sessionID, &toolName); err != nil {
		t.Fatalf("query preserved row: %v", err)
	}
	if sessionID != "legacy-session" || toolName != "Edit" {
		t.Errorf("legacy row not preserved: sessionID=%q toolName=%q", sessionID, toolName)
	}

	// Migration is idempotent — running NewDatabase again is a no-op.
	db.Close()
	db2, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("NewDatabase second open: %v", err)
	}
	defer db2.Close()
	if err := db2.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version after second open: %v", err)
	}
	if version != 2 {
		t.Errorf("expected user_version still 2 after second open, got %d", version)
	}
}

// TestFreshDatabase_NoFallbackLevelColumn verifies a brand new database
// is created with the v2 shape directly (no fallback_level).
func TestFreshDatabase_NoFallbackLevelColumn(t *testing.T) {
	db := setupTestDB(t)

	hasFallback, hasChainType, err := hookEventsColumns(db)
	if err != nil {
		t.Fatalf("inspect columns: %v", err)
	}
	if hasFallback {
		t.Error("fresh database should not have fallback_level column")
	}
	if !hasChainType {
		t.Error("fresh database should have chain_type column")
	}

	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != 2 {
		t.Errorf("expected fresh database user_version=2, got %d", version)
	}
}

// setupTestDB creates an in-memory test database with schema applied
func setupTestDB(t *testing.T) *sql.DB {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return db
}