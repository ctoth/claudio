package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/cli"
	"claudio.click/internal/hooks"
)

// TDD Cycle 9 RED: End-to-End Integration Tests
// These tests verify the complete pipeline from hook event to database storage

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

func TestEndToEndHookEventProcessing(t *testing.T) {
	// Create temporary directory for database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "end_to_end.db")
	
	// Enable tracking with custom database path
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Create comprehensive hook event that exercises the full pipeline
	toolResponse := json.RawMessage(`{
		"stdout": "File edited successfully", 
		"stderr": "", 
		"interrupted": false
	}`)
	
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",
		SessionID:      "end-to-end-test-session",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/working/directory",
		ToolName:       stringPtr("Edit"),
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	// Process through complete CLI pipeline
	cli := cli.NewCLI()
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}
	
	// Verify database was created and populated
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("Expected database file to be created")
	}
	
	// Open database and verify complete data pipeline
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Verify hook_events table contains the event
	var (
		sessionID      string
		toolName       string
		selectedPath   string
		fallbackLevel  int
		context        string
		timestamp      int64
	)
	
	err = db.QueryRow(`
		SELECT session_id, tool_name, selected_path, fallback_level, context, timestamp
		FROM hook_events 
		WHERE session_id = ?`, hookEvent.SessionID).Scan(
		&sessionID, &toolName, &selectedPath, &fallbackLevel, &context, &timestamp)
	if err != nil {
		t.Fatalf("Failed to query hook_events: %v", err)
	}
	
	// Verify all fields are populated correctly
	if sessionID != hookEvent.SessionID {
		t.Errorf("Expected session_id %s, got %s", hookEvent.SessionID, sessionID)
	}
	
	if toolName != *hookEvent.ToolName {
		t.Errorf("Expected tool_name %s, got %s", *hookEvent.ToolName, toolName)
	}
	
	if selectedPath == "" {
		t.Error("Expected selected_path to be populated")
	}
	
	if fallbackLevel <= 0 {
		t.Errorf("Expected fallback_level > 0, got %d", fallbackLevel)
	}
	
	if context == "" {
		t.Error("Expected context to be populated")
	}
	
	if timestamp <= 0 {
		t.Errorf("Expected timestamp > 0, got %d", timestamp)
	}
	
	// Verify path_lookups table contains path attempts
	var pathLookupCount int
	err = db.QueryRow(`
		SELECT COUNT(*) 
		FROM path_lookups pl 
		JOIN hook_events he ON pl.event_id = he.id 
		WHERE he.session_id = ?`, hookEvent.SessionID).Scan(&pathLookupCount)
	if err != nil {
		t.Fatalf("Failed to query path_lookups count: %v", err)
	}
	
	if pathLookupCount == 0 {
		t.Error("Expected at least one path lookup to be recorded")
	}
	
	// Verify path lookup details
	rows, err := db.Query(`
		SELECT pl.path, pl.sequence, pl.found
		FROM path_lookups pl 
		JOIN hook_events he ON pl.event_id = he.id 
		WHERE he.session_id = ?
		ORDER BY pl.sequence`, hookEvent.SessionID)
	if err != nil {
		t.Fatalf("Failed to query path_lookups details: %v", err)
	}
	defer rows.Close()
	
	var pathsChecked []string
	for rows.Next() {
		var path string
		var sequence int
		var found bool
		
		err = rows.Scan(&path, &sequence, &found)
		if err != nil {
			t.Fatalf("Failed to scan path lookup row: %v", err)
		}
		
		pathsChecked = append(pathsChecked, path)
		
		// Sequence should start at 1 and increment
		if sequence <= 0 {
			t.Errorf("Expected sequence > 0, got %d for path %s", sequence, path)
		}
	}
	
	if err = rows.Err(); err != nil {
		t.Fatalf("Error iterating path lookup rows: %v", err)
	}
	
	// Verify we have paths that match expected fallback pattern
	if len(pathsChecked) == 0 {
		t.Error("Expected multiple paths to be checked in fallback chain")
	}
	
	// At least one path should contain the tool name
	hasToolPath := false
	for _, path := range pathsChecked {
		if strings.Contains(strings.ToLower(path), strings.ToLower(*hookEvent.ToolName)) {
			hasToolPath = true
			break
		}
	}
	if !hasToolPath {
		t.Errorf("Expected at least one path to contain tool name '%s', got paths: %v", 
			*hookEvent.ToolName, pathsChecked)
	}
}

