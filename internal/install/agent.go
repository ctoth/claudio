package install

import (
	"fmt"
	"log/slog"
)

// Agent identifies which coding agent Claudio installs hooks for.
type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

// ParseAgent validates and converts a string into an Agent.
func ParseAgent(s string) (Agent, error) {
	switch Agent(s) {
	case AgentClaude, AgentCodex:
		return Agent(s), nil
	default:
		return "", fmt.Errorf("invalid agent '%s': must be 'claude' or 'codex'", s)
	}
}

// String returns the agent's string form.
func (a Agent) String() string { return string(a) }

// Matcher returns the default hook matcher pattern for the agent.
// Codex uses "*"; Claude Code uses ".*".
func (a Agent) Matcher() string {
	if a == AgentCodex {
		return "*"
	}
	return ".*"
}

// Registry returns the hook definitions supported for the agent.
func (a Agent) Registry() []HookDefinition {
	if a == AgentCodex {
		return CodexHooks
	}
	return AllHooks
}

// EnabledHooks returns the agent's default-enabled hook definitions.
func (a Agent) EnabledHooks() []HookDefinition {
	var enabled []HookDefinition
	for _, h := range a.Registry() {
		if h.DefaultEnabled {
			enabled = append(enabled, h)
		}
	}
	slog.Debug("agent enabled hooks", "agent", a, "count", len(enabled))
	return enabled
}

// HookNames returns the names of every hook in the agent's registry.
func (a Agent) HookNames() []string {
	reg := a.Registry()
	names := make([]string, len(reg))
	for i, h := range reg {
		names[i] = h.Name
	}
	return names
}

// BestConfigPath returns the config file path to install hooks into for the agent and scope.
func (a Agent) BestConfigPath(scope string) (string, error) {
	if a == AgentCodex {
		return FindBestCodexPath(scope)
	}
	return FindBestSettingsPath(scope)
}
