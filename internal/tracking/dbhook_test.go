package tracking

import (
	"encoding/json"
	"testing"
	"time"

	"claudio.click/internal/hooks"
)

func TestNewDBHook(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-123"

	hook := NewDBHook(db, sessionID)
	if hook == nil {
		t.Fatal("NewDBHook returned nil")
	}

	if hook.sessionID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, hook.sessionID)
	}

	if hook.disabled {
		t.Error("New DBHook should not be disabled initially")
	}
}

func TestDBHookLogPathCheck(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-session-456"
	hook := NewDBHook(db, sessionID)

	// Create test context
	context := &hooks.EventContext{
		Category:     hooks.Success,
		ToolName:     "git",
		OriginalTool: "Bash",
		IsSuccess:    true,
		SoundHint:    "git-commit-success",
		Operation:    "tool-complete",
	}

	// Log some path checks
	hook.LogPathCheck("success/git-commit-success.wav", false, 1, "posttool", context)
	hook.LogPathCheck("success/git-success.wav", false, 2, "posttool", context)
	hook.LogPathCheck("success/success.wav", true, 3, "posttool", context)

	// Verify data was inserted correctly
	var eventCount int
	err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to query hook_events: %v", err)
	}
	if eventCount != 1 {
		t.Errorf("Expected 1 event, got %d", eventCount)
	}

	var pathCount int
	err = db.QueryRow("SELECT COUNT(*) FROM path_lookups pl JOIN hook_events he ON pl.event_id = he.id WHERE he.session_id = ?", sessionID).Scan(&pathCount)
	if err != nil {
		t.Fatalf("Failed to query path_lookups: %v", err)
	}
	if pathCount != 3 {
		t.Errorf("Expected 3 path lookups, got %d", pathCount)
	}
}

func TestDBHookTransactionHandling(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-transaction"
	hook := NewDBHook(db, sessionID)

	context := &hooks.EventContext{
		Category:  hooks.Error,
		ToolName:  "bash",
		IsSuccess: false,
		HasError:  true,
	}

	// Log multiple path checks that should be grouped into a single event
	hook.LogPathCheck("error/bash-error.wav", false, 1, "posttool", context)
	hook.LogPathCheck("error/bash.wav", false, 2, "posttool", context)
	hook.LogPathCheck("error/error.wav", true, 3, "posttool", context)

	// Verify all paths are associated with the same event
	rows, err := db.Query(`
		SELECT pl.path, pl.sequence, pl.found, he.selected_path
		FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE he.session_id = ?
		ORDER BY pl.sequence`, sessionID)
	if err != nil {
		t.Fatalf("Failed to query path lookups: %v", err)
	}
	defer rows.Close()

	expectedPaths := []struct {
		path         string
		sequence     int
		found        bool
		selectedPath string
	}{
		{"error/bash-error.wav", 1, false, "error/error.wav"},
		{"error/bash.wav", 2, false, "error/error.wav"},
		{"error/error.wav", 3, true, "error/error.wav"},
	}

	i := 0
	for rows.Next() {
		if i >= len(expectedPaths) {
			t.Error("More rows than expected")
			break
		}

		var path, selectedPath string
		var sequence int
		var found bool

		err := rows.Scan(&path, &sequence, &found, &selectedPath)
		if err != nil {
			t.Fatalf("Failed to scan row: %v", err)
		}

		expected := expectedPaths[i]
		if path != expected.path {
			t.Errorf("Row %d: expected path %s, got %s", i, expected.path, path)
		}
		if sequence != expected.sequence {
			t.Errorf("Row %d: expected sequence %d, got %d", i, expected.sequence, sequence)
		}
		if found != expected.found {
			t.Errorf("Row %d: expected found %t, got %t", i, expected.found, found)
		}
		if selectedPath != expected.selectedPath {
			t.Errorf("Row %d: expected selectedPath %s, got %s", i, expected.selectedPath, selectedPath)
		}
		i++
	}

	if i != len(expectedPaths) {
		t.Errorf("Expected %d rows, got %d", len(expectedPaths), i)
	}
}

