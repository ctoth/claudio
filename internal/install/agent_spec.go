package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hookShape is how an agent's settings file lays out one hook event.
type hookShape int

const (
	// shapeMatcherGroups is [{"matcher": m, "hooks": [{command...}]}].
	shapeMatcherGroups hookShape = iota
	// shapeFlatCommands is [{command...}] (GitHub Copilot CLI).
	shapeFlatCommands
)

// agentSpec describes everything install/uninstall needs to know about one
// concrete agent. Adding an agent means adding one entry to agentSpecs.
type agentSpec struct {
	agent Agent
	// registry points at the package-level hook list so it is read at call time.
	registry *[]HookDefinition
	matcher  string
	shape    hookShape

	// homeEnv names the variable that replaces <home>/homeDir when set
	// (whitespace-trimmed; blank means unset). Empty when the agent has none.
	homeEnv    string
	homeDir    string
	globalFile string
	// projectPaths are the project-scope candidates in priority order,
	// relative to the current working directory.
	projectPaths []string

	// hookAgentFlag appends "--hook-agent <agent>" to the hook command.
	hookAgentFlag bool
	// eventFlagHooks get "--hook-event <name>" appended to the hook command.
	eventFlagHooks map[string]bool
	// commandName is written as the command entry's "name" when non-empty.
	commandName string
	// timeoutSec is written as the command entry's "timeoutSec" when > 0.
	timeoutSec int

	// captainHook routes install/uninstall through the captain-hook module.
	captainHook bool
	// trustHint is printed after install when the agent needs the user to
	// trust new hooks.
	trustHint string
}

// agentSpecs lists every concrete agent in install/detection order.
var agentSpecs = []agentSpec{
	{
		agent:        AgentClaude,
		registry:     &AllHooks,
		matcher:      ".*",
		homeEnv:      "CLAUDE_CONFIG_DIR",
		homeDir:      ".claude",
		globalFile:   "settings.json",
		projectPaths: []string{filepath.Join(".claude", "settings.json")},
	},
	{
		agent:        AgentCodex,
		registry:     &CodexHooks,
		matcher:      "*",
		homeEnv:      "CODEX_HOME",
		homeDir:      ".codex",
		globalFile:   "hooks.json",
		projectPaths: []string{filepath.Join(".codex", "hooks.json")},
		captainHook:  true,
		trustHint:    "Run /hooks in Codex to trust the claudio hook.",
	},
	{
		agent:         AgentGemini,
		registry:      &GeminiHooks,
		matcher:       "",
		homeDir:       ".gemini",
		globalFile:    "settings.json",
		projectPaths:  []string{filepath.Join(".gemini", "settings.json")},
		hookAgentFlag: true,
		commandName:   "claudio",
	},
	{
		agent:         AgentQwen,
		registry:      &QwenHooks,
		matcher:       ".*",
		homeDir:       ".qwen",
		globalFile:    "settings.json",
		projectPaths:  []string{filepath.Join(".qwen", "settings.json")},
		hookAgentFlag: true,
		commandName:   "claudio",
	},
	{
		agent:      AgentCopilot,
		registry:   &CopilotHooks,
		matcher:    "",
		shape:      shapeFlatCommands,
		homeEnv:    "COPILOT_HOME",
		homeDir:    ".copilot",
		globalFile: "settings.json",
		projectPaths: []string{
			filepath.Join(".github", "copilot", "settings.local.json"),
			filepath.Join(".github", "copilot", "settings.json"),
		},
		hookAgentFlag:  true,
		eventFlagHooks: map[string]bool{"subagentStart": true},
		timeoutSec:     30,
	},
}

// spec returns the agent's descriptor; ok is false for auto, all and
// unknown agents.
func (a Agent) spec() (agentSpec, bool) {
	for _, s := range agentSpecs {
		if s.agent == a {
			return s, true
		}
	}
	return agentSpec{}, false
}

// concreteSpec returns the agent's descriptor or an error naming why the
// agent has no single config target.
func (a Agent) concreteSpec() (agentSpec, error) {
	if s, ok := a.spec(); ok {
		return s, nil
	}
	if a == AgentAuto || a == AgentAll {
		return agentSpec{}, fmt.Errorf("agent '%s' must be resolved before selecting a config path", a)
	}
	return agentSpec{}, fmt.Errorf("invalid agent '%s'", a)
}

// ConfigPaths returns the agent's candidate config file paths for global
// or project scope, in priority order. Global scope uses the agent's
// config-home variable when set, else <home>/<dir>/<file>; with no home
// directory it is an error.
func (a Agent) ConfigPaths(scope string) ([]string, error) {
	s, err := a.concreteSpec()
	if err != nil {
		return nil, err
	}
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}
	if normalizedScope == ScopeProject {
		return append([]string(nil), s.projectPaths...), nil
	}
	dir, err := s.configDir()
	if err != nil {
		return nil, err
	}
	return []string{filepath.Join(dir, s.globalFile)}, nil
}

// configDir returns the agent's global configuration directory. A
// config-home variable replaces <home>/<dir> entirely, so it is the only
// candidate: falling back to <home>/<dir> would write files the agent
// never loads.
func (s agentSpec) configDir() (string, error) {
	if s.homeEnv != "" {
		if dir := strings.TrimSpace(os.Getenv(s.homeEnv)); dir != "" {
			return dir, nil
		}
	}
	home, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, s.homeDir), nil
}

// ClaudeConfigDir returns Claude Code's configuration directory:
// CLAUDE_CONFIG_DIR when set, else <home>/.claude.
func ClaudeConfigDir() (string, error) {
	s, _ := AgentClaude.spec()
	return s.configDir()
}

// BestConfigPath returns the first candidate config path that exists, or
// the first candidate for creation when none exists yet.
func (a Agent) BestConfigPath(scope string) (string, error) {
	paths, err := a.ConfigPaths(scope)
	if err != nil {
		return "", err
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}

// TrustHint returns the message to print after installing hooks for the
// agent, or "" when the agent needs no follow-up.
func (a Agent) TrustHint() string {
	s, _ := a.spec()
	return s.trustHint
}
