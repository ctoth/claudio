package sounds

import (
	"testing"

	"claudio/internal/hooks"
)

func TestSoundMapper(t *testing.T) {
	mapper := NewSoundMapper()

	if mapper == nil {
		t.Fatal("NewSoundMapper returned nil")
	}
}

func TestMapSoundCategories(t *testing.T) {
	mapper := NewSoundMapper()

	testCases := []struct {
		name           string
		context        *hooks.EventContext
		expectedPaths  []string // in priority order (6-level fallback)
	}{
		{
			name: "bash thinking (loading)",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "Bash",
				SoundHint: "bash-thinking",
				Operation: "tool-start",
			},
			expectedPaths: []string{
				"loading/bash-thinking.wav",      // Level 1: exact hint
				"loading/bash.wav",               // Level 2: tool-specific
				"loading/tool-start.wav",         // Level 3: operation-specific
				"loading/loading.wav",            // Level 4: category-specific
				"default.wav",                    // Level 5: default
				"",                              // Level 6: silent
			},
		},
		{
			name: "bash success",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Bash",
				SoundHint: "bash-success",
				Operation: "tool-complete",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/bash-success.wav",
				"success/bash.wav",
				"success/tool-complete.wav",
				"success/success.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "tool interrupted (error)",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				ToolName:  "Bash",
				SoundHint: "tool-interrupted",
				Operation: "tool-complete",
				HasError:  true,
			},
			expectedPaths: []string{
				"error/tool-interrupted.wav",
				"error/bash.wav",
				"error/tool-complete.wav",
				"error/error.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "message sent (interactive)",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "message-sent",
				Operation: "prompt",
			},
			expectedPaths: []string{
				"interactive/message-sent.wav",
				"interactive/prompt.wav",
				"interactive/interactive.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "notification",
			context: &hooks.EventContext{
				Category:  hooks.Interactive,
				SoundHint: "notification",
				Operation: "notification",
			},
			expectedPaths: []string{
				"interactive/notification.wav",
				"interactive/notification.wav", // operation == hint for notifications
				"interactive/interactive.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "file operations with type context",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				ToolName:  "Edit",
				SoundHint: "edit-success",
				Operation: "tool-complete",
				FileType:  "go",
				IsSuccess: true,
			},
			expectedPaths: []string{
				"success/edit-success.wav",
				"success/edit.wav",
				"success/tool-complete.wav",
				"success/success.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "unknown tool fallback",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "UnknownTool",
				SoundHint: "unknowntool-thinking",
				Operation: "tool-start",
			},
			expectedPaths: []string{
				"loading/unknowntool-thinking.wav",
				"loading/unknowntool.wav",
				"loading/tool-start.wav",
				"loading/loading.wav",
				"default.wav",
				"",
			},
		},
		{
			name: "empty context defaults",
			context: &hooks.EventContext{
				Category: hooks.Interactive,
			},
			expectedPaths: []string{
				"interactive/interactive.wav",
				"default.wav",
				"",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := mapper.GetSoundPaths(tc.context)

			if len(paths) != len(tc.expectedPaths) {
				t.Errorf("Expected %d paths, got %d", len(tc.expectedPaths), len(paths))
				return
			}

			for i, expected := range tc.expectedPaths {
				if paths[i] != expected {
					t.Errorf("Path[%d] = %s, expected %s", i, paths[i], expected)
				}
			}
		})
	}
}

func TestMapSoundPathNormalization(t *testing.T) {
	mapper := NewSoundMapper()

	testCases := []struct {
		name     string
		context  *hooks.EventContext
		checkIdx int
		expected string
	}{
		{
			name: "tool name normalization",
			context: &hooks.EventContext{
				Category:  hooks.Loading,
				ToolName:  "MultiEdit",
				SoundHint: "multiedit-thinking",
			},
			checkIdx: 1, // Level 2: tool-specific
			expected: "loading/multiedit.wav",
		},
		{
			name: "sound hint normalization",
			context: &hooks.EventContext{
				Category:  hooks.Error,
				SoundHint: "Tool-Interrupted",
			},
			checkIdx: 0, // Level 1: exact hint
			expected: "error/tool-interrupted.wav",
		},
		{
			name: "operation normalization",
			context: &hooks.EventContext{
				Category:  hooks.Success,
				Operation: "Tool-Complete",
			},
			checkIdx: 0, // Level 3: operation-specific (first since no hint/tool)
			expected: "success/tool-complete.wav",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			paths := mapper.GetSoundPaths(tc.context)

			if tc.checkIdx >= len(paths) {
				t.Fatalf("Path index %d out of range, only %d paths", tc.checkIdx, len(paths))
			}

			if paths[tc.checkIdx] != tc.expected {
				t.Errorf("Path[%d] = %s, expected %s", tc.checkIdx, paths[tc.checkIdx], tc.expected)
			}
		})
	}
}

