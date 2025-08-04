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
		{"PRAGMA user_version", "1"},
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
	_, err = db.Exec("INSERT INTO hook_events (timestamp, session_id, selected_path, fallback_level, context) VALUES (1234567890, 'test', 'test.wav', 1, '{}')")
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

	// Test fallback_level > 0 constraint
	_, err = db.Exec("INSERT INTO hook_events (timestamp, session_id, selected_path, fallback_level, context) VALUES (1234567891, 'test2', 'test.wav', 0, '{}')")
	if err == nil {
		t.Error("Expected CHECK constraint violation for fallback_level <= 0, but insert succeeded")
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