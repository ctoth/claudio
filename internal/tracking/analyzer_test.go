package tracking

import (
	"path/filepath"
	"testing"
	"time"
)

// TDD RED: Test for GetMissingSounds with context extraction
func TestGetMissingSoundsWithContext(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "analyzer_context_test.db")

	// Set up test database with sample missing sound data
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with context JSON containing Category and ToolName
	now := time.Now().Unix()
	testData := []struct {
		sessionID    string
		toolName     string
		selectedPath string
		contextJSON  string // JSON context with Category and ToolName
		missingPaths []string
	}{
		{
			sessionID:    "session-1",
			toolName:     "Edit",
			selectedPath: "default.wav",
			contextJSON:  `{"Category":1,"ToolName":"Edit","IsSuccess":true}`, // Success = 1
			missingPaths: []string{"success/edit-success.wav"},
		},
		{
			sessionID:    "session-2",
			toolName:     "Bash",
			selectedPath: "default.wav",
			contextJSON:  `{"Category":0,"ToolName":"git","OriginalTool":"Bash","IsSuccess":false}`, // Loading = 0
			missingPaths: []string{"loading/git-start.wav"},
		},
		{
			sessionID:    "session-3",
			toolName:     "npm",
			selectedPath: "default.wav",
			contextJSON:  `{"Category":2,"ToolName":"npm","HasError":true}`, // Error = 2
			missingPaths: []string{"error/npm-error.wav"},
		},
	}

	// Insert test hook events and path lookups
	for i, data := range testData {
		// Insert hook event with context JSON
		eventResult, err := db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			now-int64(i*60), // Different timestamps
			data.sessionID,
			data.toolName,
			data.selectedPath,
			5, // High fallback level indicates missing sounds
			data.contextJSON)
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

	// Test the GetMissingSounds function with context extraction
	filter := QueryFilter{
		Days:  0,    // All time
		Tool:  "",   // All tools
		Limit: 20,   // Limit results
	}

	missingSounds, err := GetMissingSounds(db, filter)
	if err != nil {
		t.Fatalf("GetMissingSounds failed: %v", err)
	}

	if len(missingSounds) == 0 {
		t.Fatal("Expected missing sounds, got none")
	}

	// TDD RED: Test expects Category and ToolName fields that don't exist yet
	for _, sound := range missingSounds {
		// These fields should be populated from context JSON
		if sound.Category == "" {
			t.Errorf("Expected sound.Category to be populated from context, got empty string")
		}
		if sound.ToolName == "" {
			t.Errorf("Expected sound.ToolName to be populated from context, got empty string")
		}

		// Verify specific category mappings
		switch sound.Path {
		case "success/edit-success.wav":
			if sound.Category != "success" {
				t.Errorf("Expected Category 'success' for %s, got '%s'", sound.Path, sound.Category)
			}
			if sound.ToolName != "Edit" {
				t.Errorf("Expected ToolName 'Edit' for %s, got '%s'", sound.Path, sound.ToolName)
			}
		case "loading/git-start.wav":
			if sound.Category != "loading" {
				t.Errorf("Expected Category 'loading' for %s, got '%s'", sound.Path, sound.Category)
			}
			if sound.ToolName != "git" {
				t.Errorf("Expected ToolName 'git' for %s, got '%s'", sound.Path, sound.ToolName)
			}
		case "error/npm-error.wav":
			if sound.Category != "error" {
				t.Errorf("Expected Category 'error' for %s, got '%s'", sound.Path, sound.Category)
			}
			if sound.ToolName != "npm" {
				t.Errorf("Expected ToolName 'npm' for %s, got '%s'", sound.Path, sound.ToolName)
			}
		}
	}

	// Verify the existing fields still work
	for _, sound := range missingSounds {
		if sound.Path == "" {
			t.Error("Expected sound.Path to be populated")
		}
		if sound.RequestCount <= 0 {
			t.Error("Expected sound.RequestCount to be > 0")
		}
		// Tools field should still work
		if len(sound.Tools) == 0 {
			t.Error("Expected sound.Tools to be populated")
		}
	}
}