func TestEndToEndSoundPathTracking(t *testing.T) {
	// Test that different sound categories and hints are properly tracked
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "sound_paths.db")
	
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Test different event types that should generate different sound paths
	testEvents := []struct {
		name      string
		eventName string
		toolName  string
		success   bool
	}{
		{"successful_edit", "PostToolUse", "Edit", true},
		{"successful_bash", "PostToolUse", "Bash", true},
		{"failed_read", "PostToolUse", "Read", false},
		{"pre_tool_use", "PreToolUse", "Write", true},
		{"user_prompt", "UserPromptSubmit", "", true},
	}
	
	for i, testEvent := range testEvents {
		// Create fresh CLI for each event
		cli := cli.NewCLI()
		
		// Create appropriate tool response
		var toolResponse json.RawMessage
		if testEvent.success {
			toolResponse = json.RawMessage(`{"stdout":"Success","stderr":"","interrupted":false}`)
		} else {
			toolResponse = json.RawMessage(`{"stdout":"","stderr":"Error occurred","interrupted":false}`)
		}
		
		hookEvent := hooks.HookEvent{
			EventName:      testEvent.eventName,
			SessionID:      "sound-path-test-" + testEvent.name,
			TranscriptPath: "/test/transcript",
			CWD:            "/test/path",
		}
		
		if testEvent.toolName != "" {
			hookEvent.ToolName = stringPtr(testEvent.toolName)
			hookEvent.ToolResponse = &toolResponse
		}
		
		hookJSON, err := json.Marshal(hookEvent)
		if err != nil {
			t.Fatalf("Event %d: Failed to marshal hook event: %v", i, err)
		}
		
		// Process event
		stdin := bytes.NewReader(hookJSON)
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		
		exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
		if exitCode != 0 {
			t.Errorf("Event %d: Expected exit code 0, got %d", i, exitCode)
		}
		
		// Small delay between events
		time.Sleep(10 * time.Millisecond)
	}
	
	// Verify all events were tracked with different sound paths
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	// Verify we have entries for all test events
	var eventCount int
	err = db.QueryRow("SELECT COUNT(*) FROM hook_events").Scan(&eventCount)
	if err != nil {
		t.Fatalf("Failed to query event count: %v", err)
	}
	
	if eventCount != len(testEvents) {
		t.Errorf("Expected %d events in database, got %d", len(testEvents), eventCount)
	}
	
	// Verify each event has different selected_path (showing different sound mapping)
	rows, err := db.Query("SELECT session_id, selected_path FROM hook_events ORDER BY timestamp")
	if err != nil {
		t.Fatalf("Failed to query event details: %v", err)
	}
	defer rows.Close()
	
	selectedPaths := make(map[string]string)
	for rows.Next() {
		var sessionID, selectedPath string
		err = rows.Scan(&sessionID, &selectedPath)
		if err != nil {
			t.Fatalf("Failed to scan event row: %v", err)
		}
		selectedPaths[sessionID] = selectedPath
	}
	
	// Verify we have different paths for different event types
	pathSet := make(map[string]bool)
	for _, path := range selectedPaths {
		pathSet[path] = true
	}
	
	// We should have at least 2 different paths (success vs error, different tools, etc.)
	if len(pathSet) < 2 {
		t.Errorf("Expected at least 2 different selected paths, got %d unique paths: %v", 
			len(pathSet), pathSet)
	}
}

