package sounds

import (
	"testing"

	"github.com/ctoth/claudio/internal/hooks"
)

func TestSoundMapper(t *testing.T) {
	mapper := NewSoundMapper()

	if mapper == nil {
		t.Fatal("NewSoundMapper returned nil")
	}
}

func TestMapSound5LevelFallback(t *testing.T) {
	mapper := NewSoundMapper()

	testCases := []struct {
		name           string
		context        *hooks.EventContext
		expectedPaths  []string // 5-level fallback system
		expectedLevel  int      // which level should be selected (0-based)
	}{
		{
			name: "bash thinking - full context",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "Bash",
				SoundHint: "bash-thinking",
				Operation: "tool-start",
			},
			expectedPaths: []string{
				"loading/bash-thinking.wav",    // Level 1: exact hint
				"loading/bash.wav",             // Level 2: tool-specific  
				"loading/tool-start.wav",       // Level 3: operation-specific
				"loading/loading.wav",          // Level 4: category-specific
				"default.wav",                  // Level 5: default fallback
			},
			expectedLevel: 0, // Should select level 1 (exact hint)
		},
		{
			name: "edit success - no hint",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Edit",
				Operation: "tool-complete",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/edit.wav",           // Level 2: tool-specific (level 1 skipped)
				"success/tool-complete.wav", // Level 3: operation-specific
				"success/success.wav",       // Level 4: category-specific
				"default.wav",               // Level 5: default fallback
			},
			expectedLevel: 0, // Should select level 2 (tool-specific) - first in paths array
		},
		{
			name: "error with stderr - no tool name",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				SoundHint: "stderr-error",
				Operation: "tool-complete",
				HasError:  true,
			},
			expectedPaths: []string{
				"error/stderr-error.wav",    // Level 1: exact hint
				"error/tool-complete.wav",   // Level 3: operation (level 2 skipped)
				"error/error.wav",           // Level 4: category-specific
				"default.wav",               // Level 5: default fallback
			},
			expectedLevel: 0, // Should select level 1 (exact hint)
		},
		{
			name: "notification - minimal context",
			context: &hooks.EventContext{
				Category: hooks.Interactive,
			},
			expectedPaths: []string{
				"interactive/interactive.wav", // Level 4: category-specific only
				"default.wav",                 // Level 5: default fallback
			},
			expectedLevel: 0, // Should select level 4 (category)
		},
		{
			name: "file operation with context",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Write",
				SoundHint: "file-saved",
				Operation: "tool-complete",
				FileType:  "go",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/file-saved.wav",
				"success/write.wav", 
				"success/tool-complete.wav",
				"success/success.wav",
				"default.wav",
			},
			expectedLevel: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapper.MapSound(tc.context)

			// Verify result structure
			if result == nil {
				t.Fatal("MapSound returned nil result")
			}

			// Verify paths match expected
			if len(result.AllPaths) != len(tc.expectedPaths) {
				t.Errorf("Expected %d paths, got %d", len(tc.expectedPaths), len(result.AllPaths))
				t.Logf("Expected: %v", tc.expectedPaths)
				t.Logf("Got: %v", result.AllPaths)
				return
			}

			for i, expected := range tc.expectedPaths {
				if result.AllPaths[i] != expected {
					t.Errorf("Path[%d] = %s, expected %s", i, result.AllPaths[i], expected)
				}
			}

			// Verify selected path is correct
			expectedSelected := tc.expectedPaths[tc.expectedLevel]
			if result.SelectedPath != expectedSelected {
				t.Errorf("SelectedPath = %s, expected %s", result.SelectedPath, expectedSelected)
			}

			// Note: FallbackLevel is calculated separately and represents the actual level used (1-5)

			// Verify total paths
			if result.TotalPaths != len(tc.expectedPaths) {
				t.Errorf("TotalPaths = %d, expected %d", result.TotalPaths, len(tc.expectedPaths))
			}
		})
	}
}

