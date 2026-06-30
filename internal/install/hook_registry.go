package install

import (
	"log/slog"

	"claudio.click/internal/hooks"
)

// HookDefinition represents a single hook configuration in the registry
type HookDefinition struct {
	Name           string              // Hook name (e.g., "PreToolUse")
	Category       hooks.EventCategory // Event category from parser
	Description    string              // Human-readable description
	DefaultEnabled bool                // Whether enabled by default
}

// AllHooks defines the complete registry of Claude Code hooks supported by Claudio.
// Chatty streaming or watch-style hooks stay in the registry but are disabled
// by default so install does not create noisy audio loops.
var AllHooks = []HookDefinition{
	{
		Name:           "SessionStart",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code session starts or resumes",
		DefaultEnabled: true,
	},
	{
		Name:           "Setup",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code setup-only mode runs",
		DefaultEnabled: true,
	},
	{
		Name:           "UserPromptSubmit",
		Category:       hooks.Interactive,
		Description:    "Play interaction sounds when user submits prompts",
		DefaultEnabled: true,
	},
	{
		Name:           "UserPromptExpansion",
		Category:       hooks.Interactive,
		Description:    "Play interaction sounds when slash commands expand into prompts",
		DefaultEnabled: true,
	},
	{
		Name:           "PreToolUse",
		Category:       hooks.Loading,
		Description:    "Play loading/thinking sounds before tool execution",
		DefaultEnabled: true,
	},
	{
		Name:           "PermissionRequest",
		Category:       hooks.Interactive,
		Description:    "Play sounds when Claude Code asks for tool permission",
		DefaultEnabled: true,
	},
	{
		Name:           "PermissionDenied",
		Category:       hooks.Error,
		Description:    "Play error sounds when a permission request is denied",
		DefaultEnabled: true,
	},
	{
		Name:           "PostToolUse",
		Category:       hooks.Success, // Note: Can also be Error category depending on outcome
		Description:    "Play success/error sounds after tool execution",
		DefaultEnabled: true,
	},
	{
		Name:           "PostToolUseFailure",
		Category:       hooks.Error,
		Description:    "Play error sounds after failed tool execution",
		DefaultEnabled: true,
	},
	{
		Name:           "PostToolBatch",
		Category:       hooks.Success,
		Description:    "Play sounds after a batch of tool calls completes",
		DefaultEnabled: true,
	},
	{
		Name:           "Notification",
		Category:       hooks.Interactive,
		Description:    "Play sounds for permission requests and idle notifications",
		DefaultEnabled: true,
	},
	{
		Name:           "MessageDisplay",
		Category:       hooks.Silent,
		Description:    "Keep display-stream hook registered but silent by default",
		DefaultEnabled: false,
	},
	{
		Name:           "SubagentStart",
		Category:       hooks.Loading,
		Description:    "Play sounds when a Task tool subagent starts",
		DefaultEnabled: true,
	},
	{
		Name:           "SubagentStop",
		Category:       hooks.Completion,
		Description:    "Play sounds when Task tool subagent finishes responding",
		DefaultEnabled: true,
	},
	{
		Name:           "TaskCreated",
		Category:       hooks.Loading,
		Description:    "Play sounds when Claude Code creates a task",
		DefaultEnabled: true,
	},
	{
		Name:           "TaskCompleted",
		Category:       hooks.Completion,
		Description:    "Play sounds when Claude Code completes a task",
		DefaultEnabled: true,
	},
	{
		Name:           "Stop",
		Category:       hooks.Completion,
		Description:    "Play sounds when main Claude agent finishes responding",
		DefaultEnabled: true,
	},
	{
		Name:           "StopFailure",
		Category:       hooks.Error,
		Description:    "Play error sounds when Claude Code fails to stop cleanly",
		DefaultEnabled: true,
	},
	{
		Name:           "TeammateIdle",
		Category:       hooks.Interactive,
		Description:    "Play sounds when Claude Code reports teammate idle state",
		DefaultEnabled: true,
	},
	{
		Name:           "InstructionsLoaded",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code loads instructions",
		DefaultEnabled: true,
	},
	{
		Name:           "ConfigChange",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code configuration changes",
		DefaultEnabled: true,
	},
	{
		Name:           "CwdChanged",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code changes working directory",
		DefaultEnabled: true,
	},
	{
		Name:           "FileChanged",
		Category:       hooks.System,
		Description:    "Keep file-watch hook available without enabling broad file-change audio",
		DefaultEnabled: false,
	},
	{
		Name:           "WorktreeCreate",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code creates a worktree",
		DefaultEnabled: true,
	},
	{
		Name:           "WorktreeRemove",
		Category:       hooks.System,
		Description:    "Play sounds when Claude Code removes a worktree",
		DefaultEnabled: true,
	},
	{
		Name:           "PreCompact",
		Category:       hooks.System,
		Description:    "Play sounds before Claude Code context compression",
		DefaultEnabled: true,
	},
	{
		Name:           "PostCompact",
		Category:       hooks.System,
		Description:    "Play sounds after Claude Code context compression",
		DefaultEnabled: true,
	},
	{
		Name:           "Elicitation",
		Category:       hooks.Interactive,
		Description:    "Play sounds when Claude Code requests elicitation",
		DefaultEnabled: true,
	},
	{
		Name:           "ElicitationResult",
		Category:       hooks.Interactive,
		Description:    "Play sounds when Claude Code receives elicitation result",
		DefaultEnabled: true,
	},
	{
		Name:           "SessionEnd",
		Category:       hooks.Interactive,
		Description:    "Play sounds when Claude Code session ends",
		DefaultEnabled: true,
	},
}

