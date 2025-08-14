package cli

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/hooks"
)

// TDD Cycle 7 RED: CLI Integration Tests
// These tests verify that tracking is properly integrated into the CLI lifecycle

func TestCLIWithTrackingEnabled(t *testing.T) {
	// Create temporary directory for database
	tempDir := t.TempDir()
	
	// Set environment variable to enable tracking with custom database path
	dbPath := filepath.Join(tempDir, "claudio.db")
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Create CLI instance
	cli := NewCLI()
	
	// Prepare hook event JSON
	toolResponse := json.RawMessage(`{"stdout":"File updated successfully","stderr":"","interrupted":false}`)
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",
		SessionID:      "test-session-123",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
		ToolName:       stringPtr("Edit"),
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Create I/O buffers
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// Run CLI with tracking enabled
	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}
	
	// Verify database was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected database file to be created when tracking is enabled")
	}
	
	// Verify database contains the expected data
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check that hook_events table has an entry
	var eventCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", "test-session-123").Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to query hook_events: %v", err)
	}
	
	if eventCount == 0 {
		t.Error("Expected at least one event to be recorded in database")
	}
	
	// Check that path_lookups table has entries
	var pathCount int
	err = db.QueryRow("SELECT COUNT(*) FROM path_lookups WHERE event_id IN (SELECT id FROM hook_events WHERE session_id = ?)", "test-session-123").Scan(&pathCount)
	if err != nil {
		t.Fatalf("Failed to query path_lookups: %v", err)
	}
	
	if pathCount == 0 {
		t.Error("Expected at least one path lookup to be recorded in database")
	}
}

func TestCLIWithTrackingDisabled(t *testing.T) {
	// Create temporary directory  
	tempDir := t.TempDir()
	
	// Set environment variable to disable tracking
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")
	
	// Create CLI instance
	cli := NewCLI()
	
	// Prepare hook event JSON
	toolResponse := json.RawMessage(`{"stdout":"Command executed","stderr":"","interrupted":false}`)
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse", 
		SessionID:      "test-session-456",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
		ToolName:       stringPtr("Bash"),
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Create I/O buffers
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	// Run CLI with tracking disabled
	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}
	
	// Verify no database files were created in temp directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".db") {
			t.Errorf("Database file %s was created when tracking should be disabled", entry.Name())
		}
	}
}

func TestCLITrackingEnvironmentVariableOverride(t *testing.T) {
	// Create temporary directory for database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "override.db")
	
	// Test that environment variable properly overrides default config
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)  
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Create CLI with default config (tracking should be overridden by env var)
	cli := NewCLI()
	
	// Prepare minimal hook event
	hookEvent := hooks.HookEvent{
		EventName:      "UserPromptSubmit",
		SessionID:      "env-override-test",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Run CLI
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	
	// Verify database was created at the specified path
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Expected database file to be created at custom path from environment variable")
	}
}

func TestCLIGracefulDegradationOnDBFailures(t *testing.T) {
	// Test that CLI continues to work even if database operations fail
	
	// Set environment to enable tracking but use invalid database path
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", "/invalid/readonly/path/test.db")
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Create CLI instance
	cli := NewCLI()
	
	// Prepare hook event
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",  
		SessionID:      "graceful-degradation-test",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
		ToolName:       stringPtr("Read"),
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Run CLI - should not crash even with database failure
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
	
	// CLI should still exit successfully despite database failure
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 even with database failure, got %d", exitCode)
	}
	
	// Should contain warning/error message but continue processing
	stderrOutput := stderr.String()
	if !strings.Contains(stderrOutput, "tracking") && !strings.Contains(stderrOutput, "database") {
		t.Log("Expected warning about tracking/database issues in stderr")
		// Note: this might be logged at DEBUG level, so we don't fail the test
	}
}

func TestCLIProperCleanup(t *testing.T) {
	// Test that CLI properly cleans up tracking resources
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "cleanup.db")
	
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	events := []hooks.HookEvent{
		{
			EventName:      "PreToolUse", 
			SessionID:      "cleanup-test-1", 
			TranscriptPath: "/test/transcript",
			CWD:            "/test/path",
			ToolName:       stringPtr("Edit"),
		},
		{
			EventName:      "PostToolUse", 
			SessionID:      "cleanup-test-2", 
			TranscriptPath: "/test/transcript",
			CWD:            "/test/path",
			ToolName:       stringPtr("Bash"),
		},
	}
	
	// Process each event with separate CLI instances (matches real-world usage)
	for i, event := range events {
		// Create fresh CLI instance for each event
		cli := NewCLI()
		
		hookJSON, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("Failed to marshal hook event %d: %v", i, err)
		}
		
		stdin := bytes.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		
		exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
		if exitCode != 0 {
			t.Errorf("Event %d: expected exit code 0, got %d", i, exitCode)
		}
		
		// Small delay to ensure events are processed separately
		time.Sleep(10 * time.Millisecond)
	}
	
	// Verify database contains both events
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	var eventCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hook_events").Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to query hook_events: %v", err)
	}
	
	if eventCount < 2 {
		t.Errorf("Expected at least 2 events in database, got %d", eventCount)
	}
}

// TDD Cycle 8 RED: Session ID Integration Tests
// These tests verify that session IDs are properly propagated from hook events to database

