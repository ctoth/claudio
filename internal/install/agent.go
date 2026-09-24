package install

import (
	"fmt"
	"log/slog"
	"strings"
)

// Agent identifies which coding agent Claudio installs hooks for.
type Agent string

const (
	AgentAuto    Agent = "auto"
	AgentAll     Agent = "all"
	AgentClaude  Agent = "claude"
	AgentCodex   Agent = "codex"
	AgentGemini  Agent = "gemini"
	AgentQwen    Agent = "qwen"
	AgentCopilot Agent = "copilot"
)

const (
	ScopeGlobal  = "global"
	ScopeProject = "project"
	scopeUser    = "user"
)

// ParseAgent validates and converts a string into an Agent.
func ParseAgent(s string) (Agent, error) {
	agent := Agent(strings.ToLower(strings.TrimSpace(s)))
	if agent == AgentAuto || agent == AgentAll || agent.IsConcrete() {
		return agent, nil
	}
	return "", fmt.Errorf("invalid agent '%s': must be 'auto', 'claude', 'codex', 'gemini', 'qwen', 'copilot', or 'all'", s)
}

// String returns the agent's string form.
func (a Agent) String() string { return string(a) }

// ConcreteAgents returns every directly installable agent.
func ConcreteAgents() []Agent {
	agents := make([]Agent, len(agentSpecs))
	for i, s := range agentSpecs {
		agents[i] = s.agent
	}
	return agents
}

// IsConcrete returns true for agents that map to one config target.
func (a Agent) IsConcrete() bool {
	_, ok := a.spec()
	return ok
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
	if s, ok := a.spec(); ok {
		return s.matcher
	}
	return ".*"
}

// Registry returns the hook definitions supported for the agent.
func (a Agent) Registry() []HookDefinition {
	if s, ok := a.spec(); ok {
		return *s.registry
	}
	return nil
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