func TestMapSoundPathNormalization(t *testing.T) {
	mapper := NewSoundMapper()

	testCases := []struct {
		name        string
		input       string
		expected    string
	}{
		{"lowercase", "Bash", "bash"},
		{"spaces to hyphens", "tool interrupted", "tool-interrupted"},
		{"underscores to hyphens", "file_saved", "file-saved"},
		{"mixed case and chars", "Multi_Edit Thing", "multi-edit-thing"},
		{"special chars removed", "tool@#$error!", "tool-error"},
		{"multiple hyphens collapsed", "tool---error", "tool-error"}, // Should collapse multiple hyphens
		{"numbers preserved", "mp3-decoder-v2", "mp3-decoder-v2"},
		{"already normalized", "bash-thinking", "bash-thinking"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test normalization through a context that will use the input
			context := &hooks.EventContext{
				Category:  hooks.Loading,
				SoundHint: tc.input,
			}

			result := mapper.MapSound(context)
			expected := "loading/" + tc.expected + ".wav"

			if len(result.AllPaths) == 0 {
				t.Fatal("No paths generated")
			}

			if result.AllPaths[0] != expected {
				t.Errorf("Normalized path = %s, expected %s", result.AllPaths[0], expected)
			}
		})
	}
}

func TestMapSoundFallbackSelection(t *testing.T) {
	mapper := NewSoundMapper()

	// Create a mapper that can tell us which files exist vs don't exist
	// For testing, we'll simulate file existence checking
	
	testCases := []struct {
		name          string
		context       *hooks.EventContext
		existingFiles map[string]bool // which files "exist"
		expectedPath  string
		expectedLevel int
	}{
		{
			name: "level 1 exists",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Bash", 
				SoundHint: "bash-success",
			},
			existingFiles: map[string]bool{
				"success/bash-success.wav": true,
			},
			expectedPath:  "success/bash-success.wav",
			expectedLevel: 1,
		},
		{
			name: "level 1 missing, level 2 exists",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Bash",
				SoundHint: "bash-success",
			},
			existingFiles: map[string]bool{
				"success/bash-success.wav": false,
				"success/bash.wav":         true,
			},
			expectedPath:  "success/bash.wav",
			expectedLevel: 2,
		},
		{
			name: "fallback to category level",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				ToolName:  "UnknownTool",
				SoundHint: "unknown-error",
			},
			existingFiles: map[string]bool{
				"error/unknown-error.wav":  false,
				"error/unknowntool.wav":    false,
				"error/error.wav":          true,
			},
			expectedPath:  "error/error.wav",
			expectedLevel: 4,
		},
		{
			name: "fallback to default",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "NewTool",
				SoundHint: "new-loading",
			},
			existingFiles: map[string]bool{
				"loading/new-loading.wav": false,
				"loading/newtool.wav":     false,
				"loading/loading.wav":     false,
				"default.wav":             true,
			},
			expectedPath:  "default.wav",
			expectedLevel: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// For now, just test that the mapper generates the right paths
			// File existence checking will be implemented in the actual file resolver
			result := mapper.MapSound(tc.context)

			// Find the expected path in the generated paths
			found := false
			for _, path := range result.AllPaths {
				if path == tc.expectedPath {
					found = true
					// In a real implementation, this would be the first existing file
					break
				}
			}

			if !found {
				t.Errorf("Expected path %s not found in generated paths: %v", tc.expectedPath, result.AllPaths)
			}
		})
	}
}