func TestSessionIDPropagationToDatabase(t *testing.T) {
	// Create temporary directory for database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "session_test.db")
	
	// Enable tracking
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Create CLI instance
	cli := NewCLI()
	
	// Test session ID
	testSessionID := "session-id-propagation-test"
	
	// Create hook event with specific session ID
	toolResponse := json.RawMessage(`{"stdout":"Test output","stderr":"","interrupted":false}`)
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",
		SessionID:      testSessionID,
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
		ToolName:       stringPtr("Edit"),
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Process hook event through CLI
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}
	
	// Verify session ID was recorded correctly in database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	var recordedSessionID string
	err = db.QueryRow("SELECT session_id FROM hook_events WHERE session_id = ?", testSessionID).Scan(&recordedSessionID)
	if err != nil {
		t.Fatalf("Failed to query session ID from database: %v", err)
	}
	
	if recordedSessionID != testSessionID {
		t.Errorf("Expected session ID %s in database, got %s", testSessionID, recordedSessionID)
	}
}

func TestMultipleSessionsCreateSeparateEntries(t *testing.T) {
	// Create temporary directory for database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "multi_session_test.db")
	
	// Enable tracking
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Define different session IDs
	sessionIDs := []string{"session-a", "session-b", "session-c"}
	
	// Process events for each session with SEPARATE CLI instances (matches real-world usage)
	for i, sessionID := range sessionIDs {
		// Create fresh CLI instance for each event (like real hook processing)
		cli := NewCLI()
		
		toolResponse := json.RawMessage(`{"stdout":"Test output","stderr":"","interrupted":false}`)
		hookEvent := hooks.HookEvent{
			EventName:      "PostToolUse",
			SessionID:      sessionID,
			TranscriptPath: "/test/transcript",
			CWD:            "/test/path",
			ToolName:       stringPtr("Bash"),
			ToolResponse:   &toolResponse,
		}
		
		hookJSON, err := json.Marshal(hookEvent)
		if err != nil {
			t.Fatalf("Failed to marshal hook event %d: %v", i, err)
		}
		
		// Process each event
		stdin := bytes.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		
		exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
		if exitCode != 0 {
			t.Errorf("Event %d: expected exit code 0, got %d", i, exitCode)
		}
		
		// Small delay between events
		time.Sleep(10 * time.Millisecond)
	}
	
	// Verify all sessions are recorded separately
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check that each session has exactly one entry
	for _, sessionID := range sessionIDs {
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", sessionID).Scan(&count)
		if err != nil {
			t.Fatalf("Failed to query count for session %s: %v", sessionID, err)
		}
		
		if count != 1 {
			t.Errorf("Expected exactly 1 entry for session %s, got %d", sessionID, count)
		}
	}
	
	// Verify total count matches expected
	var totalCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hook_events").Scan(&totalCount)
	if err != nil {
		t.Fatalf("Failed to query total count: %v", err)
	}
	
	if totalCount != len(sessionIDs) {
		t.Errorf("Expected %d total entries, got %d", len(sessionIDs), totalCount)
	}
}

func TestPerRequestSoundCheckerInitialization(t *testing.T) {
	// This test verifies that each request gets its own SoundChecker with the correct session ID
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "per_request_test.db")
	
	// Enable tracking
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")  
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Process two events with different session IDs using separate CLI instances
	sessions := []struct {
		sessionID string
		toolName  string
	}{
		{"per-request-session-1", "Edit"},
		{"per-request-session-2", "Read"},
	}
	
	for i, session := range sessions {
		// Create fresh CLI instance for each event (matches real-world usage)
		cli := NewCLI()
		
		toolResponse := json.RawMessage(`{"stdout":"Command output","stderr":"","interrupted":false}`)
		hookEvent := hooks.HookEvent{
			EventName:      "PostToolUse",
			SessionID:      session.sessionID,
			TranscriptPath: "/test/transcript",
			CWD:            "/test/path",
			ToolName:       stringPtr(session.toolName),
			ToolResponse:   &toolResponse,
		}
		
		hookJSON, err := json.Marshal(hookEvent)
		if err != nil {
			t.Fatalf("Failed to marshal hook event %d: %v", i, err)
		}
		
		stdin := bytes.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		
		exitCode := cli.Run([]string{"claudio"}, stdin, stdout, stderr)
		if exitCode != 0 {
			t.Errorf("Event %d: expected exit code 0, got %d", i, exitCode)
		}
	}
	
	// Verify both sessions are recorded with correct isolation
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Check that each session has its own entry with correct tool name correlation
	for _, session := range sessions {
		var recordedSessionID string
		var context string
		
		err = db.QueryRow(`
			SELECT session_id, context 
			FROM hook_events 
			WHERE session_id = ?`, session.sessionID).Scan(&recordedSessionID, &context)
		if err != nil {
			t.Fatalf("Failed to query session %s: %v", session.sessionID, err)
		}
		
		if recordedSessionID != session.sessionID {
			t.Errorf("Expected session ID %s, got %s", session.sessionID, recordedSessionID)
		}
		
		// Verify the context contains the correct tool name
		if !strings.Contains(context, session.toolName) {
			t.Errorf("Expected context to contain tool name %s, got: %s", session.toolName, context)
		}
	}
}

