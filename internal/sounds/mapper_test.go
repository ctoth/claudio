package sounds

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"claudio.click/internal/hooks"
	"claudio.click/internal/tracking"
)

func TestSoundMapper(t *testing.T) {
	mapper := NewSoundMapper(nil)

	if mapper == nil {
		t.Fatal("NewSoundMapper returned nil")
	}
}

func TestMapSoundEventSpecificFallback(t *testing.T) {
	mapper := NewSoundMapper(nil)

	testCases := []struct {
		name          string
		context       *hooks.EventContext
		expectedPaths []string // Event-specific fallback system
		expectedLevel int      // which level should be selected (0-based)
		chainType     string   // expected chain type
	}{
		{
			name: "bash thinking - enhanced 9-level fallback",
			context: &hooks.EventContext{
				Category:     hooks.Loading,
				ToolName:     "Bash",
				OriginalTool: "", // Not extracted from another tool
				SoundHint:    "bash-thinking",
				Operation:    "tool-start",
			},
			expectedPaths: []string{
				"loading/bash-thinking.wav", // Level 1: exact hint
				"loading/bash-start.wav",    // Level 2: command with suffix (bash becomes "bash")
				"loading/bash.wav",          // Level 3: command-only (bash tool)
				"loading/tool-start.wav",    // Level 4: operation-specific
				"loading/loading.wav",       // Level 5: category-specific
				"default.wav",               // Level 6: default fallback
			},
			expectedLevel: 0, // Should select level 1 (exact hint)
			chainType:     "enhanced",
		},
		{
			name: "edit success - PostTool 6-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Edit",
				Operation: "tool-complete",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/edit-success.wav",  // Level 1: command with suffix
				"success/tool-complete.wav", // Level 2: operation-specific
				"success/success.wav",       // Level 3: category-specific
				"default.wav",               // Level 4: default fallback
			},
			expectedLevel: 0, // Should select level 1 (command with suffix)
			chainType:     "posttool",
		},
		{
			name: "error with stderr - simple 4-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				SoundHint: "stderr-error",
				Operation: "tool-complete",
				HasError:  true,
			},
			expectedPaths: []string{
				"error/stderr-error.wav",  // Level 1: exact hint
				"error/tool-complete.wav", // Level 2: event-specific
				"error/error.wav",         // Level 3: category-specific
				"default.wav",             // Level 4: default fallback
			},
			expectedLevel: 0, // Should select level 1 (exact hint)
			chainType:     "simple",
		},
		{
			name: "notification - simple 4-level fallback",
			context: &hooks.EventContext{
				Category: hooks.Interactive,
			},
			expectedPaths: []string{
				"interactive/interactive.wav", // Level 1: category-specific only
				"default.wav",                 // Level 2: default fallback
			},
			expectedLevel: 0, // Should select level 1 (category)
			chainType:     "simple",
		},
		{
			name: "file operation with context - PostTool 6-level fallback",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Write",
				SoundHint: "file-saved",
				Operation: "tool-complete",
				FileType:  "go",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/file-saved.wav",    // Level 1: exact hint
				"success/write-success.wav", // Level 2: command with suffix
				"success/tool-complete.wav", // Level 3: operation-specific
				"success/success.wav",       // Level 4: category-specific
				"default.wav",               // Level 5: default fallback
			},
			expectedLevel: 0,
			chainType:     "posttool",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapper.MapSound(tc.context)

			// Verify result structure
			if result == nil {
				t.Fatal("MapSound returned nil result")
			}

			// Verify chain type is correct
			if result.ChainType != tc.chainType {
				t.Errorf("ChainType = %s, expected %s", result.ChainType, tc.chainType)
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

			// Verify total paths
			if result.TotalPaths != len(tc.expectedPaths) {
				t.Errorf("TotalPaths = %d, expected %d", result.TotalPaths, len(tc.expectedPaths))
			}
		})
	}
}

