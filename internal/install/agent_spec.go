package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	captainhook "github.com/ctoth/captain-hook"
)

// agentSpec describes everything install/uninstall needs to know about one
// concrete agent. Adding an agent means adding one entry to agentSpecs.
type agentSpec struct {
	agent Agent
	// catalogAgent names the agent in captain-hook's event catalog, which
	// supplies its hook events and settings layout. Empty for OpenCode,
	// whose plugin events are claudio's own (OpenCodeHooks).
	catalogAgent captainhook.Agent
	matcher      string

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
	// flagCamelCaseEvents appends "--hook-event <name>" to the command of
	// events with no PascalCase key, whose camelCase payloads may not
	// name the event (GitHub Copilot CLI).
	flagCamelCaseEvents bool
	// commandName is written as the command entry's "name" when non-empty.
	commandName string
	// timeoutSec is written as the command entry's "timeoutSec" when > 0.
	timeoutSec int

	// powerShellCommand writes a forward-slash, quoted command plus a
	// PowerShell commandWindows override (Codex runs hooks through
	// PowerShell on Windows).
	powerShellCommand bool
	// trustHint is printed after install when the agent needs the user to
	// trust new hooks.
	trustHint string
	// pluginFile means the config path is a claudio-owned OpenCode plugin
	// file written whole, not a settings file claudio merges hooks into.
	pluginFile bool
}

// agentSpecs lists every concrete agent in install/detection order. An
// agent in captain-hook's catalog takes its matcher, settings locations and
// PowerShell command form from there; its entry here holds only what is
// claudio's own choice.
var agentSpecs = withCatalogFacts([]agentSpec{
	{
		agent:        AgentClaude,
		catalogAgent: captainhook.AgentClaude,
	},
	{
		agent:        AgentCodex,
		catalogAgent: captainhook.AgentCodex,
		trustHint:    "Run /hooks in Codex to trust the claudio hook.",
	},
	{
		agent:         AgentGemini,
		catalogAgent:  captainhook.AgentGemini,
		hookAgentFlag: true,
		commandName:   "claudio",
	},
	{
		agent:         AgentQwen,
		catalogAgent:  captainhook.AgentQwen,
		hookAgentFlag: true,
		commandName:   "claudio",
	},
	{
		agent:               AgentCopilot,
		catalogAgent:        captainhook.AgentCopilot,
		hookAgentFlag:       true,
		flagCamelCaseEvents: true,
		timeoutSec:          30,
	},
	{
		agent:        AgentCommandCode,
		catalogAgent: captainhook.AgentCommandCode,
		trustHint:    "Restart Command Code to load the claudio hooks.",
	},
	{
		agent:        AgentOpenCode,
		homeEnv:      "OPENCODE_CONFIG_DIR",
		homeDir:      filepath.Join(".config", "opencode"),
		globalFile:   filepath.Join("plugins", "claudio.js"),
		projectPaths: []string{filepath.Join(".opencode", "plugins", "claudio.js")},
		pluginFile:   true,
		trustHint:    "Restart OpenCode to load the claudio plugin.",
	},
})

// withCatalogFacts fills each catalog agent's matcher, settings locations
// and PowerShell command form from captain-hook's catalog. An agent missing
// from the catalog is a build mistake, so it panics at startup.
func withCatalogFacts(specs []agentSpec) []agentSpec {
	for i := range specs {
		s := &specs[i]
		if s.catalogAgent == "" {
			continue
		}
		hooks, ok := captainhook.Lookup(s.catalogAgent)
		if !ok {
			panic(fmt.Sprintf("agent %q is not in captain-hook's catalog", s.catalogAgent))
		}
		s.matcher = hooks.Matcher
		s.homeEnv = hooks.HomeEnv
		s.homeDir = hooks.ConfigDir
		s.globalFile = hooks.SettingsFile
		s.powerShellCommand = hooks.PowerShell
		s.projectPaths = make([]string, len(hooks.ProjectFiles))
		for j, file := range hooks.ProjectFiles {
			s.projectPaths[j] = filepath.FromSlash(file)
		}
	}
	return specs
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
	if s.agent == AgentOpenCode {
		if dir := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); dir != "" {
			return filepath.Join(dir, "opencode"), nil
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

// UsesPluginFile reports whether the agent's config path is a claudio-owned
// plugin file rather than a settings file with hooks merged in.
func (a Agent) UsesPluginFile() bool {
	s, _ := a.spec()
	return s.pluginFile
}

// TrustHint returns the message to print after installing hooks for the
// agent, or "" when the agent needs no follow-up.
func (a Agent) TrustHint() string {
	s, _ := a.spec()
	return s.trustHint
}