func TestDBHookJSONContextMarshaling(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-json"
	hook := NewDBHook(db, sessionID)

	context := &hooks.EventContext{
		Category:     hooks.Loading,
		ToolName:     "git",
		OriginalTool: "Bash",
		IsSuccess:    false,
		HasError:     false,
		SoundHint:    "git-commit-start",
		FileType:     "go",
		Operation:    "tool-start",
	}

	hook.LogPathCheck("loading/git-commit-start.wav", true, 1, "enhanced", context)

	// Verify context was stored as valid JSON
	var contextJSON string
	err := db.QueryRow("SELECT context FROM hook_events WHERE session_id = ?", sessionID).Scan(&contextJSON)
	if err != nil {
		t.Fatalf("Failed to query context: %v", err)
	}

	// Verify it's valid JSON by unmarshaling
	var unmarshaled hooks.EventContext
	err = json.Unmarshal([]byte(contextJSON), &unmarshaled)
	if err != nil {
		t.Fatalf("Context is not valid JSON: %v", err)
	}

	// Verify key fields were preserved
	if unmarshaled.Category != context.Category {
		t.Errorf("Expected category %v, got %v", context.Category, unmarshaled.Category)
	}
	if unmarshaled.ToolName != context.ToolName {
		t.Errorf("Expected tool name %s, got %s", context.ToolName, unmarshaled.ToolName)
	}
	if unmarshaled.SoundHint != context.SoundHint {
		t.Errorf("Expected sound hint %s, got %s", context.SoundHint, unmarshaled.SoundHint)
	}
}

func TestDBHookEventGrouping(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-grouping"
	hook := NewDBHook(db, sessionID)

	context1 := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "git",
		Operation: "tool-complete",
	}

	context2 := &hooks.EventContext{
		Category:  hooks.Loading,
		ToolName:  "bash",
		Operation: "tool-start",
	}

	// First event - multiple paths
	hook.LogPathCheck("success/git.wav", true, 1, "posttool", context1)
	hook.LogPathCheck("success/success.wav", false, 2, "posttool", context1)

	// Second event - different context
	hook.LogPathCheck("loading/bash.wav", false, 1, "enhanced", context2)

	// Should have 2 events total
	var eventCount int
	err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to count events: %v", err)
	}
	if eventCount != 2 {
		t.Errorf("Expected 2 events, got %d", eventCount)
	}

	// First event should have 2 paths, second should have 1
	rows, err := db.Query(`
		SELECT he.tool_name, COUNT(pl.id) as path_count
		FROM hook_events he
		LEFT JOIN path_lookups pl ON he.id = pl.event_id
		WHERE he.session_id = ?
		GROUP BY he.id
		ORDER BY he.timestamp`, sessionID)
	if err != nil {
		t.Fatalf("Failed to query event grouping: %v", err)
	}
	defer rows.Close()

	expectedCounts := []struct {
		toolName  string
		pathCount int
	}{
		{"git", 2},
		{"bash", 1},
	}

	i := 0
	for rows.Next() {
		if i >= len(expectedCounts) {
			t.Error("More events than expected")
			break
		}

		var toolName string
		var pathCount int
		err := rows.Scan(&toolName, &pathCount)
		if err != nil {
			t.Fatalf("Failed to scan event: %v", err)
		}

		expected := expectedCounts[i]
		if toolName != expected.toolName {
			t.Errorf("Event %d: expected tool %s, got %s", i, expected.toolName, toolName)
		}
		if pathCount != expected.pathCount {
			t.Errorf("Event %d: expected %d paths, got %d", i, expected.pathCount, pathCount)
		}
		i++
	}
}

func TestDBHookGracefulDegradation(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-degradation"
	hook := NewDBHook(db, sessionID)

	// Close the database to simulate failure
	db.Close()

	context := &hooks.EventContext{
		Category: hooks.Error,
		ToolName: "test",
	}

	// This should not panic or cause the program to fail
	hook.LogPathCheck("error/test.wav", false, 1, "posttool", context)

	// Hook should be disabled after failure
	if !hook.disabled {
		t.Error("DBHook should be disabled after database failure")
	}

	// Subsequent calls should be no-ops
	hook.LogPathCheck("error/test2.wav", false, 2, "posttool", context)
	// Should not panic
}