// TDD RED: Test GetSoundUsage function that doesn't exist yet
func TestGetSoundUsage(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "usage_test.db")

	// Set up test database with sample usage data
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with successful sound playback events
	now := time.Now().Unix()
	oneWeekAgo := now - (7 * 24 * 60 * 60)
	testEvents := []struct {
		timestamp   int64
		sessionID   string
		toolName    string
		soundPath   string
		contextJSON string
		fallbackLevel int
	}{
		{
			timestamp:     now,
			sessionID:     "session-1",
			toolName:      "Edit",
			soundPath:     "success/edit-success.wav",
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
			fallbackLevel: 1, // Low fallback = successful sound found
		},
		{
			timestamp:     now - 1800, // 30 minutes ago
			sessionID:     "session-1", 
			toolName:      "Edit",
			soundPath:     "success/edit-success.wav",
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
			fallbackLevel: 1,
		},
		{
			timestamp:     now - 3600, // 1 hour ago
			sessionID:     "session-2",
			toolName:      "Bash",
			soundPath:     "loading/bash-thinking.wav",
			contextJSON:   `{"Category":0,"ToolName":"Bash","IsLoading":true}`,
			fallbackLevel: 2, // Medium fallback
		},
		{
			timestamp:     oneWeekAgo,
			sessionID:     "session-old",
			toolName:      "Git",
			soundPath:     "success/git-push.wav", 
			contextJSON:   `{"Category":1,"ToolName":"Git","IsSuccess":true}`,
			fallbackLevel: 1,
		},
	}

	for _, event := range testEvents {
		// Insert hook event
		_, err = db.Exec(`
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, ?, ?, ?, ?, ?)`,
			event.timestamp, event.sessionID, event.toolName, event.soundPath, event.fallbackLevel, event.contextJSON)
		if err != nil {
			t.Fatalf("Failed to insert test event: %v", err)
		}
	}

	// TDD RED: Test GetSoundUsage function that doesn't exist yet
	filter := QueryFilter{
		Days: 1, // Last 24 hours - should exclude week-old event
		Limit: 10,
		OrderBy: "frequency",
		OrderDesc: true,
	}

	usage, err := GetSoundUsage(db, filter)
	if err != nil {
		t.Fatalf("GetSoundUsage failed: %v", err)
	}

	// Should have 2 unique sounds (edit-success appears twice, bash-thinking once)
	if len(usage) != 2 {
		t.Errorf("Expected 2 unique sounds, got %d", len(usage))
	}

	// Check first result (edit-success with highest frequency)
	if usage[0].Path != "success/edit-success.wav" {
		t.Errorf("Expected first result to be edit-success.wav, got %s", usage[0].Path)
	}
	if usage[0].PlayCount != 2 {
		t.Errorf("Expected edit-success.wav to have PlayCount 2, got %d", usage[0].PlayCount)
	}
	if usage[0].Category != "success" {
		t.Errorf("Expected edit-success.wav to have Category 'success', got '%s'", usage[0].Category)
	}
	if usage[0].ToolName != "Edit" {
		t.Errorf("Expected edit-success.wav to have ToolName 'Edit', got '%s'", usage[0].ToolName)
	}

	// Check second result (bash-thinking with lower frequency)
	if usage[1].Path != "loading/bash-thinking.wav" {
		t.Errorf("Expected second result to be bash-thinking.wav, got %s", usage[1].Path)
	}
	if usage[1].PlayCount != 1 {
		t.Errorf("Expected bash-thinking.wav to have PlayCount 1, got %d", usage[1].PlayCount)
	}
}