// CodexHooks defines the registry of OpenAI Codex CLI hooks supported by Claudio.
// Codex lacks Notification and SessionEnd; it adds SubagentStart and PostCompact.
var CodexHooks = []HookDefinition{
	{Name: "PreToolUse", Category: hooks.Loading, Description: "Play loading sounds before Codex tool execution", DefaultEnabled: true},
	{Name: "PostToolUse", Category: hooks.Success, Description: "Play success/error sounds after Codex tool execution", DefaultEnabled: true},
	{Name: "UserPromptSubmit", Category: hooks.Interactive, Description: "Play interaction sounds when user submits prompts", DefaultEnabled: true},
	{Name: "Stop", Category: hooks.Completion, Description: "Play sounds when Codex finishes responding", DefaultEnabled: true},
	{Name: "SubagentStop", Category: hooks.Completion, Description: "Play sounds when a Codex subagent finishes", DefaultEnabled: true},
	{Name: "SubagentStart", Category: hooks.Loading, Description: "Play sounds when a Codex subagent starts", DefaultEnabled: true},
	{Name: "PreCompact", Category: hooks.System, Description: "Play sounds before Codex context compaction", DefaultEnabled: true},
	{Name: "PostCompact", Category: hooks.System, Description: "Play sounds after Codex context compaction", DefaultEnabled: true},
	{Name: "SessionStart", Category: hooks.System, Description: "Play sounds when a Codex session starts or resumes", DefaultEnabled: true},
	{Name: "PermissionRequest", Category: hooks.Interactive, Description: "Play sounds for Codex permission requests", DefaultEnabled: true},
}

// GeminiHooks defines the registry of Gemini CLI hooks supported by Claudio.
var GeminiHooks = []HookDefinition{
	{Name: "BeforeTool", Category: hooks.Loading, Description: "Play loading sounds before Gemini tool execution", DefaultEnabled: true},
	{Name: "AfterTool", Category: hooks.Success, Description: "Play success/error sounds after Gemini tool execution", DefaultEnabled: true},
	{Name: "BeforeAgent", Category: hooks.Interactive, Description: "Play interaction sounds when a Gemini prompt starts", DefaultEnabled: true},
	{Name: "AfterAgent", Category: hooks.Completion, Description: "Play sounds when Gemini finishes responding", DefaultEnabled: true},
	{Name: "BeforeModel", Category: hooks.Silent, Description: "Install no-op Gemini hook before model requests", DefaultEnabled: true},
	{Name: "AfterModel", Category: hooks.Silent, Description: "Install no-op Gemini hook after model responses", DefaultEnabled: true},
	{Name: "BeforeToolSelection", Category: hooks.Silent, Description: "Install no-op Gemini hook before tool selection", DefaultEnabled: true},
	{Name: "SessionStart", Category: hooks.System, Description: "Play sounds when a Gemini session starts or resumes", DefaultEnabled: true},
	{Name: "SessionEnd", Category: hooks.Interactive, Description: "Play sounds when a Gemini session ends", DefaultEnabled: true},
	{Name: "Notification", Category: hooks.Interactive, Description: "Play sounds for Gemini notifications", DefaultEnabled: true},
	{Name: "PreCompress", Category: hooks.System, Description: "Play sounds before Gemini context compression", DefaultEnabled: true},
}