func TestMapSoundPathNormalization(t *testing.T) {
	mapper := NewSoundMapper(nil)

	testCases := []struct {
		name     string
		input    string
		expected string
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
	mapper := NewSoundMapper(nil)

	// Test fallback path generation - updated for event-specific architecture

	testCases := []struct {
		name          string
		context       *hooks.EventContext
		expectedPaths []string // paths that should be generated
		description   string
	}{
		{
			name: "PostToolUse success generates correct fallback paths",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Bash",
				SoundHint: "bash-success",
				Operation: "tool-complete",
			},
			expectedPaths: []string{
				"success/bash-success.wav",  // Level 1: exact hint
				"success/bash-success.wav",  // Level 2: command with suffix (bash-success)
				"success/tool-complete.wav", // Level 3: operation-specific
				"success/success.wav",       // Level 4: category-specific
				"default.wav",               // Level 5: default fallback
			},
			description: "PostToolUse should use 6-level fallback (skips bare command-only sounds like bash.wav)",
		},
		{
			name: "Error event generates simple fallback paths",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				SoundHint: "unknown-error",
			},
			expectedPaths: []string{
				"error/unknown-error.wav", // Level 1: exact hint
				"error/error.wav",         // Level 2: category-specific
				"default.wav",             // Level 3: default fallback
			},
			description: "Error without tool should use simple 4-level fallback",
		},
		{
			name: "Loading event generates enhanced fallback paths",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "NewTool",
				SoundHint: "new-loading",
				Operation: "tool-start",
			},
			expectedPaths: []string{
				"loading/new-loading.wav",   // Level 1: exact hint
				"loading/newtool-start.wav", // Level 2: command with suffix
				"loading/newtool.wav",       // Level 3: command-only
				"loading/tool-start.wav",    // Level 4: operation-specific
				"loading/loading.wav",       // Level 5: category-specific
				"default.wav",               // Level 6: default fallback
			},
			description: "Loading with tool should use enhanced 9-level fallback including command-only",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mapper.MapSound(tc.context)

			// Test that the expected paths are generated
			t.Logf("%s: Generated paths: %v", tc.description, result.AllPaths)

			// Check that important expected paths are present
			for _, expectedPath := range tc.expectedPaths {
				found := false
				for _, actualPath := range result.AllPaths {
					if actualPath == expectedPath {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected path %s not found in generated paths: %v", expectedPath, result.AllPaths)
				}
			}
		})
	}
}