// TDD RED: Test GetUsageSummary function that doesn't exist yet
func TestGetUsageSummary(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "summary_test.db")

	// Set up test database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with mixed usage patterns
	now := time.Now().Unix()
	testEvents := []struct {
		toolName    string
		soundPath   string
		contextJSON string
		fallbackLevel int
		count       int // How many times to insert this event
	}{
		{
			toolName:      "Edit",
			soundPath:     "success/edit-success.wav",
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
			fallbackLevel: 1, // Perfect match
			count:         10,
		},
		{
			toolName:      "Bash",
			soundPath:     "success/tool-complete.wav", 
			contextJSON:   `{"Category":1,"ToolName":"Bash","IsSuccess":true}`,
			fallbackLevel: 4, // High fallback to generic sound
			count:         5,
		},
		{
			toolName:      "Read",
			soundPath:     "default.wav",
			contextJSON:   `{"Category":2,"ToolName":"Read","HasError":true}`,
			fallbackLevel: 5, // Complete fallback to default
			count:         3,
		},
	}

	for _, event := range testEvents {
		for i := 0; i < event.count; i++ {
			_, err = db.Exec(`
				INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
				VALUES (?, ?, ?, ?, ?, ?)`,
				now-int64(i*60), "test-session", event.toolName, event.soundPath, event.fallbackLevel, event.contextJSON)
			if err != nil {
				t.Fatalf("Failed to insert test event: %v", err)
			}
		}
	}

	// TDD RED: Test GetUsageSummary function that doesn't exist yet
	filter := QueryFilter{
		Days: 0, // All time
		Limit: 0, // No limit
	}

	summary, err := GetUsageSummary(db, filter)
	if err != nil {
		t.Fatalf("GetUsageSummary failed: %v", err)
	}

	// Check total events
	expectedTotal := 10 + 5 + 3 // 18 total events
	if summary.TotalEvents != expectedTotal {
		t.Errorf("Expected TotalEvents %d, got %d", expectedTotal, summary.TotalEvents)
	}

	// Check unique sounds (3 different sound files)
	if summary.UniqueSounds != 3 {
		t.Errorf("Expected UniqueSounds 3, got %d", summary.UniqueSounds)
	}

	// fallback_level aggregations (AvgFallbackLevel / FallbackDistribution)
	// were removed in schema v2 because the per-row value conflated chain
	// shapes. See review finding #20.
}

// TDD RED: Test GetToolUsageStats function that doesn't exist yet
func TestGetToolUsageStats(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "tool_stats_test.db")

	// Set up test database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with different tools
	now := time.Now().Unix()
	testEvents := []struct {
		toolName      string
		contextJSON   string
		fallbackLevel int
		count         int
	}{
		{
			toolName:      "Edit",
			contextJSON:   `{"Category":1,"ToolName":"Edit","IsSuccess":true}`,
			fallbackLevel: 1,
			count:         15, // Most used tool
		},
		{
			toolName:      "Bash",
			contextJSON:   `{"Category":1,"ToolName":"Bash","IsSuccess":true}`,
			fallbackLevel: 3,
			count:         10,
		},
		{
			toolName:      "Read",
			contextJSON:   `{"Category":0,"ToolName":"Read","IsLoading":true}`,
			fallbackLevel: 2,
			count:         5, // Least used tool
		},
	}

	for _, event := range testEvents {
		for i := 0; i < event.count; i++ {
			_, err = db.Exec(`
				INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
				VALUES (?, ?, ?, ?, ?, ?)`,
				now-int64(i*60), "test-session", event.toolName, "test.wav", event.fallbackLevel, event.contextJSON)
			if err != nil {
				t.Fatalf("Failed to insert test event: %v", err)
			}
		}
	}

	// TDD RED: Test GetToolUsageStats function that doesn't exist yet
	filter := QueryFilter{
		Days:      0, // All time
		Limit:     0, // No limit
		OrderBy:   "usage_count",
		OrderDesc: true, // Most used first
	}

	toolStats, err := GetToolUsageStats(db, filter)
	if err != nil {
		t.Fatalf("GetToolUsageStats failed: %v", err)
	}

	// Should have 3 tools
	if len(toolStats) != 3 {
		t.Errorf("Expected 3 tools, got %d", len(toolStats))
	}

	// First tool should be Edit (most used)
	if toolStats[0].ToolName != "Edit" {
		t.Errorf("Expected first tool to be 'Edit', got '%s'", toolStats[0].ToolName)
	}
	if toolStats[0].UsageCount != 15 {
		t.Errorf("Expected Edit usage count 15, got %d", toolStats[0].UsageCount)
	}
	// Second tool should be Bash
	if toolStats[1].ToolName != "Bash" {
		t.Errorf("Expected second tool to be 'Bash', got '%s'", toolStats[1].ToolName)
	}
	if toolStats[1].UsageCount != 10 {
		t.Errorf("Expected Bash usage count 10, got %d", toolStats[1].UsageCount)
	}

	// Third tool should be Read (least used)
	if toolStats[2].ToolName != "Read" {
		t.Errorf("Expected third tool to be 'Read', got '%s'", toolStats[2].ToolName)
	}
	if toolStats[2].UsageCount != 5 {
		t.Errorf("Expected Read usage count 5, got %d", toolStats[2].UsageCount)
	}
}

