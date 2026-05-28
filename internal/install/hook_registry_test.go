package install

import (
	"testing"

	"claudio.click/internal/hooks"
)

func TestGetAllHooks(t *testing.T) {
	// TDD RED: Test that GetAllHooks returns all 8 expected hooks
	allHooks := GetAllHooks()

	expectedCount := 8
	if len(allHooks) != expectedCount {
		t.Errorf("Expected %d hooks, got %d", expectedCount, len(allHooks))
	}

	// Verify all expected hook names are present
	expectedNames := []string{
		"PreToolUse", "PostToolUse", "UserPromptSubmit",
		"Notification", "Stop", "SubagentStop", "PreCompact", "SessionStart",
	}

	hookMap := make(map[string]bool)
	for _, hook := range allHooks {
		hookMap[hook.Name] = true
	}

	for _, expectedName := range expectedNames {
		if !hookMap[expectedName] {
			t.Errorf("Expected hook '%s' not found in registry", expectedName)
		}
	}
}

func TestGetEnabledHooks(t *testing.T) {
	// TDD RED: Test that GetEnabledHooks filters correctly
	enabledHooks := GetEnabledHooks()

	// All hooks should be enabled by default
	expectedCount := 8
	if len(enabledHooks) != expectedCount {
		t.Errorf("Expected %d enabled hooks, got %d", expectedCount, len(enabledHooks))
	}

	// Verify all returned hooks have DefaultEnabled = true
	for _, hook := range enabledHooks {
		if !hook.DefaultEnabled {
			t.Errorf("Hook '%s' returned by GetEnabledHooks but DefaultEnabled is false", hook.Name)
		}
	}
}

func TestGetHookByName(t *testing.T) {
	// TDD RED: Test hook lookup functionality
	testCases := []struct {
		name        string
		shouldExist bool
	}{
		{"PreToolUse", true},
		{"PostToolUse", true},
		{"UserPromptSubmit", true},
		{"Notification", true},
		{"Stop", true},
		{"SubagentStop", true},
		{"PreCompact", true},
		{"SessionStart", true},
		{"NonExistentHook", false},
	}

	for _, tc := range testCases {
		hook, found := GetHookByName(tc.name)

		if tc.shouldExist {
			if !found {
				t.Errorf("Expected to find hook '%s' but it was not found", tc.name)
			}
			if hook.Name != tc.name {
				t.Errorf("Expected hook name '%s', got '%s'", tc.name, hook.Name)
			}
		} else {
			if found {
				t.Errorf("Expected hook '%s' to not exist, but it was found", tc.name)
			}
		}
	}
}

func TestGetHookNames(t *testing.T) {
	// TDD RED: Test that GetHookNames returns correct slice of names
	hookNames := GetHookNames()

	expectedNames := []string{
		"PreToolUse", "PostToolUse", "UserPromptSubmit",
		"Notification", "Stop", "SubagentStop", "PreCompact", "SessionStart",
	}

	if len(hookNames) != len(expectedNames) {
		t.Errorf("Expected %d hook names, got %d", len(expectedNames), len(hookNames))
	}

	nameMap := make(map[string]bool)
	for _, name := range hookNames {
		nameMap[name] = true
	}

	for _, expectedName := range expectedNames {
		if !nameMap[expectedName] {
			t.Errorf("Expected hook name '%s' not found", expectedName)
		}
	}
}

func TestHookCategoriesMatchParser(t *testing.T) {
	// TDD RED: Test that hook categories match parser expectations
	allHooks := GetAllHooks()

	expectedCategories := map[string]hooks.EventCategory{
		"PreToolUse":       hooks.Loading,
		"PostToolUse":      hooks.Success, // Note: PostToolUse can be Success or Error, using Success as default
		"UserPromptSubmit": hooks.Interactive,
		"Notification":     hooks.Interactive,
		"Stop":             hooks.Completion,
		"SubagentStop":     hooks.Completion,
		"PreCompact":       hooks.System,
		"SessionStart":     hooks.System,
	}

	for _, hook := range allHooks {
		expectedCategory, exists := expectedCategories[hook.Name]
		if !exists {
			t.Errorf("No expected category defined for hook '%s'", hook.Name)
			continue
		}

		if hook.Category != expectedCategory {
			t.Errorf("Hook '%s' has category %v, expected %v",
				hook.Name, hook.Category, expectedCategory)
		}
	}
}

func TestHookDescriptionsNonEmpty(t *testing.T) {
	// TDD RED: Test that all hook descriptions are non-empty
	allHooks := GetAllHooks()

	for _, hook := range allHooks {
		if hook.Description == "" {
			t.Errorf("Hook '%s' has empty description", hook.Name)
		}
	}
}

func TestDefaultEnabledStatus(t *testing.T) {
	// TDD RED: Test that all hooks are enabled by default
	allHooks := GetAllHooks()

	for _, hook := range allHooks {
		if !hook.DefaultEnabled {
			t.Errorf("Hook '%s' is not enabled by default", hook.Name)
		}
	}
}

func TestCodexRegistryContents(t *testing.T) {
	want := map[string]bool{
		"PreToolUse": true, "PostToolUse": true, "UserPromptSubmit": true,
		"Stop": true, "SubagentStop": true, "SubagentStart": true,
		"PreCompact": true, "PostCompact": true, "SessionStart": true,
		"PermissionRequest": true,
	}
	got := map[string]bool{}
	for _, h := range CodexHooks {
		got[h.Name] = true
	}
	if len(got) != len(want) {
		t.Errorf("codex registry has %d events, want %d", len(got), len(want))
	}
	for name := range want {
		if !got[name] {
			t.Errorf("codex registry missing %q", name)
		}
	}
	if got["Notification"] || got["SessionEnd"] {
		t.Error("codex registry must not contain Notification or SessionEnd")
	}
}

func TestAgentEnabledHooksAndNames(t *testing.T) {
	if len(AgentCodex.EnabledHooks()) != len(CodexHooks) {
		t.Errorf("expected all codex hooks enabled by default")
	}
	if len(AgentCodex.HookNames()) != 10 {
		t.Errorf("expected 10 codex hook names, got %d", len(AgentCodex.HookNames()))
	}
	if len(AgentClaude.HookNames()) != len(AllHooks) {
		t.Errorf("claude hook names mismatch")
	}
}