func TestMapSoundEdgeCases(t *testing.T) {
	mapper := NewSoundMapper(nil)

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
	mapper := NewSoundMapper(nil)

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
	mapper := NewSoundMapper(nil)

	// Test context with extracted command but original tool for fallback
	context := &hooks.EventContext{
		Category:     hooks.Success,
		ToolName:     "git",  // Extracted from Bash
		OriginalTool: "Bash", // Original tool for fallback
		SoundHint:    "git-commit-success",
		Operation:    "tool-complete",
	}

	result := mapper.MapSound(context)

	// PostToolUse 6-level fallback (skips command-only sounds)
	expectedPaths := []string{
		"success/git-commit-success.wav", // Level 1: exact hint
		"success/git-success.wav",        // Level 2: command with suffix
		"success/bash-success.wav",       // Level 3: original tool with suffix
		"success/tool-complete.wav",      // Level 4: operation-specific
		"success/success.wav",            // Level 5: category-specific
		"default.wav",                    // Level 6: default fallback
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

	// Verify this uses PostToolUse chain type
	if result.ChainType != "posttool" {
		t.Errorf("Expected chain type 'posttool', got %s", result.ChainType)
	}
}

// TDD Phase 2.1 RED: Event-specific fallback chain differentiation tests
func TestEventSpecificFallbackChains(t *testing.T) {
	mapper := NewSoundMapper(nil)

	tests := []struct {
		name              string
		eventName         string
		context           *hooks.EventContext
		expectedChainType string
		expectedPathCount int
		description       string
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
			expectedChainType: "enhanced",
			expectedPathCount: 9,
			description:       "PreToolUse should use 9-level enhanced fallback with command-only levels",
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
			expectedChainType: "posttool",
			expectedPathCount: 6,
			description:       "PostToolUse should use 6-level fallback (skip command-only sounds)",
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
			expectedChainType: "posttool",
			expectedPathCount: 6,
			description:       "PostToolUse error should use 6-level fallback (skip command-only sounds)",
		},
		{
			name:      "UserPromptSubmit events use 4-level simple fallback chain",
			eventName: "UserPromptSubmit",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "message-sent",
				Operation: "prompt",
			},
			expectedChainType: "simple",
			expectedPathCount: 4,
			description:       "UserPromptSubmit should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "Notification events use 4-level simple fallback chain",
			eventName: "Notification",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "notification-permission",
				Operation: "notification",
			},
			expectedChainType: "simple",
			expectedPathCount: 4,
			description:       "Notification should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "Stop events use 4-level simple fallback chain",
			eventName: "Stop",
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "agent-complete",
				Operation: "stop",
			},
			expectedChainType: "simple",
			expectedPathCount: 4,
			description:       "Stop should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "SubagentStop events use 4-level simple fallback chain",
			eventName: "SubagentStop",
			context: &hooks.EventContext{
				Category:  hooks.Completion,
				SoundHint: "subagent-complete",
				Operation: "subagent-stop",
			},
			expectedChainType: "simple",
			expectedPathCount: 4,
			description:       "SubagentStop should use 4-level simple fallback (no tool commands)",
		},
		{
			name:      "PreCompact events use 4-level simple fallback chain",
			eventName: "PreCompact",
			context: &hooks.EventContext{
				Category:  hooks.System,
				SoundHint: "compacting",
				Operation: "compact",
			},
			expectedChainType: "simple",
			expectedPathCount: 4,
			description:       "PreCompact should use 4-level simple fallback (no tool commands)",
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
	mapper := NewSoundMapper(nil)

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
		"loading/git-commit-start.wav", // 1. Exact hint match
		"loading/git-commit.wav",       // 2. Command-subcommand without suffix
		"loading/git-start.wav",        // 3. Command with suffix
		"loading/git.wav",              // 4. Command-only (command fallback)
		"loading/bash-start.wav",       // 5. Original tool with suffix
		"loading/bash.wav",             // 6. Original tool fallback
		"loading/tool-start.wav",       // 7. Operation-specific
		"loading/loading.wav",          // 8. Category-specific
		"default.wav",                  // 9. Default fallback
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
	mapper := NewSoundMapper(nil)

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
	mapper := NewSoundMapper(nil)

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
				"interactive/message-sent.wav",  // 1. Specific hint
				"interactive/prompt-submit.wav", // 2. Event-specific (not tool-based)
				"interactive/interactive.wav",   // 3. Category-specific
				"default.wav",                   // 4. Default fallback
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
				"system/compacting.wav",  // 1. Specific hint
				"system/pre-compact.wav", // 2. Event-specific
				"system/system.wav",      // 3. Category-specific
				"default.wav",            // 4. Default fallback
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

// TDD Cycle 6 RED: SoundChecker Integration Tests
func TestNewSoundMapperWithSoundChecker(t *testing.T) {
	checker := tracking.NewSoundChecker()
	mapper := NewSoundMapper(checker)

	if mapper == nil {
		t.Fatal("NewSoundMapper with SoundChecker returned nil")
	}
}

func TestNewSoundMapperWithNilChecker(t *testing.T) {
	mapper := NewSoundMapper(nil)

	if mapper == nil {
		t.Fatal("NewSoundMapper with nil checker returned nil")
	}
}

func TestMapSoundTriggersPathChecking(t *testing.T) {
	// Create a test hook to capture path checks
	var checkedPaths []string
	var checkedSequences []int
	var checkedExists []bool
	var checkedContext *hooks.EventContext

	testHook := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		checkedPaths = append(checkedPaths, path)
		checkedExists = append(checkedExists, exists)
		checkedSequences = append(checkedSequences, sequence)
		checkedContext = context
	}

	checker := tracking.NewSoundChecker(tracking.WithHook(testHook))
	mapper := NewSoundMapper(checker)

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		SoundHint: "file-saved",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context)

	// Verify that path checking was triggered
	if len(checkedPaths) == 0 {
		t.Error("Expected path checking to be triggered, but no paths were checked")
	}

	// Verify that all paths from the result were checked
	if len(checkedPaths) != len(result.AllPaths) {
		t.Errorf("Expected %d paths to be checked, got %d", len(result.AllPaths), len(checkedPaths))
	}

	// Verify paths match between result and checked paths
	for i, expectedPath := range result.AllPaths {
		if i < len(checkedPaths) && checkedPaths[i] != expectedPath {
			t.Errorf("Checked path[%d] = %s, expected %s", i, checkedPaths[i], expectedPath)
		}
	}

	// Verify sequences are 1-based and incremental
	for i, seq := range checkedSequences {
		expectedSeq := i + 1 // 1-based
		if seq != expectedSeq {
			t.Errorf("Sequence[%d] = %d, expected %d", i, seq, expectedSeq)
		}
	}

	// Verify context was passed correctly
	if checkedContext != context {
		t.Error("Expected original context to be passed to path checker")
	}
}