// TDD RED: Test GetCategoryDistribution function that doesn't exist yet  
func TestGetCategoryDistribution(t *testing.T) {
	// Create temporary directory for test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "category_test.db")

	// Set up test database
	db, err := NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert test data with different categories
	now := time.Now().Unix()
	testEvents := []struct {
		category string
		count    int
	}{
		{"success", 20},  // Most common
		{"loading", 15},
		{"error", 10},
		{"interactive", 5}, // Least common
	}

	for _, event := range testEvents {
		categoryInt := categoryStringToInt(event.category)
		contextJSON := `{"Category":` + string(rune(categoryInt+48)) + `,"ToolName":"Test"}`
		
		for i := 0; i < event.count; i++ {
			_, err = db.Exec(`
				INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, chain_type, context)
				VALUES (?, ?, ?, ?, ?, ?)`,
				now-int64(i*60), "test-session", "Test", "test.wav", 1, contextJSON)
			if err != nil {
				t.Fatalf("Failed to insert test event: %v", err)
			}
		}
	}

	// TDD RED: Test GetCategoryDistribution function that doesn't exist yet
	filter := QueryFilter{
		Days:      0, // All time
		OrderBy:   "count",
		OrderDesc: true, // Most common first
	}

	categoryDist, err := GetCategoryDistribution(db, filter)
	if err != nil {
		t.Fatalf("GetCategoryDistribution failed: %v", err)
	}

	// Should have 4 categories
	if len(categoryDist) != 4 {
		t.Errorf("Expected 4 categories, got %d", len(categoryDist))
	}

	// Check ordering (success should be first)
	if categoryDist[0].Category != "success" {
		t.Errorf("Expected first category to be 'success', got '%s'", categoryDist[0].Category)
	}
	if categoryDist[0].Count != 20 {
		t.Errorf("Expected success count 20, got %d", categoryDist[0].Count)
	}

	// Check percentages (success = 20/50 = 40%)
	expectedPercentage := 40.0
	if categoryDist[0].Percentage < expectedPercentage-0.1 || categoryDist[0].Percentage > expectedPercentage+0.1 {
		t.Errorf("Expected success percentage ~%.1f%%, got %.1f%%", expectedPercentage, categoryDist[0].Percentage)
	}

	// Check last category (interactive should be last)
	if categoryDist[3].Category != "interactive" {
		t.Errorf("Expected last category to be 'interactive', got '%s'", categoryDist[3].Category)
	}
	if categoryDist[3].Count != 5 {
		t.Errorf("Expected interactive count 5, got %d", categoryDist[3].Count)
	}
}

