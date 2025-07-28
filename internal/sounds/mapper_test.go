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

// TDD Phase 2.1 RED: Event-specific fallback chain differentiation tests
func TestEventSpecificFallbackChains(t *testing.T) {
	mapper := NewSoundMapper()

	tests := []struct {
		name               string
		eventName          string
		context            *hooks.EventContext
		expectedChainType  string
		expectedPathCount  int
		description        string
	}{
		{
			name:      "PreToolUse events use 9-level enhanced fallback chain",
			eventName: "PreToolUse",
			context: &hooks.EventContext{
				Category:     hooks.Loading,
				ToolName:     "git",
				OriginalTool: "Bash", 
				SoundHint:    "git-commit-start",
				Operation:    "tool-start",
			},
			expectedChainType:  "enhanced",
			expectedPathCount:  9,
			description:        "PreToolUse should use 9-level enhanced fallback with command-only levels",
		},
		{
			name:      "PostToolUse success events use 6-level fallback chain",
			eventName: "PostToolUse",
			context: &hooks.EventContext{
				Category:     hooks.Success,
				ToolName:     "git",
				OriginalTool: "Bash",
				SoundHint:    "git-commit-success", 
				Operation:    "tool-complete",
				IsSuccess:    true,
			},
			expectedChainType:  "posttool",
			expectedPathCount:  6,
			description:        "PostToolUse should use 6-level fallback (skip command-only sounds)",
		},
		{
			name:      "PostToolUse error events use 6-level fallback chain",
			eventName: "PostToolUse",
			context: &hooks.EventContext{
				Category:     hooks.Error,
				ToolName:     "git",
				OriginalTool: "Bash",
				SoundHint:    "git-commit-error",
				Operation:    "tool-complete",
				HasError:     true,
			},
			expectedChainType:  "posttool",
			expectedPathCount:  6,
			description:        "PostToolUse error should use 6-level fallback (skip command-only sounds)",
		},
		{
			name:      "UserPromptSubmit events use 4-level simple fallback chain",
			eventName: "UserPromptSubmit",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "message-sent",
				Operation: "prompt",
			},
			expectedChainType:  "simple",
			expectedPathCount:  4,
			description:        "UserPromptSubmit should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "Notification events use 4-level simple fallback chain",
			eventName: "Notification",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "notification-permission",
				Operation: "notification",
			},
			expectedChainType:  "simple",
			expectedPathCount:  4,
			description:        "Notification should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "Stop events use 4-level simple fallback chain", 
			eventName: "Stop",
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "agent-complete",
				Operation: "stop",
			},
			expectedChainType:  "simple",
			expectedPathCount:  4,
			description:        "Stop should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "SubagentStop events use 4-level simple fallback chain",
			eventName: "SubagentStop", 
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "subagent-complete",
				Operation: "subagent-stop",
			},
			expectedChainType:  "simple",
			expectedPathCount:  4,
			description:        "SubagentStop should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "PreCompact events use 4-level simple fallback chain",
			eventName: "PreCompact",
			context: &hooks.EventContext{
				Category:  hooks.System,
				SoundHint: "compacting",
				Operation: "compact",
			},
			expectedChainType:  "simple",  
			expectedPathCount:  4,
			description:        "PreCompact should use 4-level simple fallback (no tool commands)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapSound(tt.context)

			if result == nil {
				t.Fatal("MapSound returned nil result")
			}

			// Test will fail initially - mapper doesn't differentiate by event type yet
			if len(result.AllPaths) != tt.expectedPathCount {
				t.Errorf("%s: expected %d paths for %s chain, got %d. %s", 
					tt.name, tt.expectedPathCount, tt.expectedChainType, len(result.AllPaths), tt.description)
				t.Logf("Got paths: %v", result.AllPaths)
			}

			// Verify the mapper result indicates the correct chain type
			// This metadata will need to be added to SoundMappingResult
			if result.ChainType != tt.expectedChainType {
				t.Errorf("%s: expected chain type '%s', got '%s'", 
					tt.name, tt.expectedChainType, result.ChainType)
			}
		})
	}
}

// TDD Phase 2.1 RED: PreToolUse enhanced 9-level fallback chain test
func TestPreToolUse9LevelEnhancedFallback(t *testing.T) {
	mapper := NewSoundMapper()

	context := &hooks.EventContext{
		Category:     hooks.Loading,
		ToolName:     "git",
		OriginalTool: "Bash",
		SoundHint:    "git-commit-start", 
		Operation:    "tool-start",
	}

	result := mapper.MapSound(context)

	// PreToolUse should generate 9-level enhanced fallback including command-only sounds
	expectedPaths := []string{
		"loading/git-commit-start.wav",    // 1. Exact hint match
		"loading/git-commit.wav",          // 2. Command-subcommand without suffix  
		"loading/git-start.wav",           // 3. Command with suffix
		"loading/git.wav",                 // 4. Command-only (command fallback)
		"loading/bash-start.wav",          // 5. Original tool with suffix
		"loading/bash.wav",                // 6. Original tool fallback
		"loading/tool-start.wav",          // 7. Operation-specific
		"loading/loading.wav",             // 8. Category-specific
		"default.wav",                     // 9. Default fallback
	}

	// Test will fail initially - current mapper doesn't generate this chain structure
	if len(result.AllPaths) != len(expectedPaths) {
		t.Errorf("PreToolUse 9-level chain: expected %d paths, got %d", 
			len(expectedPaths), len(result.AllPaths))
		t.Logf("Expected: %v", expectedPaths)
		t.Logf("Got: %v", result.AllPaths)
	}

	for i, expected := range expectedPaths {
		if i < len(result.AllPaths) && result.AllPaths[i] != expected {
			t.Errorf("PreToolUse path[%d] = %s, expected %s", i, result.AllPaths[i], expected)
		}
	}
}