func TestMapSoundEdgeCases(t *testing.T) {
	mapper := NewSoundMapper()

	t.Run("nil context", func(t *testing.T) {
		result := mapper.MapSound(nil)

		if result == nil {
			t.Fatal("MapSound should handle nil context gracefully")
		}

		// Should fallback to default only
		expectedPaths := []string{"default.wav"}
		if len(result.AllPaths) != len(expectedPaths) {
			t.Errorf("Expected %d paths for nil context, got %d", len(expectedPaths), len(result.AllPaths))
		}

		if result.AllPaths[0] != "default.wav" {
			t.Errorf("Expected default.wav for nil context, got %s", result.AllPaths[0])
		}

		if result.FallbackLevel != 6 {
			t.Errorf("Expected fallback level 6 for nil context, got %d", result.FallbackLevel)
		}
	})

	t.Run("empty strings", func(t *testing.T) {
		result := mapper.MapSound(&hooks.EventContext{
			Category:  hooks.Interactive,
			ToolName:  "",
			SoundHint: "",
			Operation: "",
		})

		// Should only have category and default
		expectedPaths := []string{
			"interactive/interactive.wav",
			"default.wav",
		}

		if len(result.AllPaths) != len(expectedPaths) {
			t.Errorf("Expected %d paths, got %d", len(expectedPaths), len(result.AllPaths))
		}

		for i, expected := range expectedPaths {
			if result.AllPaths[i] != expected {
				t.Errorf("Path[%d] = %s, expected %s", i, result.AllPaths[i], expected)
			}
		}
	})

	t.Run("unknown category", func(t *testing.T) {
		// This tests what happens with an invalid category value
		context := &hooks.EventContext{
			Category: hooks.EventCategory(999), // Invalid category
		}

		result := mapper.MapSound(context)

		// Should still work, falling back to default
		if len(result.AllPaths) == 0 {
			t.Error("Should generate at least default path for unknown category")
		}

		// Last path should always be default
		lastPath := result.AllPaths[len(result.AllPaths)-1]
		if lastPath != "default.wav" {
			t.Errorf("Last path should be default.wav, got %s", lastPath)
		}
	})
}

func TestMapSoundResultMetadata(t *testing.T) {
	mapper := NewSoundMapper()

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		SoundHint: "file-saved",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context)

	// Verify all metadata fields are set correctly
	if result.SelectedPath == "" {
		t.Error("SelectedPath should not be empty")
	}

	if result.FallbackLevel < 1 || result.FallbackLevel > 5 {
		t.Errorf("FallbackLevel should be 1-5, got %d", result.FallbackLevel)
	}

	if result.TotalPaths != len(result.AllPaths) {
		t.Errorf("TotalPaths (%d) should match len(AllPaths) (%d)", result.TotalPaths, len(result.AllPaths))
	}

	if len(result.AllPaths) == 0 {
		t.Error("AllPaths should not be empty")
	}

	// First path should be the selected path (in the basic case)
	if result.SelectedPath != result.AllPaths[0] {
		t.Errorf("SelectedPath (%s) should match first path (%s) in basic case", result.SelectedPath, result.AllPaths[0])
	}
}

func TestMapSoundWithOriginalToolFallback(t *testing.T) {
	mapper := NewSoundMapper()

	// Test context with extracted command but original tool for fallback
	context := &hooks.EventContext{
		Category:     hooks.Success,
		ToolName:     "git", // Extracted from Bash
		OriginalTool: "Bash", // Original tool for fallback
		SoundHint:    "git-commit-success",
		Operation:    "tool-complete",
	}

	result := mapper.MapSound(context)

	// Should have proper fallback including original tool
	expectedPaths := []string{
		"success/git-commit-success.wav",
		"success/git.wav",
		"success/bash.wav", // Original tool fallback
		"success/tool-complete.wav",
		"success/success.wav",
		"default.wav",
	}

	if len(result.AllPaths) != len(expectedPaths) {
		t.Errorf("Expected %d paths, got %d", len(expectedPaths), len(result.AllPaths))
		t.Logf("Expected: %v", expectedPaths)
		t.Logf("Got: %v", result.AllPaths)
		return
	}

	for i, expected := range expectedPaths {
		if result.AllPaths[i] != expected {
			t.Errorf("Path[%d] = %s, expected %s", i, result.AllPaths[i], expected)
		}
	}
}