// TestGetChainTypeStatistics asserts the new per-chain-type analytics
// surface that replaced fallback_level aggregation. Each event's
// chain_type is recorded; depth comes from the path_lookups row whose
// path matches selected_path.
func TestGetChainTypeStatistics(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now().Unix()

	// Three events: two "enhanced" (depths 1 and 3) and one "posttool"
	// (depth 2). Avg depth: enhanced=(1+3)/2=2.0; posttool=2.0.
	events := []struct {
		sessionID    string
		toolName     string
		selectedPath string
		chainType    string
		paths        []struct {
			path  string
			seq   int
			found int
		}
	}{
		{
			sessionID:    "s1",
			toolName:     "git",
			selectedPath: "loading/git-start.wav",
			chainType:    "enhanced",
			paths: []struct {
				path  string
				seq   int
				found int
			}{
				{"loading/git-start.wav", 1, 1},
			},
		},
		{
			sessionID:    "s2",
			toolName:     "bash",
			selectedPath: "loading/loading.wav",
			chainType:    "enhanced",
			paths: []struct {
				path  string
				seq   int
				found int
			}{
				{"loading/bash-start.wav", 1, 0},
				{"loading/bash.wav", 2, 0},
				{"loading/loading.wav", 3, 1},
			},
		},
		{
			sessionID:    "s3",
			toolName:     "edit",
			selectedPath: "success/edit-success.wav",
			chainType:    "posttool",
			paths: []struct {
				path  string
				seq   int
				found int
			}{
				{"success/edit-success.wav", 2, 1},
			},
		},
	}

	for i, e := range events {
		res, err := db.Exec(`INSERT INTO hook_events
			(timestamp, session_id, tool_name, selected_path, chain_type, context)
			VALUES (?, ?, ?, ?, ?, '{"Category":1,"ToolName":"`+e.toolName+`"}')`,
			now-int64(i*60), e.sessionID, e.toolName, e.selectedPath, e.chainType)
		if err != nil {
			t.Fatalf("insert event %d: %v", i, err)
		}
		eventID, _ := res.LastInsertId()
		for _, p := range e.paths {
			if _, err := db.Exec(`INSERT INTO path_lookups (event_id, path, sequence, found) VALUES (?, ?, ?, ?)`,
				eventID, p.path, p.seq, p.found); err != nil {
				t.Fatalf("insert path lookup: %v", err)
			}
		}
	}

	stats, err := GetChainTypeStatistics(db, QueryFilter{})
	if err != nil {
		t.Fatalf("GetChainTypeStatistics: %v", err)
	}
	if len(stats) != 2 {
		t.Fatalf("expected 2 chain-type rows, got %d", len(stats))
	}

	byChain := map[string]ChainTypeStatistic{}
	for _, s := range stats {
		byChain[s.ChainType] = s
	}

	enh, ok := byChain["enhanced"]
	if !ok {
		t.Fatal("expected 'enhanced' chain type in results")
	}
	if enh.EventCount != 2 {
		t.Errorf("expected enhanced EventCount=2, got %d", enh.EventCount)
	}
	if enh.AvgDepth < 1.99 || enh.AvgDepth > 2.01 {
		t.Errorf("expected enhanced AvgDepth=2.0, got %.2f", enh.AvgDepth)
	}

	post, ok := byChain["posttool"]
	if !ok {
		t.Fatal("expected 'posttool' chain type in results")
	}
	if post.EventCount != 1 {
		t.Errorf("expected posttool EventCount=1, got %d", post.EventCount)
	}
	if post.AvgDepth < 1.99 || post.AvgDepth > 2.01 {
		t.Errorf("expected posttool AvgDepth=2.0, got %.2f", post.AvgDepth)
	}

	// Percentages: enhanced 66.7%, posttool 33.3%. Sum to 100.
	totalPct := 0.0
	for _, s := range stats {
		totalPct += s.Percentage
	}
	if totalPct < 99.9 || totalPct > 100.1 {
		t.Errorf("expected percentages to sum to ~100, got %.2f", totalPct)
	}
}