// TDD Phase 2.1 RED: PostToolUse 6-level fallback chain test (skip command-only sounds)
func TestPostToolUse6LevelFallback(t *testing.T) {
	mapper := NewSoundMapper()

	tests := []struct {
		name     string
		context  *hooks.EventContext
		expected []string
	}{
		{
			name: "PostToolUse success skips command-only sounds",
			context: &hooks.EventContext{
				Category:     hooks.Success,
				ToolName:     "git",
				OriginalTool: "Bash",
				SoundHint:    "git-commit-success",
				Operation:    "tool-complete",
				IsSuccess:    true,
			},
			expected: []string{
				"success/git-commit-success.wav", // 1. Exact hint match
				"success/git-success.wav",        // 2. Command with suffix (no command-only)
				"success/bash-success.wav",       // 3. Original tool with suffix  
				"success/tool-complete.wav",      // 4. Operation-specific
				"success/success.wav",            // 5. Category-specific
				"default.wav",                    // 6. Default fallback
			},
		},
		{
			name: "PostToolUse error skips command-only sounds",
			context: &hooks.EventContext{
				Category:     hooks.Error,
				ToolName:     "git", 
				OriginalTool: "Bash",
				SoundHint:    "git-commit-error",
				Operation:    "tool-complete",
				HasError:     true,
			},
			expected: []string{
				"error/git-commit-error.wav", // 1. Exact hint match
				"error/git-error.wav",        // 2. Command with suffix (no command-only)
				"error/bash-error.wav",       // 3. Original tool with suffix
				"error/tool-complete.wav",    // 4. Operation-specific  
				"error/error.wav",            // 5. Category-specific
				"default.wav",                // 6. Default fallback
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapSound(tt.context)

			// Test will fail initially - current mapper doesn't skip command-only sounds
			if len(result.AllPaths) != len(tt.expected) {
				t.Errorf("%s: expected %d paths, got %d", tt.name, len(tt.expected), len(result.AllPaths))
				t.Logf("Expected: %v", tt.expected)
				t.Logf("Got: %v", result.AllPaths)
			}

			for i, expected := range tt.expected {
				if i < len(result.AllPaths) && result.AllPaths[i] != expected {
					t.Errorf("%s path[%d] = %s, expected %s", tt.name, i, result.AllPaths[i], expected)
				}
			}
		})
	}
}

// TDD Phase 2.1 RED: Simple event 4-level fallback chain test  
func TestSimpleEvent4LevelFallback(t *testing.T) {
	mapper := NewSoundMapper()

	tests := []struct {
		name     string
		context  *hooks.EventContext
		expected []string
	}{
		{
			name: "UserPromptSubmit simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "message-sent",
				Operation: "prompt",
			},
			expected: []string{
				"interactive/message-sent.wav",      // 1. Specific hint
				"interactive/prompt-submit.wav",     // 2. Event-specific (not tool-based)
				"interactive/interactive.wav",       // 3. Category-specific 
				"default.wav",                       // 4. Default fallback
			},
		},
		{
			name: "Notification simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "notification-permission",
				Operation: "notification",
			},
			expected: []string{
				"interactive/notification-permission.wav", // 1. Specific hint
				"interactive/notification.wav",            // 2. Event-specific
				"interactive/interactive.wav",             // 3. Category-specific
				"default.wav",                             // 4. Default fallback
			},  
		},
		{
			name: "Stop completion simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "agent-complete",
				Operation: "stop",
			},
			expected: []string{
				"completion/agent-complete.wav", // 1. Specific hint
				"completion/stop.wav",           // 2. Event-specific  
				"completion/completion.wav",     // 3. Category-specific
				"default.wav",                   // 4. Default fallback
			},
		},
		{
			name: "SubagentStop completion simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "subagent-complete",
				Operation: "subagent-stop",
			},
			expected: []string{
				"completion/subagent-complete.wav", // 1. Specific hint  
				"completion/subagent-stop.wav",     // 2. Event-specific
				"completion/completion.wav",        // 3. Category-specific
				"default.wav",                      // 4. Default fallback
			},
		},
		{
			name: "PreCompact system simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.System,
				SoundHint: "compacting", 
				Operation: "compact",
			},
			expected: []string{
				"system/compacting.wav",     // 1. Specific hint
				"system/pre-compact.wav",    // 2. Event-specific
				"system/system.wav",         // 3. Category-specific  
				"default.wav",               // 4. Default fallback
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapper.MapSound(tt.context)

			// Test will fail initially - current mapper doesn't generate event-specific paths
			if len(result.AllPaths) != len(tt.expected) {
				t.Errorf("%s: expected %d paths, got %d", tt.name, len(tt.expected), len(result.AllPaths))
				t.Logf("Expected: %v", tt.expected)
				t.Logf("Got: %v", result.AllPaths) 
			}

			for i, expected := range tt.expected {
				if i < len(result.AllPaths) && result.AllPaths[i] != expected {
					t.Errorf("%s path[%d] = %s, expected %s", tt.name, i, result.AllPaths[i], expected)
				}
			}
		})
	}
}