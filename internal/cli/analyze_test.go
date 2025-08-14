package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/tracking"
)

// TDD Step 3 GREEN: Data structures and functions moved to analyze_command.go

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
		// Insert hook event with proper context JSON for tool extraction
		contextJSON := `{"Category":1,"ToolName":"` + data.toolName + `","IsSuccess":true}`
		if data.toolName == "Bash" {
			// For Bash tools, use git as the extracted tool name
			contextJSON = `{"Category":1,"ToolName":"git","OriginalTool":"Bash","IsSuccess":true}`
		}
		eventResult, err := db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			now-int64(i*60), // Different timestamps
			data.sessionID,
			data.toolName,
			data.selectedPath,
			5, // High fallback level indicates missing sounds
			contextJSON)
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
	
	// TDD Step 3 GREEN: Update expectations for hierarchical tool-first output format with corrected tool names
	expectedHierarchicalContent := []string{
		"Missing Sounds by Tool", // New hierarchical header
		"Edit", // Tool names should appear as headers (from context ToolName)
		"git",  // Tool names should appear as headers (extracted from Bash context ToolName)
		"success/edit-success.wav", // Should appear under Edit tool
		"success/bash-success.wav", // Should appear under git tool (extracted from Bash)
		"success/git-push.wav",     // Should appear under git tool (extracted from Bash)
		"success/tool-complete.wav", // Should appear under Edit tool
		"requests", // Should show request counts
		"total:", // Should show tool totals
	}

	for _, content := range expectedHierarchicalContent {
		if !strings.Contains(output, content) {
			t.Errorf("Expected hierarchical output to contain '%s', got: %s", content, output)
		}
	}

	// TDD Step 3 GREEN: Verify tools are ordered by total requests (Edit=3, git=2)
	editToolIndex := strings.Index(output, "Edit (total:")   // Tool header with total
	gitToolIndex := strings.Index(output, "git (total:")     // Tool header with total
	
	if editToolIndex == -1 || gitToolIndex == -1 {
		t.Fatal("Expected both Edit and git tool headers in hierarchical output")
	}

	// Edit (3 total requests) should appear before git (2 total requests) 
	if editToolIndex > gitToolIndex {
		t.Error("Expected tools to be ordered by total requests (Edit=3 should come before git=2)")
	}

	// TDD Step 3 GREEN: Verify hierarchical indentation for sounds under tools  
	if !strings.Contains(output, "    success/edit-success.wav") {
		t.Error("Expected sounds to be indented under their tool section (4 spaces)")
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

// TDD RED: Test for grouping missing sounds by tool
func TestGroupMissingSoundsByTool(t *testing.T) {
	// Create test data with Category and ToolName populated
	missingSounds := []tracking.MissingSound{
		{
			Path:         "success/git-commit-success.wav",
			RequestCount: 50,
			Tools:        []string{"git"},
			Category:     "success",
			ToolName:     "git",
		},
		{
			Path:         "loading/git-start.wav",
			RequestCount: 30,
			Tools:        []string{"git"},
			Category:     "loading", 
			ToolName:     "git",
		},
		{
			Path:         "success/npm-success.wav",
			RequestCount: 25,
			Tools:        []string{"npm"},
			Category:     "success",
			ToolName:     "npm",
		},
		{
			Path:         "error/bash-error.wav",
			RequestCount: 15,
			Tools:        []string{"bash"},
			Category:     "error",
			ToolName:     "bash",
		},
		{
			Path:         "interactive/notification.wav",
			RequestCount: 10,
			Tools:        []string{"Edit", "Read"},
			Category:     "interactive",
			ToolName:     "", // No specific tool
		},
	}

	// TDD RED: Call grouping function that doesn't exist yet
	analysis := groupByTool(missingSounds)

	// Verify Analysis structure exists
	if len(analysis.Tools) == 0 {
		t.Error("Expected analysis.Tools to contain grouped tools")
	}

	// Verify git tool group (should have highest total)
	var gitTool *ToolGroup
	for i := range analysis.Tools {
		if analysis.Tools[i].Name == "git" {
			gitTool = &analysis.Tools[i]
			break
		}
	}

	if gitTool == nil {
		t.Fatal("Expected to find git tool group")
	}

	// Git should have 80 total requests (50 + 30)
	if gitTool.Total != 80 {
		t.Errorf("Expected git tool total to be 80, got %d", gitTool.Total)
	}

	// Git should have 2 missing sounds
	if gitTool.Count != 2 {
		t.Errorf("Expected git tool count to be 2, got %d", gitTool.Count)
	}

	// Git should have success and loading categories
	expectedCategories := map[string]bool{
		"success": false,
		"loading": false,
	}

	for _, category := range gitTool.Categories {
		if _, exists := expectedCategories[category.Name]; exists {
			expectedCategories[category.Name] = true
		}
	}

	for catName, found := range expectedCategories {
		if !found {
			t.Errorf("Expected git tool to have %s category", catName)
		}
	}

	// Verify tools are sorted by total requests (git=80, npm=25, bash=15)
	if len(analysis.Tools) < 3 {
		t.Fatal("Expected at least 3 tool groups")
	}

	if analysis.Tools[0].Name != "git" {
		t.Errorf("Expected first tool to be 'git', got '%s'", analysis.Tools[0].Name)
	}

	if analysis.Tools[1].Name != "npm" {
		t.Errorf("Expected second tool to be 'npm', got '%s'", analysis.Tools[1].Name)
	}

	if analysis.Tools[2].Name != "bash" {
		t.Errorf("Expected third tool to be 'bash', got '%s'", analysis.Tools[2].Name)
	}

	// Verify Other category contains non-tool sounds
	if len(analysis.Other) == 0 {
		t.Error("Expected analysis.Other to contain non-tool sounds")
	}

	// Interactive category should be in Other
	var interactiveOther *CategoryGroup
	for i := range analysis.Other {
		if analysis.Other[i].Name == "interactive" {
			interactiveOther = &analysis.Other[i]
			break
		}
	}

	if interactiveOther == nil {
		t.Error("Expected interactive category in Other section")
	}
}