func TestMapSoundWithoutToolName(t *testing.T) {
	mapper := NewSoundMapper()

	// Test cases where tool name is empty (should skip level 2)
	context := &hooks.EventContext{
		Category:  hooks.Loading,
		SoundHint: "tool-loading",
		Operation: "tool-start",
	}

	paths := mapper.GetSoundPaths(context)

	expectedPaths := []string{
		"loading/tool-loading.wav",    // Level 1: exact hint
		"loading/tool-start.wav",      // Level 3: operation (skipping level 2)
		"loading/loading.wav",         // Level 4: category
		"default.wav",                 // Level 5: default
		"",                           // Level 6: silent
	}

	if len(paths) != len(expectedPaths) {
		t.Errorf("Expected %d paths, got %d", len(expectedPaths), len(paths))
		return
	}

	for i, expected := range expectedPaths {
		if paths[i] != expected {
			t.Errorf("Path[%d] = %s, expected %s", i, paths[i], expected)
		}
	}
}

func TestMapSoundWithoutOperation(t *testing.T) {
	mapper := NewSoundMapper()

	// Test cases where operation is empty (should skip level 3)
	context := &hooks.EventContext{
		Category:  hooks.Success,
		ToolName:  "Read",
		SoundHint: "read-success",
	}

	paths := mapper.GetSoundPaths(context)

	expectedPaths := []string{
		"success/read-success.wav", // Level 1: exact hint
		"success/read.wav",         // Level 2: tool-specific
		"success/success.wav",      // Level 4: category (skipping level 3)
		"default.wav",              // Level 5: default
		"",                        // Level 6: silent
	}

	if len(paths) != len(expectedPaths) {
		t.Errorf("Expected %d paths, got %d", len(expectedPaths), len(paths))
		return
	}

	for i, expected := range expectedPaths {
		if paths[i] != expected {
			t.Errorf("Path[%d] = %s, expected %s", i, paths[i], expected)
		}
	}
}

func TestMapSoundMinimalContext(t *testing.T) {
	mapper := NewSoundMapper()

	// Test with minimal context (only category)
	context := &hooks.EventContext{
		Category: hooks.Error,
	}

	paths := mapper.GetSoundPaths(context)

	expectedPaths := []string{
		"error/error.wav", // Level 4: category only
		"default.wav",     // Level 5: default
		"",               // Level 6: silent
	}

	if len(paths) != len(expectedPaths) {
		t.Errorf("Expected %d paths, got %d", len(expectedPaths), len(paths))
		return
	}

	for i, expected := range expectedPaths {
		if paths[i] != expected {
			t.Errorf("Path[%d] = %s, expected %s", i, paths[i], expected)
		}
	}
}

func TestMapSoundCategoryStrings(t *testing.T) {
	mapper := NewSoundMapper()

	testCases := []struct {
		category hooks.EventCategory
		expected string
	}{
		{hooks.Loading, "loading"},
		{hooks.Success, "success"},
		{hooks.Error, "error"},
		{hooks.Interactive, "interactive"},
	}

	for _, tc := range testCases {
		context := &hooks.EventContext{
			Category:  tc.category,
			SoundHint: "test-hint",
		}

		paths := mapper.GetSoundPaths(context)

		if len(paths) < 2 {
			t.Fatalf("Expected at least 2 paths for category %v", tc.category)
		}

		expectedPath := tc.expected + "/test-hint.wav"
		if paths[0] != expectedPath {
			t.Errorf("First path = %s, expected %s", paths[0], expectedPath)
		}
	}
}