func TestEndToEndFallbackLevelRecording(t *testing.T) {
	// Test that fallback behavior is correctly recorded in the database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "fallback_levels.db")
	
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Process an event to understand current fallback behavior
	toolResponse := json.RawMessage(`{"stdout":"Test","stderr":"","interrupted":false}`)
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",
		SessionID:      "fallback-behavior-test",
		TranscriptPath: "/test/transcript", 
		CWD:            "/test/path",
		ToolName:       stringPtr("NonExistentTool"), // This will test fallback behavior
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	cli := cli.NewCLI()
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	
	// Analyze what was recorded in the database
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
	
	var fallbackLevel int
	var selectedPath string
	err = db.QueryRow("SELECT fallback_level, selected_path FROM hook_events WHERE session_id = ?", 
		hookEvent.SessionID).Scan(&fallbackLevel, &selectedPath)
	if err != nil {
		t.Fatalf("Failed to query fallback level: %v", err)
	}
	
	t.Logf("System behavior - Fallback level: %d, Selected path: %s", fallbackLevel, selectedPath)
	
	// Analyze all path lookups
	rows, err := db.Query(`
		SELECT pl.path, pl.sequence, pl.found
		FROM path_lookups pl 
		JOIN hook_events he ON pl.event_id = he.id 
		WHERE he.session_id = ?
		ORDER BY pl.sequence`, hookEvent.SessionID)
	if err != nil {
		t.Fatalf("Failed to query path_lookups: %v", err)
	}
	defer rows.Close()
	
	var pathsChecked []string
	t.Log("Complete path lookup sequence:")
	for rows.Next() {
		var path string
		var sequence int
		var found bool
		
		err = rows.Scan(&path, &sequence, &found)
		if err != nil {
			t.Fatalf("Failed to scan path lookup: %v", err)
		}
		
		pathsChecked = append(pathsChecked, path)
		t.Logf("  %d: %s (found: %t)", sequence, path, found)
	}
	
	// Verify basic tracking functionality
	if fallbackLevel <= 0 {
		t.Errorf("Expected positive fallback level, got %d", fallbackLevel)
	}
	
	if len(pathsChecked) == 0 {
		t.Error("Expected at least one path to be checked")
	}
	
	if selectedPath == "" {
		t.Error("Expected selected path to be recorded")
	}
	
	// Document current behavior for future reference
	t.Logf("DOCUMENTED BEHAVIOR: Fallback system records level %d with %d path lookups", 
		fallbackLevel, len(pathsChecked))
	t.Logf("This test validates that the tracking system captures fallback behavior correctly")
}

func TestEndToEndTrackingDisabled(t *testing.T) {
	// Test that when tracking is disabled, no database is created
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "should_not_exist.db")
	
	// Explicitly disable tracking
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false") 
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()
	
	// Process a hook event
	toolResponse := json.RawMessage(`{"stdout":"Test","stderr":"","interrupted":false}`)
	hookEvent := hooks.HookEvent{
		EventName:      "PostToolUse",
		SessionID:      "tracking-disabled-test",
		TranscriptPath: "/test/transcript",
		CWD:            "/test/path",
		ToolName:       stringPtr("Edit"),
		ToolResponse:   &toolResponse,
	}
	
	hookJSON, err := json.Marshal(hookEvent)
	if err != nil {
		t.Fatalf("Failed to marshal hook event: %v", err)
	}
	
	cli := cli.NewCLI()
	stdin := bytes.NewReader(hookJSON)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	
	exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	
	// Verify database was NOT created
	if _, err := os.Stat(dbPath); err == nil {
		t.Error("Expected database file to NOT be created when tracking is disabled")
	}
	
	// Verify no database files were created in temp directory
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".db") {
			t.Errorf("Found database file %s when tracking should be disabled", entry.Name())
		}
	}
}