func TestMapSoundFallbackLevelBasedOnFileExistence(t *testing.T) {
	// Create a temporary directory structure for this test
	tempDir := t.TempDir()
	
	// Create the success subdirectory
	successDir := filepath.Join(tempDir, "success")
	err := os.MkdirAll(successDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}
	
	// Create only the second file (edit-success.wav) to simulate level 2 existing
	level2File := filepath.Join(successDir, "edit-success.wav")
	err = os.WriteFile(level2File, []byte("test audio data"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	// Change to the temp directory so relative paths work
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	var checkedPaths []string
	trackingHook := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		checkedPaths = append(checkedPaths, path)
		t.Logf("Path check: %s (exists: %v, sequence: %d)", path, exists, sequence)
	}

	checker := tracking.NewSoundChecker(tracking.WithHook(trackingHook))
	mapper := NewSoundMapper(checker)

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		SoundHint: "file-saved",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context)

	// Since level 1 doesn't exist but level 2 does, fallback level should be 2
	expectedFallbackLevel := 2
	if result.FallbackLevel != expectedFallbackLevel {
		t.Errorf("Expected fallback level %d, got %d", expectedFallbackLevel, result.FallbackLevel)
		t.Logf("All paths: %v", result.AllPaths)
		t.Logf("Checked paths: %v", checkedPaths)
	}

	// Selected path should be the second path (first existing one)
	if len(result.AllPaths) >= 2 {
		expectedSelectedPath := result.AllPaths[1] // Second path (level 2)
		if result.SelectedPath != expectedSelectedPath {
			t.Errorf("Expected selected path %s, got %s", expectedSelectedPath, result.SelectedPath)
		}
	} else {
		t.Error("Expected at least 2 paths in AllPaths")
	}

	// Verify all expected paths were checked
	if len(checkedPaths) != len(result.AllPaths) {
		t.Errorf("Expected %d path checks, got %d", len(result.AllPaths), len(checkedPaths))
	}
}

func TestMapSoundAllPathsChecked(t *testing.T) {
	var checkedPaths []string

	testHook := func(path string, exists bool, sequence int, context *hooks.EventContext) {
		checkedPaths = append(checkedPaths, path)
	}

	checker := tracking.NewSoundChecker(tracking.WithHook(testHook))
	mapper := NewSoundMapper(checker)

	// Test with enhanced 9-level fallback (PreToolUse)
	context := &hooks.EventContext{
		Category:     hooks.Loading,
		ToolName:     "git",
		OriginalTool: "Bash",
		SoundHint:    "git-commit-start",
		Operation:    "tool-start",
	}

	result := mapper.MapSound(context)

	// Verify all paths in the result were checked
	if len(checkedPaths) != len(result.AllPaths) {
		t.Errorf("Expected all %d paths to be checked, only checked %d", len(result.AllPaths), len(checkedPaths))
	}

	// Verify exact path matching
	for i, expectedPath := range result.AllPaths {
		if i < len(checkedPaths) && checkedPaths[i] != expectedPath {
			t.Errorf("Path check[%d]: expected %s, got %s", i, expectedPath, checkedPaths[i])
		}
	}

	// Should generate 9 paths for enhanced fallback
	expectedPathCount := 9
	if len(result.AllPaths) != expectedPathCount {
		t.Errorf("Expected %d paths for enhanced fallback, got %d", expectedPathCount, len(result.AllPaths))
	}
}

