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
			INSERT INTO hook_events (timestamp, session_id, tool_name, selected_path, fallback_level, context)
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
	query := MissingSoundQuery{
		Days:  0,    // All time
		Tool:  "",   // All tools
		Limit: 20,   // Limit results
	}

	missingSounds, err := GetMissingSounds(db, query)
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