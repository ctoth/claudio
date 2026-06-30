package sounds

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
	"github.com/stretchr/testify/require"
)

// newTestResolver builds a real soundpack.SoundpackResolver backed by a
// JSONMapper. The mapping is logical-path → physical-absolute-path. Tests
// must create the physical files on disk (typically under t.TempDir()) so
// that os.Stat inside the resolver finds them.
//
// Using the real UnifiedSoundpackResolver keeps the observer-firing
// contract end-to-end — there's no hand-rolled mock to keep in sync with
// the production resolver's observer semantics.
func newTestResolver(mappings map[string]string) soundpack.SoundpackResolver {
	return soundpack.NewSoundpackResolver(soundpack.NewJSONMapper("test", mappings))
}

func TestSoundMapper(t *testing.T) {
	mapper := NewSoundMapper()

	if mapper == nil {
		t.Fatal("NewSoundMapper returned nil")
	}
}

func TestMapSoundSilentContextReturnsNil(t *testing.T) {
	mapper := NewSoundMapper()
	result := mapper.MapSound(context.Background(), &hooks.EventContext{
		Category:  hooks.Silent,
		Operation: "beforemodel",
	})
	if result != nil {
		t.Fatalf("silent context returned sound result: %#v", result)
	}
}

func TestMapSoundEventSpecificFallback(t *testing.T) {
	mapper := NewSoundMapper()

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
			result := mapper.MapSound(context.Background(), tc.context)

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
	mapper := NewSoundMapper()

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
			eventCtx := &hooks.EventContext{
				Category:  hooks.Loading,
				SoundHint: tc.input,
			}

			result := mapper.MapSound(context.Background(), eventCtx)
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
			result := mapper.MapSound(context.Background(), tc.context)

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
	mapper := NewSoundMapper()

	t.Run("nil context", func(t *testing.T) {
		result := mapper.MapSound(context.Background(), nil)

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
		result := mapper.MapSound(context.Background(), &hooks.EventContext{
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
		eventCtx := &hooks.EventContext{
			Category: hooks.EventCategory(999), // Invalid category
		}

		result := mapper.MapSound(context.Background(), eventCtx)

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

	eventCtx := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		SoundHint: "file-saved",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context.Background(), eventCtx)

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
	eventCtx := &hooks.EventContext{
		Category:     hooks.Success,
		ToolName:     "git",  // Extracted from Bash
		OriginalTool: "Bash", // Original tool for fallback
		SoundHint:    "git-commit-success",
		Operation:    "tool-complete",
	}

	result := mapper.MapSound(context.Background(), eventCtx)

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
	mapper := NewSoundMapper()

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
			result := mapper.MapSound(context.Background(), tt.context)

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

	eventCtx := &hooks.EventContext{
		Category:     hooks.Loading,
		ToolName:     "git",
		OriginalTool: "Bash",
		SoundHint:    "git-commit-start",
		Operation:    "tool-start",
	}

	result := mapper.MapSound(context.Background(), eventCtx)

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
			result := mapper.MapSound(context.Background(), tt.context)

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
			result := mapper.MapSound(context.Background(), tt.context)

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

// TestNewSoundMapperWithSoundChecker, TestNewSoundMapperWithNilChecker,
// TestMapSoundTriggersPathChecking, TestMapSoundFallbackLevelBasedOnFileExistence,
// TestMapSoundAllPathsChecked, TestMapSoundWithoutTrackingSkipsPathChecking
// — all deleted. They tested the streaming-hook ecosystem (PathCheckedHook,
// WithHook, NewSoundChecker no-resolver constructor) that was removed when
// tracking moved to the soundpack.PathObserver / LookupBuffer model. The
// observation contract those tests covered (1-based sequence, all paths
// checked, exists flag accuracy, context-passthrough) now lives in
// TestResolveWithObserver_* in soundpack and TestLookupBuffer_* in tracking.

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

	// Real resolver mapping logical → physical absolute paths. Anything not
	// in the map produces a MapPath miss, so the resolver walks the chain
	// the same way the production resolver does.
	resolver := newTestResolver(map[string]string{
		"success/bash-success.wav": bashSuccessFile, // This should be found
		"default.wav":              defaultFile,
		// Note: success/tool-complete.wav and success/success.wav are NOT mapped
	})

	mapper := NewSoundMapperWithResolver(resolver)

	eventCtx := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "bash",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context.Background(), eventCtx)

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
	resolver := newTestResolver(map[string]string{
		"default.wav": defaultFile,
	})

	mapper := NewSoundMapperWithResolver(resolver)

	eventCtx := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "unknowntool",
		Operation: "tool-complete",
	}

	result := mapper.MapSound(context.Background(), eventCtx)

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

// observerCall captures one PathObserver invocation for assertions.
type observerCall struct {
	path     string
	sequence int
	exists   bool
}

// captureObserver returns a thread-safe PathObserver that appends every
// callback into the provided slice (under the supplied mutex).
func captureObserver(mu *sync.Mutex, calls *[]observerCall) soundpack.PathObserver {
	return func(path string, sequence int, exists bool) {
		mu.Lock()
		*calls = append(*calls, observerCall{path: path, sequence: sequence, exists: exists})
		mu.Unlock()
	}
}

// TestMapSound_ObserverFiresOncePerChainCandidate locks in the contract
// that MapSound fires the wired PathObserver EXACTLY ONCE per deduped chain
// candidate, in order, with 1-based sequence. The observer stream is the
// telemetry source that LookupBuffer accumulates and the CLI hands to
// EventRecorder.RecordEvent in one transaction — so this invariant is the
// updated form of Chunk 13's "one RecordEvent per MapSound" guarantee.
//
// Covers all three chain types (enhanced, posttool, simple). Regression
// guard against streaming-style observation that would fire partial state
// during resolution.
func TestMapSound_ObserverFiresOncePerChainCandidate(t *testing.T) {
	tempDir := t.TempDir()
	defaultFile := filepath.Join(tempDir, "default.wav")
	if err := os.WriteFile(defaultFile, []byte("test"), 0644); err != nil {
		t.Fatalf("write default.wav: %v", err)
	}
	// Only default.wav is mapped — every other chain candidate produces a
	// MapPath miss, so the resolver walks the entire chain and the observer
	// stream is fully populated for the dedup invariant to chew on.
	resolver := newTestResolver(map[string]string{
		"default.wav": defaultFile,
	})

	tests := []struct {
		name              string
		eventCtx          *hooks.EventContext
		expectedChainType string
	}{
		{
			name: "enhanced chain (PreToolUse with tool)",
			eventCtx: &hooks.EventContext{
				Category:     hooks.Loading,
				ToolName:     "Bash",
				OriginalTool: "Bash",
				SoundHint:    "bash-thinking",
				Operation:    "tool-start",
			},
			expectedChainType: ChainTypeEnhanced,
		},
		{
			name: "posttool chain (PostToolUse success)",
			eventCtx: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Edit",
				Operation: "tool-complete",
				IsSuccess: true,
			},
			expectedChainType: ChainTypePostTool,
		},
		{
			name: "simple chain (Interactive event)",
			eventCtx: &hooks.EventContext{
				Category:  hooks.Interactive,
				Operation: "prompt",
			},
			expectedChainType: ChainTypeSimple,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			var calls []observerCall
			mapper := NewSoundMapperWithResolver(
				resolver,
				WithObserver(captureObserver(&mu, &calls)),
			)

			result := mapper.MapSound(context.Background(), tc.eventCtx)
			if result == nil {
				t.Fatal("MapSound returned nil result")
			}

			// Chain type tag matches the routed method.
			if result.ChainType != tc.expectedChainType {
				t.Errorf("chainType: got %q, want %q", result.ChainType, tc.expectedChainType)
			}

			mu.Lock()
			defer mu.Unlock()

			// Observer fires once per candidate; calls list must be non-empty.
			if len(calls) == 0 {
				t.Fatal("observer received zero callbacks; expected one per candidate")
			}

			// SelectedPath must be set; the chain must include it.
			if result.SelectedPath == "" {
				t.Error("SelectedPath empty after resolution")
			}
			selectedFound := false
			for _, c := range calls {
				if c.path == result.SelectedPath {
					selectedFound = true
					break
				}
			}
			if !selectedFound {
				t.Errorf("selectedPath %q not in observer call paths", result.SelectedPath)
			}

			// Dedup invariant: every observed path is unique. PostTool chains
			// can synthesize duplicates (hint == tool-suffix); the mapper
			// dedupes BEFORE resolution so the observer stream and the
			// downstream UNIQUE(event_id, path) constraint stay aligned.
			seen := make(map[string]bool)
			for _, c := range calls {
				if seen[c.path] {
					t.Errorf("duplicate path in observer calls: %q (dedup invariant violated)", c.path)
				}
				seen[c.path] = true
			}

			// Observer stream length == AllPaths length: the resolver walks
			// the full deduped chain (post-winner candidates fire with
			// exists=false to preserve telemetry shape).
			if len(calls) != len(result.AllPaths) {
				t.Errorf("observer calls=%d, AllPaths=%d (want equal)",
					len(calls), len(result.AllPaths))
			} else {
				for i, c := range calls {
					if result.AllPaths[i] != c.path {
						t.Errorf("AllPaths[%d]=%q != calls[%d].path=%q",
							i, result.AllPaths[i], i, c.path)
					}
				}
			}

			// Sequences are 1-based and contiguous.
			for i, c := range calls {
				if c.sequence != i+1 {
					t.Errorf("calls[%d].sequence=%d, want %d", i, c.sequence, i+1)
				}
			}
		})
	}
}

// TestMapSound_NilObserver_NoPanic pins that a mapper with no observer
// configured (WithObserver not used) never panics and resolves normally.
// Observation is optional.
func TestMapSound_NilObserver_NoPanic(t *testing.T) {
	tempDir := t.TempDir()
	defaultFile := filepath.Join(tempDir, "default.wav")
	if err := os.WriteFile(defaultFile, []byte("test"), 0644); err != nil {
		t.Fatalf("write default.wav: %v", err)
	}
	resolver := newTestResolver(map[string]string{"default.wav": defaultFile})

	// No WithObserver option.
	mapper := NewSoundMapperWithResolver(resolver)

	result := mapper.MapSound(context.Background(), &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Edit",
		Operation: "tool-complete",
	})
	if result == nil {
		t.Fatal("MapSound returned nil result")
	}
	if result.SelectedPath == "" {
		t.Error("SelectedPath should be set even without an observer")
	}
}
