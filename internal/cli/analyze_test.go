package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ctoth/claudio/internal/tracking"
)

// TDD RED: Test analyze missing command functionality

func TestAnalyzeMissingCommand(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "analyze_test.db")

	// Set up test database with sample missing sound data
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data - simulate hook events with missing sounds
	now := time.Now().Unix()
	testData := []struct {
		sessionID    string
		toolName     string
		selectedPath string
		missingPaths []string
	}{
		{
			sessionID:    "session-1",
			toolName:     "Edit",  
			selectedPath: "default.wav",
			missingPaths: []string{"success/edit-success.wav", "success/tool-complete.wav"},
		},
		{
			sessionID:    "session-2", 
			toolName:     "Bash",
			selectedPath: "default.wav",
			missingPaths: []string{"success/bash-success.wav", "success/git-push.wav"},
		},
		{
			sessionID:    "session-3",
			toolName:     "Edit",
			selectedPath: "default.wav", 
			missingPaths: []string{"success/edit-success.wav"}, // Duplicate - should aggregate
		},
	}

	// Insert test hook events and path lookups
	for i, data := range testData {
		// Insert hook event
		eventResult, err := db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			now-int64(i*60), // Different timestamps
			data.sessionID,
			data.toolName,
			data.selectedPath,
			5, // High fallback level indicates missing sounds
			`{"Category":1,"ToolName":"`+data.toolName+`","IsSuccess":true}`)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}

		eventID, _ := eventResult.LastInsertId()

		// Insert missing path lookups
		for j, path := range data.missingPaths {
			_, err = db.Exec(`
				INSERT INTO path_lookups (event_id, path, sequence, found)
				VALUES (?, ?, ?, ?)`,
				eventID, path, j+1, 0) // found=0 means missing
			if err != nil {
				t.Fatalf("Failed to insert test path lookup: %v", err)
			}
		}
	}

	// Set environment to use our test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test the analyze missing command
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze missing
	exitCode := cli.Run([]string{"claudio", "analyze", "missing"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()
	
	// Verify output contains expected missing sounds
	expectedContent := []string{
		"Missing Sounds",
		"success/edit-success.wav", // Should appear (2 requests)
		"success/bash-success.wav", // Should appear (1 request)
		"success/git-push.wav",     // Should appear (1 request)
		"success/tool-complete.wav", // Should appear (1 request)
		"requests", // Should show request counts
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Expected output to contain '%s', got: %s", content, output)
		}
	}

	// Verify sounds are ordered by frequency (most requested first)
	editIndex := strings.Index(output, "success/edit-success.wav")
	bashIndex := strings.Index(output, "success/bash-success.wav")
	
	if editIndex == -1 || bashIndex == -1 {
		t.Fatal("Expected both edit and bash sounds in output")
	}

	// edit-success.wav (2 requests) should appear before bash-success.wav (1 request)
	if editIndex > bashIndex {
		t.Error("Expected sounds to be ordered by frequency (most requested first)")
	}
}

