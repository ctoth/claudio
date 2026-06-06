package install

import (
	"fmt"
	"log/slog"
	"strings"
)

// Agent identifies which coding agent Claudio installs hooks for.
type Agent string

const (
	AgentAuto   Agent = "auto"
	AgentAll    Agent = "all"
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
	AgentGemini Agent = "gemini"
	AgentQwen   Agent = "qwen"
)

const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
	scopeUser    = "user"
)

// ParseAgent validates and converts a string into an Agent.
func ParseAgent(s string) (Agent, error) {
	agent := Agent(strings.ToLower(strings.TrimSpace(s)))
	switch agent {
	case AgentAuto, AgentAll, AgentClaude, AgentCodex, AgentGemini, AgentQwen:
		return agent, nil
	default:
		return "", fmt.Errorf("invalid agent '%s': must be 'auto', 'claude', 'codex', 'gemini', 'qwen', or 'all'", s)
	}
}

// String returns the agent's string form.
func (a Agent) String() string { return string(a) }

// ConcreteAgents returns every directly installable agent.
func ConcreteAgents() []Agent {
	return []Agent{AgentClaude, AgentCodex, AgentGemini, AgentQwen}
}

// IsConcrete returns true for agents that map to one config target.
func (a Agent) IsConcrete() bool {
	switch a {
	case AgentClaude, AgentCodex, AgentGemini, AgentQwen:
		return true
	default:
		return false
	}
}

// NormalizeScope converts public and legacy scope names to Claudio's public scope vocabulary.
func NormalizeScope(scope string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(scope))
	switch normalized {
	case ScopeGlobal, scopeUser:
		return ScopeGlobal, nil
	case ScopeProject:
		return ScopeProject, nil
	default:
		return "", fmt.Errorf("invalid scope '%s': must be 'global' or 'project'", scope)
	}
}

// Matcher returns the default hook matcher pattern for the agent.
// Codex uses "*"; Claude Code uses ".*".
func (a Agent) Matcher() string {
	switch a {
	case AgentCodex:
		return "*"
	case AgentGemini:
		return ""
	case AgentQwen:
		return ".*"
	default:
		return ".*"
	}
}

// Registry returns the hook definitions supported for the agent.
func (a Agent) Registry() []HookDefinition {
	switch a {
	case AgentCodex:
		return CodexHooks
	case AgentGemini:
		return GeminiHooks
	case AgentQwen:
		return QwenHooks
	case AgentClaude:
		return AllHooks
	default:
		return nil
	}
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
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return "", err
	}
	switch a {
	case AgentCodex:
		return FindBestCodexPath(normalizedScope)
	case AgentClaude:
		return FindBestSettingsPath(normalizedScope)
	case AgentGemini:
		return FindBestGeminiPath(normalizedScope)
	case AgentQwen:
		return FindBestQwenPath(normalizedScope)
	case AgentAuto, AgentAll:
		return "", fmt.Errorf("agent '%s' must be resolved before selecting a config path", a)
	default:
		return "", fmt.Errorf("invalid agent '%s'", a)
	}
}