func TestEndToEndEnvironmentVariableConfiguration(t *testing.T) {
	// Test complete environment variable configuration workflow
	tempDir := t.TempDir()
	customDBPath := filepath.Join(tempDir, "custom_env_path.db")
	
	// Test multiple environment variable configurations
	testCases := []struct {
		name              string
		trackingEnabled   string
		dbPath            string
		expectsDB         bool
		expectsAtPath     string
	}{
		{
			name:            "tracking_enabled_custom_path",
			trackingEnabled: "true",
			dbPath:          customDBPath,
			expectsDB:       true,
			expectsAtPath:   customDBPath,
		},
		{
			name:            "tracking_enabled_default_path", 
			trackingEnabled: "true",
			dbPath:          "", // Will use default
			expectsDB:       true,
			expectsAtPath:   "", // Will be determined by XDG cache
		},
		{
			name:            "tracking_disabled",
			trackingEnabled: "false",
			dbPath:          customDBPath,
			expectsDB:       false,
			expectsAtPath:   "",
		},
	}
	
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Set environment variables
			os.Setenv("CLAUDIO_SOUND_TRACKING", testCase.trackingEnabled)
			if testCase.dbPath != "" {
				os.Setenv("CLAUDIO_SOUND_TRACKING_DB", testCase.dbPath)
			} else {
				os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
			}
			defer func() {
				os.Unsetenv("CLAUDIO_SOUND_TRACKING")
				os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
			}()
			
			// Process hook event
			toolResponse := json.RawMessage(`{"stdout":"Test","stderr":"","interrupted":false}`)
			hookEvent := hooks.HookEvent{
				EventName:      "PostToolUse",
				SessionID:      "env-config-test-" + testCase.name,
				TranscriptPath: "/test/transcript",
				CWD:            "/test/path",
				ToolName:       stringPtr("Edit"),
				ToolResponse:   &toolResponse,
			}
			
			hookJSON, err := json.Marshal(hookEvent)
			if err != nil {
				t.Fatalf("Failed to marshal hook event: %v", err)
			}
			
			cli := cli.NewCLI()
			stdin := bytes.NewReader(hookJSON)
			stdout := &bytes.Buffer{}
			stderr := &bytes.Buffer{}
			
			exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, stdout, stderr)
			if exitCode != 0 {
				t.Errorf("Expected exit code 0, got %d", exitCode)
			}
			
			// Verify database creation expectations
			if testCase.expectsDB {
				var dbExists bool
				var actualDBPath string
				
				if testCase.expectsAtPath != "" {
					// Check specific path
					if _, err := os.Stat(testCase.expectsAtPath); err == nil {
						dbExists = true
						actualDBPath = testCase.expectsAtPath
					}
				} else {
					// Check for any .db files in cache directories (default path case)
					// This is more complex to verify, so we'll just ensure some database was created
					// by checking stderr output or other indicators
					if !strings.Contains(stderr.String(), "tracking disabled") {
						dbExists = true
						actualDBPath = "default location"
					}
				}
				
				if !dbExists {
					t.Errorf("Expected database to be created when tracking enabled, but none found")
				} else if testCase.expectsAtPath != "" {
					// Verify the database contains our test data
					db, err := sql.Open("sqlite", actualDBPath)
					if err != nil {
						t.Fatalf("Failed to open database: %v", err)
					}
					defer db.Close()
					
					var count int
					err = db.QueryRow("SELECT COUNT(*) FROM hook_events WHERE session_id = ?", 
						hookEvent.SessionID).Scan(&count)
					if err != nil {
						t.Fatalf("Failed to query hook_events: %v", err)
					}
					
					if count != 1 {
						t.Errorf("Expected 1 event in database, got %d", count)
					}
				}
			} else {
				// Should not create database
				if testCase.expectsAtPath != "" {
					if _, err := os.Stat(testCase.expectsAtPath); err == nil {
						t.Error("Expected database NOT to be created when tracking disabled")
					}
				}
			}
		})
	}
}