// QwenHooks defines the registry of Qwen Code hooks supported by Claudio.
var QwenHooks = []HookDefinition{
	{Name: "PreToolUse", Category: hooks.Loading, Description: "Play loading sounds before Qwen tool execution", DefaultEnabled: true},
	{Name: "PostToolUse", Category: hooks.Success, Description: "Play success sounds after Qwen tool execution", DefaultEnabled: true},
	{Name: "PostToolUseFailure", Category: hooks.Error, Description: "Play error sounds after failed Qwen tool execution", DefaultEnabled: true},
	{Name: "UserPromptSubmit", Category: hooks.Interactive, Description: "Play interaction sounds when user submits prompts", DefaultEnabled: true},
	{Name: "SessionStart", Category: hooks.System, Description: "Play sounds when a Qwen session starts or resumes", DefaultEnabled: true},
	{Name: "SessionEnd", Category: hooks.Interactive, Description: "Play sounds when a Qwen session ends", DefaultEnabled: true},
	{Name: "Stop", Category: hooks.Completion, Description: "Play sounds when Qwen finishes responding", DefaultEnabled: true},
	{Name: "StopFailure", Category: hooks.Error, Description: "Play sounds when Qwen fails to stop cleanly", DefaultEnabled: true},
	{Name: "SubagentStart", Category: hooks.Loading, Description: "Play sounds when a Qwen subagent starts", DefaultEnabled: true},
	{Name: "SubagentStop", Category: hooks.Completion, Description: "Play sounds when a Qwen subagent finishes", DefaultEnabled: true},
	{Name: "PreCompact", Category: hooks.System, Description: "Play sounds before Qwen context compaction", DefaultEnabled: true},
	{Name: "PostCompact", Category: hooks.System, Description: "Play sounds after Qwen context compaction", DefaultEnabled: true},
	{Name: "Notification", Category: hooks.Interactive, Description: "Play sounds for Qwen notifications", DefaultEnabled: true},
	{Name: "PermissionRequest", Category: hooks.Interactive, Description: "Play sounds for Qwen permission requests", DefaultEnabled: true},
	{Name: "TodoCreated", Category: hooks.Loading, Description: "Play sounds when Qwen creates a todo item", DefaultEnabled: true},
	{Name: "TodoCompleted", Category: hooks.Completion, Description: "Play sounds when Qwen completes a todo item", DefaultEnabled: true},
}

// CopilotHooks defines the registry of GitHub Copilot CLI hooks supported by Claudio.
// PascalCase event names request VS Code-compatible snake_case payloads.
// Copilot's subagentStart hook currently only uses the native camelCase
// payload, so Claudio passes its event name explicitly in the hook command.
var CopilotHooks = []HookDefinition{
	{Name: "PreToolUse", Category: hooks.Loading, Description: "Play loading sounds before Copilot tool execution", DefaultEnabled: true},
	{Name: "PostToolUse", Category: hooks.Success, Description: "Play success sounds after Copilot tool execution", DefaultEnabled: true},
	{Name: "PostToolUseFailure", Category: hooks.Error, Description: "Play error sounds after failed Copilot tool execution", DefaultEnabled: true},
	{Name: "UserPromptSubmit", Category: hooks.Interactive, Description: "Play interaction sounds when user submits prompts", DefaultEnabled: true},
	{Name: "SessionStart", Category: hooks.System, Description: "Play sounds when a Copilot session starts or resumes", DefaultEnabled: true},
	{Name: "SessionEnd", Category: hooks.Interactive, Description: "Play sounds when a Copilot session ends", DefaultEnabled: true},
	{Name: "Stop", Category: hooks.Completion, Description: "Play sounds when Copilot finishes responding", DefaultEnabled: true},
	{Name: "subagentStart", Category: hooks.Loading, Description: "Play sounds when a Copilot subagent starts", DefaultEnabled: true},
	{Name: "SubagentStop", Category: hooks.Completion, Description: "Play sounds when a Copilot subagent finishes", DefaultEnabled: true},
	{Name: "PreCompact", Category: hooks.System, Description: "Play sounds before Copilot context compaction", DefaultEnabled: true},
	{Name: "Notification", Category: hooks.Interactive, Description: "Play sounds for Copilot notifications", DefaultEnabled: true},
	{Name: "PermissionRequest", Category: hooks.Interactive, Description: "Play sounds for Copilot permission requests", DefaultEnabled: true},
	{Name: "ErrorOccurred", Category: hooks.Error, Description: "Play sounds for Copilot execution errors", DefaultEnabled: true},
}

// GetAllHooks returns all hooks defined in the registry
func GetAllHooks() []HookDefinition {
	slog.Debug("retrieving all hooks from registry", "total_hooks", len(AllHooks))
	return AllHooks
}

// GetEnabledHooks returns only hooks that are enabled by default
func GetEnabledHooks() []HookDefinition {
	var enabled []HookDefinition

	for _, hook := range AllHooks {
		if hook.DefaultEnabled {
			enabled = append(enabled, hook)
		}
	}

	slog.Debug("retrieved enabled hooks from registry",
		"enabled_count", len(enabled),
		"total_count", len(AllHooks))

	return enabled
}

// GetHookByName looks up a hook by name and returns it with a found flag
func GetHookByName(name string) (HookDefinition, bool) {
	slog.Debug("looking up hook by name", "hook_name", name)

	for _, hook := range AllHooks {
		if hook.Name == name {
			slog.Debug("found hook in registry", "hook_name", name, "category", hook.Category)
			return hook, true
		}
	}

	slog.Debug("hook not found in registry", "hook_name", name)
	return HookDefinition{}, false
}

// GetHookNames returns a slice of all hook names from the registry
func GetHookNames() []string {
	names := make([]string, len(AllHooks))

	for i, hook := range AllHooks {
		names[i] = hook.Name
	}

	slog.Debug("retrieved hook names from registry",
		"hook_count", len(names),
		"names", names)

	return names
}
