package install

import (
	"testing"

	"claudio.click/internal/hooks"
)

func TestGetAllHooks(t *testing.T) {
	allHooks := GetAllHooks()

	expectedCount := len(expectedClaudeHookNames())
	if len(allHooks) != expectedCount {
		t.Errorf("Expected %d hooks, got %d", expectedCount, len(allHooks))
	}

	// Verify all expected hook names are present
	expectedNames := expectedClaudeHookNames()

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
	enabledHooks := GetEnabledHooks()

	expectedCount := len(expectedClaudeHookNames()) - len(defaultDisabledClaudeHookNames())
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
		{"SessionEnd", true},
		{"PostToolUseFailure", true},
		{"PostToolBatch", true},
		{"PermissionRequest", true},
		{"PermissionDenied", true},
		{"PostCompact", true},
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
	hookNames := GetHookNames()

	expectedNames := expectedClaudeHookNames()

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
	allHooks := GetAllHooks()

	expectedCategories := map[string]hooks.EventCategory{
		"PreToolUse":          hooks.Loading,
		"PostToolUse":         hooks.Success, // Note: PostToolUse can be Success or Error, using Success as default
		"PostToolUseFailure":  hooks.Error,
		"PostToolBatch":       hooks.Success,
		"UserPromptSubmit":    hooks.Interactive,
		"UserPromptExpansion": hooks.Interactive,
		"Notification":        hooks.Interactive,
		"MessageDisplay":      hooks.Silent,
		"Stop":                hooks.Completion,
		"StopFailure":         hooks.Error,
		"PermissionRequest":   hooks.Interactive,
		"PermissionDenied":    hooks.Error,
		"Setup":               hooks.System,
		"SubagentStart":       hooks.Loading,
		"SubagentStop":        hooks.Completion,
		"TaskCreated":         hooks.Loading,
		"TaskCompleted":       hooks.Completion,
		"TeammateIdle":        hooks.Interactive,
		"InstructionsLoaded":  hooks.System,
		"ConfigChange":        hooks.System,
		"CwdChanged":          hooks.System,
		"FileChanged":         hooks.System,
		"WorktreeCreate":      hooks.System,
		"WorktreeRemove":      hooks.System,
		"PreCompact":          hooks.System,
		"PostCompact":         hooks.System,
		"SessionStart":        hooks.System,
		"SessionEnd":          hooks.Interactive,
		"Elicitation":         hooks.Interactive,
		"ElicitationResult":   hooks.Interactive,
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
	allHooks := GetAllHooks()
	disabled := defaultDisabledClaudeHookNames()

	for _, hook := range allHooks {
		if disabled[hook.Name] {
			if hook.DefaultEnabled {
				t.Errorf("Hook '%s' should be disabled by default", hook.Name)
			}
			continue
		}
		if !hook.DefaultEnabled {
			t.Errorf("Hook '%s' is not enabled by default", hook.Name)
		}
	}
}

func TestClaudeRegistryTracksCurrentHookEvents(t *testing.T) {
	want := map[string]bool{}
	for _, name := range expectedClaudeHookNames() {
		want[name] = true
	}
	got := map[string]bool{}
	for _, h := range AllHooks {
		got[h.Name] = true
	}
	if len(got) != len(want) {
		t.Errorf("claude registry has %d events, want %d", len(got), len(want))
	}
	for name := range want {
		if !got[name] {
			t.Errorf("claude registry missing %q", name)
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

func TestQwenRegistryContents(t *testing.T) {
	want := map[string]bool{
		"PreToolUse": true, "PostToolUse": true, "PostToolUseFailure": true,
		"UserPromptSubmit": true, "SessionStart": true, "SessionEnd": true,
		"Stop": true, "StopFailure": true, "SubagentStart": true,
		"SubagentStop": true, "PreCompact": true, "PostCompact": true,
		"Notification": true, "PermissionRequest": true,
		"TodoCreated": true, "TodoCompleted": true,
	}
	got := map[string]bool{}
	for _, h := range QwenHooks {
		got[h.Name] = true
	}
	if len(got) != len(want) {
		t.Errorf("qwen registry has %d events, want %d", len(got), len(want))
	}
	for name := range want {
		if !got[name] {
			t.Errorf("qwen registry missing %q", name)
		}
	}
}

func expectedClaudeHookNames() []string {
	return []string{
		"SessionStart",
		"Setup",
		"UserPromptSubmit",
		"UserPromptExpansion",
		"PreToolUse",
		"PermissionRequest",
		"PermissionDenied",
		"PostToolUse",
		"PostToolUseFailure",
		"PostToolBatch",
		"Notification",
		"MessageDisplay",
		"SubagentStart",
		"SubagentStop",
		"TaskCreated",
		"TaskCompleted",
		"Stop",
		"StopFailure",
		"TeammateIdle",
		"InstructionsLoaded",
		"ConfigChange",
		"CwdChanged",
		"FileChanged",
		"WorktreeCreate",
		"WorktreeRemove",
		"PreCompact",
		"PostCompact",
		"Elicitation",
		"ElicitationResult",
		"SessionEnd",
	}
}

func defaultDisabledClaudeHookNames() map[string]bool {
	return map[string]bool{
		"FileChanged":    true,
		"MessageDisplay": true,
	}
}

func enabledHookNames(agent Agent) []string {
	definitions := agent.EnabledHooks()
	names := make([]string, len(definitions))
	for i, definition := range definitions {
		names[i] = definition.Name
	}
	return names
}

func TestCopilotRegistryContents(t *testing.T) {
	want := map[string]bool{
		"PreToolUse": true, "PostToolUse": true, "PostToolUseFailure": true,
		"UserPromptSubmit": true, "SessionStart": true, "SessionEnd": true,
		"Stop": true, "SubagentStop": true, "PreCompact": true,
		"subagentStart": true, "Notification": true, "PermissionRequest": true,
		"ErrorOccurred": true,
	}
	got := map[string]bool{}
	for _, h := range CopilotHooks {
		got[h.Name] = true
	}
	if len(got) != len(want) {
		t.Errorf("copilot registry has %d events, want %d", len(got), len(want))
	}
	for name := range want {
		if !got[name] {
			t.Errorf("copilot registry missing %q", name)
		}
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
	if len(AgentQwen.HookNames()) != len(QwenHooks) {
		t.Errorf("qwen hook names mismatch")
	}
	if len(AgentCopilot.HookNames()) != len(CopilotHooks) {
		t.Errorf("copilot hook names mismatch")
	}
}