func TestMapSoundWithoutTrackingSkipsPathChecking(t *testing.T) {
	var pathCheckTriggered bool

	// This hook should never be called 
	_ = func(path string, exists bool, sequence int, context *hooks.EventContext) {
		pathCheckTriggered = true
	}

	// This should NOT trigger path checking
	mapper := NewSoundMapper(nil)

	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context)

	// Path checking should not have been triggered
	if pathCheckTriggered {
		t.Error("Expected no path checking for nil SoundChecker, but path checking was triggered")
	}

	// Should still return valid result
	if result == nil {
		t.Error("Expected valid result even without tracking")
	}

	if len(result.AllPaths) == 0 {
		t.Error("Expected paths to be generated even without tracking")
	}

	// Fallback level should be 1 (default behavior without actual checking)
	if result.FallbackLevel != 1 {
		t.Errorf("Expected fallback level 1 for no tracking, got %d", result.FallbackLevel)
	}
}

func TestSoundMapper_BugReproduction_AllPathsFallbackToDefault(t *testing.T) {
	// This test reproduces the original bug where all sounds fallback to default.wav
	// because SoundChecker can't find logical paths
	
	// Create temp files for realistic testing
	tempDir := t.TempDir()
	bashSuccessFile := filepath.Join(tempDir, "bash-success.wav")
	defaultFile := filepath.Join(tempDir, "default.wav")
	
	err := os.WriteFile(bashSuccessFile, []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(defaultFile, []byte("test"), 0644)
	require.NoError(t, err)
	
	// Mock resolver that maps specific sounds
	resolver := &MockSoundpackResolver{
		mappings: map[string]string{
			"success/bash-success.wav": bashSuccessFile, // This should be found
			"default.wav": defaultFile,
		},
		// Note: success/tool-complete.wav and success/success.wav are NOT mapped
	}
	
	mapper := NewSoundMapperWithResolver(resolver)
	
	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "bash",
		Operation: "tool-complete",
	}
	
	result := mapper.MapSound(context)
	
	// Should find bash-success.wav (level 1) instead of falling back to default.wav
	expectedPath := "success/bash-success.wav"
	if result.SelectedPath != expectedPath {
		t.Errorf("Expected selected path %s, got %s (fallback level %d)", 
			expectedPath, result.SelectedPath, result.FallbackLevel)
		t.Logf("All paths: %v", result.AllPaths)
	}
	
	// Should be fallback level 1 (first path found)
	if result.FallbackLevel != 1 {
		t.Errorf("Expected fallback level 1, got %d", result.FallbackLevel)
	}
}

func TestSoundMapper_BugReproduction_NoSpecificSoundFallsToDefault(t *testing.T) {
	// Test case where specific sound doesn't exist, should fallback properly
	
	tempDir := t.TempDir()
	defaultFile := filepath.Join(tempDir, "default.wav")
	
	err := os.WriteFile(defaultFile, []byte("test"), 0644)
	require.NoError(t, err)
	
	// Resolver only has default.wav, no specific sounds
	resolver := &MockSoundpackResolver{
		mappings: map[string]string{
			"default.wav": defaultFile,
		},
	}
	
	mapper := NewSoundMapperWithResolver(resolver)
	
	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "unknowntool",
		Operation: "tool-complete",
	}
	
	result := mapper.MapSound(context)
	
	// Should fallback to default.wav (last level)
	expectedPath := "default.wav"
	if result.SelectedPath != expectedPath {
		t.Errorf("Expected selected path %s, got %s", expectedPath, result.SelectedPath)
	}
	
	// Should be fallback level = len(paths) (last level)
	if result.FallbackLevel != len(result.AllPaths) {
		t.Errorf("Expected fallback level %d (last), got %d", len(result.AllPaths), result.FallbackLevel)
	}
}

// MockSoundpackResolver for testing (same as in types_test.go)
type MockSoundpackResolver struct {
	mappings map[string]string
}

func (m *MockSoundpackResolver) ResolveSound(relativePath string) (string, error) {
	if physical, exists := m.mappings[relativePath]; exists {
		return physical, nil
	}
	return "", fmt.Errorf("sound not found: %s", relativePath)
}

func (m *MockSoundpackResolver) ResolveSoundWithFallback(paths []string) (string, error) {
	for _, path := range paths {
		if resolved, err := m.ResolveSound(path); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("no sounds found in fallback chain")
}

func (m *MockSoundpackResolver) GetName() string { return "mock" }
func (m *MockSoundpackResolver) GetType() string { return "mock" }