func TestAnalyzeMissingCommandWithFilters(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "analyze_filter_test.db")

	// Set up test database
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with different tools and timestamps
	now := time.Now().Unix()
	oneWeekAgo := now - (7 * 24 * 60 * 60)
	twoWeeksAgo := now - (14 * 24 * 60 * 60)

	testEvents := []struct {
		timestamp   int64
		toolName    string
		missingPath string
	}{
		{now, "Edit", "success/edit-success.wav"},
		{oneWeekAgo, "Bash", "success/bash-success.wav"},
		{twoWeeksAgo, "Git", "success/git-push.wav"}, // Should be filtered out by --days 7
	}

	for i, event := range testEvents {
		// Insert hook event
		eventResult, err := db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			event.timestamp,
			"session-"+string(rune(i+1)),
			event.toolName,
			"default.wav",
			5,
			`{"Category":1,"ToolName":"`+event.toolName+`"}`)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}

		eventID, _ := eventResult.LastInsertId()

		// Insert missing path lookup
		_, err = db.Exec(`
			INSERT INTO path_lookups (event_id, path, sequence, found)
			VALUES (?, ?, ?, ?)`,
			eventID, event.missingPath, 1, 0)
		if err != nil {
			t.Fatalf("Failed to insert test path lookup: %v", err)
		}
	}

	// Set environment to use our test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test with --days filter
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze missing --days 7
	exitCode := cli.Run([]string{"claudio", "analyze", "missing", "--days", "7"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Should include recent sounds
	if !strings.Contains(output, "success/edit-success.wav") {
		t.Error("Expected output to include recent edit sound")
	}
	if !strings.Contains(output, "success/bash-success.wav") {
		t.Error("Expected output to include week-old bash sound")
	}

	// Should NOT include old sounds (older than 7 days)
	if strings.Contains(output, "success/git-push.wav") {
		t.Error("Expected output to exclude sounds older than 7 days")
	}
}

func TestAnalyzeMissingCommandWithToolFilter(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "analyze_tool_test.db")

	// Set up test database
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with different tools
	now := time.Now().Unix()
	testEvents := []struct {
		toolName    string
		missingPath string
	}{
		{"Edit", "success/edit-success.wav"},
		{"Bash", "success/bash-success.wav"},
		{"Read", "success/read-success.wav"},
	}

	for i, event := range testEvents {
		// Insert hook event
		eventResult, err := db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			now,
			"session-"+string(rune(i+1)),
			event.toolName,
			"default.wav",
			5,
			`{"Category":1,"ToolName":"`+event.toolName+`"}`)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}

		eventID, _ := eventResult.LastInsertId()

		// Insert missing path lookup
		_, err = db.Exec(`
			INSERT INTO path_lookups (event_id, path, sequence, found)
			VALUES (?, ?, ?, ?)`,
			eventID, event.missingPath, 1, 0)
		if err != nil {
			t.Fatalf("Failed to insert test path lookup: %v", err)
		}
	}

	// Set environment to use our test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test with --tool filter
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze missing --tool Edit
	exitCode := cli.Run([]string{"claudio", "analyze", "missing", "--tool", "Edit"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Should include Edit sounds only
	if !strings.Contains(output, "success/edit-success.wav") {
		t.Error("Expected output to include Edit sound")
	}

	// Should NOT include other tools
	if strings.Contains(output, "success/bash-success.wav") {
		t.Error("Expected output to exclude Bash sound when filtering by Edit")
	}
	if strings.Contains(output, "success/read-success.wav") {
		t.Error("Expected output to exclude Read sound when filtering by Edit")
	}
}

func TestAnalyzeMissingCommandNoDatabase(t *testing.T) {
	// Test behavior when tracking is disabled or database doesn't exist
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze missing
	exitCode := cli.Run([]string{"claudio", "analyze", "missing"}, stdin, stdout, stderr)
	
	// Should handle gracefully - either exit with error message or show empty results
	if exitCode != 0 && exitCode != 1 {
		t.Errorf("Expected exit code 0 or 1 when tracking disabled, got %d", exitCode)
	}

	// Should contain helpful message
	output := stdout.String() + stderr.String()
	if !strings.Contains(output, "tracking") && !strings.Contains(output, "database") && !strings.Contains(output, "disabled") {
		t.Error("Expected helpful message when tracking is disabled")
	}
}

func TestAnalyzeMissingCommandHelp(t *testing.T) {
	// Test help output for analyze missing command
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze missing --help
	exitCode := cli.Run([]string{"claudio", "analyze", "missing", "--help"}, stdin, stdout, stderr)
	
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help command, got %d", exitCode)
	}

	helpOutput := stdout.String()

	// Should contain command description and flags
	expectedHelpContent := []string{
		"missing",
		"--days",
		"--tool", 
		"--limit",
		"sounds",
		"requested",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}
}