func TestDBHookGetHook(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-get-hook"
	dbHook := NewDBHook(db, sessionID)

	// Get the PathCheckedHook function
	hook := dbHook.GetHook()
	if hook == nil {
		t.Fatal("GetHook returned nil")
	}

	// Test that the hook function works
	context := &hooks.EventContext{
		Category: hooks.Success,
		ToolName: "test",
	}

	// Should not panic
	hook("success/test.wav", true, 1, "posttool", context)

	// Verify data was inserted
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query events: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 event, got %d", count)
	}
}

func TestDBHookTimestampHandling(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-timestamp"
	hook := NewDBHook(db, sessionID)

	context := &hooks.EventContext{
		Category: hooks.Success,
		ToolName: "test",
	}

	startTime := time.Now().Unix()
	hook.LogPathCheck("success/test.wav", true, 1, "posttool", context)
	endTime := time.Now().Unix()

	var timestamp int64
	err := db.QueryRow("SELECT timestamp FROM hook_events WHERE session_id = ?", sessionID).Scan(&timestamp)
	if err != nil {
		t.Fatalf("Failed to query timestamp: %v", err)
	}

	if timestamp < startTime || timestamp > endTime {
		t.Errorf("Timestamp %d is not within expected range [%d, %d]", timestamp, startTime, endTime)
	}
}

// TestDBHook_WritesChainType pins the contract that the chain type passed
// to LogPathCheck lands on the hook_events row (and survives the
// "later-existing-path wins" update path).
func TestDBHook_WritesChainType(t *testing.T) {
	db := setupTestDB(t)
	sessionID := "test-chain-type"
	hook := NewDBHook(db, sessionID)

	context := &hooks.EventContext{
		Category:  hooks.Loading,
		ToolName:  "git",
		Operation: "tool-start",
	}

	// Simulate a 3-path enhanced chain where the second path exists.
	hook.LogPathCheck("loading/git-commit-start.wav", false, 1, "enhanced", context)
	hook.LogPathCheck("loading/git-start.wav", true, 2, "enhanced", context)
	hook.LogPathCheck("loading/loading.wav", false, 3, "enhanced", context)

	var chainType string
	if err := db.QueryRow(
		"SELECT chain_type FROM hook_events WHERE session_id = ?",
		sessionID).Scan(&chainType); err != nil {
		t.Fatalf("query chain_type: %v", err)
	}
	if chainType != "enhanced" {
		t.Errorf("expected chain_type 'enhanced', got %q", chainType)
	}

	// selected_path should reflect the existing path (level 2).
	var selectedPath string
	if err := db.QueryRow(
		"SELECT selected_path FROM hook_events WHERE session_id = ?",
		sessionID).Scan(&selectedPath); err != nil {
		t.Fatalf("query selected_path: %v", err)
	}
	if selectedPath != "loading/git-start.wav" {
		t.Errorf("expected selected_path 'loading/git-start.wav', got %q", selectedPath)
	}

	// path_lookups should record all three sequence numbers.
	rows, err := db.Query(
		`SELECT path, sequence, found FROM path_lookups pl
		JOIN hook_events he ON pl.event_id = he.id
		WHERE he.session_id = ? ORDER BY sequence`, sessionID)
	if err != nil {
		t.Fatalf("query path_lookups: %v", err)
	}
	defer rows.Close()

	var lookups []struct {
		path     string
		sequence int
		found    int
	}
	for rows.Next() {
		var p string
		var s, f int
		if err := rows.Scan(&p, &s, &f); err != nil {
			t.Fatalf("scan path lookup: %v", err)
		}
		lookups = append(lookups, struct {
			path     string
			sequence int
			found    int
		}{p, s, f})
	}
	if len(lookups) != 3 {
		t.Fatalf("expected 3 path_lookups, got %d", len(lookups))
	}
	if lookups[1].sequence != 2 || lookups[1].found != 1 {
		t.Errorf("expected sequence=2 found=1 at lookup[1], got %+v", lookups[1])
	}
}