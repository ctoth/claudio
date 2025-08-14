package cli

import (
	"bytes"
	"fmt"
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

// TDD RED: Test analyze usage command functionality

func TestAnalyzeUsageCommand(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "usage_test.db")

	// Set up test database with sample usage data
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data - simulate actual sound playback events
	now := time.Now().Unix()
	testData := []struct {
		timestamp     int64
		sessionID     string
		toolName      string
		soundPath     string
		fallbackLevel int
		contextJSON   string
	}{
		{
			timestamp:     now,
			sessionID:     "session-1",
			toolName:      "Edit", 
			soundPath:     "success/edit-success.wav",
			fallbackLevel: 1, // Exact match
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
		},
		{
			timestamp:     now - 1800,
			sessionID:     "session-1",
			toolName:      "Edit",
			soundPath:     "success/edit-success.wav", 
			fallbackLevel: 1,
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
		},
		{
			timestamp:     now - 3600,
			sessionID:     "session-2",
			toolName:      "Bash",
			soundPath:     "loading/bash-thinking.wav",
			fallbackLevel: 2, // Tool-specific
			contextJSON:   `{"Category":0,"ToolName":"Bash","IsLoading":true}`,
		},
		{
			timestamp:     now - 7200, 
			sessionID:     "session-3",
			toolName:      "Read",
			soundPath:     "default.wav",
			fallbackLevel: 5, // Complete fallback
			contextJSON:   `{"Category":2,"ToolName":"Read","HasError":true}`,
		},
	}

	for _, data := range testData {
		// Insert hook event
		_, err = db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			data.timestamp, data.sessionID, data.toolName, data.soundPath, data.fallbackLevel, data.contextJSON)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}
	}

	// Set environment to use our test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test the analyze usage command
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage
	exitCode := cli.Run([]string{"claudio", "analyze", "usage"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Verify output contains expected elements
	expectedContent := []string{
		"Sound Usage Statistics",
		"Most Frequently Used Sounds:",
		"success/edit-success.wav", // Should appear with count 2
		"loading/bash-thinking.wav", // Should appear with count 1
		"default.wav",              // Should appear with count 1
		"2 times",                  // Edit sound played twice
		"exact match",              // Fallback description
		"tool-specific",            // Fallback description  
		"default fallback",         // Fallback description
		"To improve your sound coverage:", // Footer advice
	}

	for _, content := range expectedContent {
		if !strings.Contains(output, content) {
			t.Errorf("Expected output to contain '%s', got: %s", content, output)
		}
	}

	// Verify ordering (edit-success should be first with 2 times)
	editIndex := strings.Index(output, "success/edit-success.wav")
	bashIndex := strings.Index(output, "loading/bash-thinking.wav")
	defaultIndex := strings.Index(output, "default.wav")

	if editIndex == -1 || bashIndex == -1 || defaultIndex == -1 {
		t.Fatal("Expected all sounds to appear in output")
	}

	// Edit should appear first (highest count)
	if editIndex > bashIndex || editIndex > defaultIndex {
		t.Error("Expected success/edit-success.wav to appear first (highest frequency)")
	}
}

func TestAnalyzeUsageCommandWithFilters(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "usage_filter_test.db")

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
		timestamp int64
		toolName  string
		soundPath string
		category  int
	}{
		{now, "Edit", "success/edit-success.wav", 1},         // Recent, should be included
		{oneWeekAgo, "Bash", "success/bash-success.wav", 1}, // Week old, should be included with --days 7
		{twoWeeksAgo, "Read", "success/read-success.wav", 1}, // Too old, should be filtered out
		{now, "Edit", "error/edit-error.wav", 2},            // Recent but different category
	}

	for i, event := range testEvents {
		contextJSON := fmt.Sprintf(`{"Category":%d,"ToolName":"%s"}`, event.category, event.toolName)
		_, err = db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			event.timestamp, fmt.Sprintf("session-%d", i), event.toolName, event.soundPath, 1, contextJSON)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
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

	// Run: claudio analyze usage --days 7 --tool Edit --category success
	exitCode := cli.Run([]string{"claudio", "analyze", "usage", "--days", "7", "--tool", "Edit", "--category", "success"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Should include recent Edit success sound
	if !strings.Contains(output, "success/edit-success.wav") {
		t.Error("Expected output to include recent Edit success sound")
	}

	// Should show filter information
	if !strings.Contains(output, "Tool Filter: Edit") {
		t.Error("Expected output to show tool filter")
	}
	if !strings.Contains(output, "Category Filter: success") {
		t.Error("Expected output to show category filter")
	}

	// Should NOT include other sounds due to filters
	if strings.Contains(output, "success/bash-success.wav") {
		t.Error("Expected output to exclude Bash sound due to tool filter")
	}
	if strings.Contains(output, "success/read-success.wav") {
		t.Error("Expected output to exclude Read sound due to time filter")
	}
	if strings.Contains(output, "error/edit-error.wav") {
		t.Error("Expected output to exclude error sound due to category filter")
	}
}

func TestAnalyzeUsageCommandWithSummaryAndFallbacks(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "usage_summary_test.db")

	// Set up test database
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with various fallback levels
	now := time.Now().Unix()
	testEvents := []struct {
		soundPath     string
		fallbackLevel int
		count         int
	}{
		{"success/exact-match.wav", 1, 10},       // Exact matches
		{"success/tool-specific.wav", 2, 5},      // Tool-specific
		{"default.wav", 5, 2},                    // Default fallback
	}

	for _, event := range testEvents {
		for i := 0; i < event.count; i++ {
			contextJSON := `{"Category":1,"ToolName":"Test"}`
			_, err = db.Exec(`
				INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
				VALUES (?, ?, ?, ?, ?, ?)`,
				now-int64(i*60), "session-test", "Test", event.soundPath, event.fallbackLevel, contextJSON)
			if err != nil {
				t.Fatalf("Failed to insert test event: %v", err)
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

	// Test with --show-summary and --show-fallbacks
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage --show-summary --show-fallbacks
	exitCode := cli.Run([]string{"claudio", "analyze", "usage", "--show-summary", "--show-fallbacks"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Should include summary statistics
	if !strings.Contains(output, "Summary:") {
		t.Error("Expected output to include summary statistics")
	}
	if !strings.Contains(output, "17 total events") { // 10+5+2=17
		t.Error("Expected summary to show correct total events")
	}
	if !strings.Contains(output, "3 unique sounds") {
		t.Error("Expected summary to show correct unique sounds count")
	}

	// Should include fallback statistics
	if !strings.Contains(output, "Fallback Level Statistics:") {
		t.Error("Expected output to include fallback statistics")
	}
	if !strings.Contains(output, "Level 1:") {
		t.Error("Expected fallback stats to include Level 1")
	}
	if !strings.Contains(output, "Exact hint match") {
		t.Error("Expected fallback stats to include level descriptions")
	}
}

func TestAnalyzeUsageCommandWithPresets(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "usage_preset_test.db")

	// Set up test database
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with different timestamps
	now := time.Now()
	testEvents := []struct {
		timestamp time.Time
		soundPath string
	}{
		{now, "today.wav"},                          // Today
		{now.AddDate(0, 0, -1), "yesterday.wav"},    // Yesterday
		{now.AddDate(0, 0, -8), "last-week.wav"},    // Over a week ago
	}

	for i, event := range testEvents {
		contextJSON := `{"Category":1,"ToolName":"Test"}`
		_, err = db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			event.timestamp.Unix(), fmt.Sprintf("session-%d", i), "Test", event.soundPath, 1, contextJSON)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}
	}

	// Set environment to use our test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test with --preset today
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage --preset today
	exitCode := cli.Run([]string{"claudio", "analyze", "usage", "--preset", "today"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
		t.Logf("Stderr: %s", stderr.String())
	}

	output := stdout.String()

	// Should show preset information
	if !strings.Contains(output, "Time Range: today") {
		t.Error("Expected output to show today preset")
	}

	// Should include today's sound
	if !strings.Contains(output, "today.wav") {
		t.Error("Expected output to include today's sound")
	}

	// Should NOT include older sounds
	if strings.Contains(output, "yesterday.wav") {
		t.Error("Expected output to exclude yesterday's sound with today preset")
	}
	if strings.Contains(output, "last-week.wav") {
		t.Error("Expected output to exclude last week's sound with today preset")
	}
}

func TestAnalyzeUsageCommandNoDatabase(t *testing.T) {
	// Test behavior when tracking is disabled
	os.Setenv("CLAUDIO_SOUND_TRACKING", "false")
	defer os.Unsetenv("CLAUDIO_SOUND_TRACKING")

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage
	exitCode := cli.Run([]string{"claudio", "analyze", "usage"}, stdin, stdout, stderr)

	// Should handle gracefully
	if exitCode != 0 {
		t.Errorf("Expected exit code 0 when tracking disabled, got %d", exitCode)
	}

	// Should contain helpful message
	output := stdout.String()
	if !strings.Contains(output, "Sound tracking is not enabled") {
		t.Error("Expected helpful message when tracking is disabled")
	}
	if !strings.Contains(output, "CLAUDIO_SOUND_TRACKING=true") {
		t.Error("Expected instruction to enable tracking")
	}
}

func TestAnalyzeUsageCommandNoData(t *testing.T) {
	// Create empty database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "empty_test.db")

	// Set up empty test database
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Set environment to use our empty test database
	os.Setenv("CLAUDIO_SOUND_TRACKING", "true")
	os.Setenv("CLAUDIO_SOUND_TRACKING_DB", dbPath)
	defer func() {
		os.Unsetenv("CLAUDIO_SOUND_TRACKING")
		os.Unsetenv("CLAUDIO_SOUND_TRACKING_DB")
	}()

	// Test the analyze usage command with no data
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage --tool Edit
	exitCode := cli.Run([]string{"claudio", "analyze", "usage", "--tool", "Edit"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for empty database, got %d", exitCode)
	}

	output := stdout.String()

	// Should contain helpful message about no data
	if !strings.Contains(output, "No sound usage data found") {
		t.Error("Expected message about no data found")
	}

	// Should provide helpful suggestions 
	if !strings.Contains(output, "Try removing the --tool filter") {
		t.Error("Expected suggestion to remove tool filter")
	}
}

func TestAnalyzeUsageCommandHelp(t *testing.T) {
	// Test help output for analyze usage command
	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	// Run: claudio analyze usage --help
	exitCode := cli.Run([]string{"claudio", "analyze", "usage", "--help"}, stdin, stdout, stderr)

	if exitCode != 0 {
		t.Errorf("Expected exit code 0 for help command, got %d", exitCode)
	}

	helpOutput := stdout.String()

	// Should contain command description and flags
	expectedHelpContent := []string{
		"usage",
		"Show actual sound usage patterns",
		"--days",
		"--tool",
		"--category",
		"--preset",
		"--show-fallbacks",
		"--show-summary",
		"--limit",
		"Examples:",
		"claudio analyze usage --days 30",
		"claudio analyze usage --preset today",
		"fallback levels",
		"optimization opportunities",
	}

	for _, content := range expectedHelpContent {
		if !strings.Contains(helpOutput, content) {
			t.Errorf("Help output should contain '%s'", content)
		}
	}
}