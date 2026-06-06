package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/afero"
)

const NoSupportedAgentsDetectedMessage = "No supported agents detected. Install Claude Code, Codex CLI, Gemini CLI, Qwen Code, or GitHub Copilot CLI, or rerun with --agent claude, --agent codex, --agent gemini, --agent qwen, --agent copilot, or --agent all."

// AgentTarget is one concrete agent config file selected for install or uninstall.
type AgentTarget struct {
	Agent      Agent
	ConfigPath string
}

// ResolveAgentTargets converts auto/all/concrete agent selections into concrete
// agent config file targets for the requested scope.
func ResolveAgentTargets(agent Agent, scope string) ([]AgentTarget, error) {
	normalizedScope, err := NormalizeScope(scope)
	if err != nil {
		return nil, err
	}

	switch agent {
	case AgentAuto:
		return resolveAutoAgentTargets(normalizedScope)
	case AgentAll:
		return resolveAllAgentTargets(normalizedScope)
	default:
		if !agent.IsConcrete() {
			return nil, fmt.Errorf("invalid agent '%s'", agent)
		}
		target, err := resolveConcreteAgentTarget(agent, normalizedScope)
		if err != nil {
			return nil, err
		}
		return []AgentTarget{target}, nil
	}
}

func resolveAllAgentTargets(scope string) ([]AgentTarget, error) {
	targets := make([]AgentTarget, 0, len(ConcreteAgents()))
	for _, agent := range ConcreteAgents() {
		target, err := resolveConcreteAgentTarget(agent, scope)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func resolveAutoAgentTargets(scope string) ([]AgentTarget, error) {
	var targets []AgentTarget
	for _, agent := range ConcreteAgents() {
		if !hasAgentEvidence(agent, scope) {
			continue
		}
		target, err := resolveConcreteAgentTarget(agent, scope)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("%s", NoSupportedAgentsDetectedMessage)
	}
	return targets, nil
}

func resolveConcreteAgentTarget(agent Agent, scope string) (AgentTarget, error) {
	path, err := agent.BestConfigPath(scope)
	if err != nil {
		return AgentTarget{}, err
	}
	return AgentTarget{Agent: agent, ConfigPath: path}, nil
}

func hasAgentEvidence(agent Agent, scope string) bool {
	if hasAgentExecutable(agent) {
		return true
	}
	if hasAgentConfigEvidence(agent, scope) {
		return true
	}
	if hasExistingClaudioHooks(agent, scope) {
		return true
	}

	if scope == ScopeProject {
		return hasAgentConfigEvidence(agent, ScopeGlobal) || hasExistingClaudioHooks(agent, ScopeGlobal)
	}
	return false
}

func hasAgentExecutable(agent Agent) bool {
	command := string(agent)
	_, err := exec.LookPath(command)
	return err == nil
}

func hasAgentConfigEvidence(agent Agent, scope string) bool {
	paths, err := agentConfigPaths(agent, scope)
	if err != nil {
		return false
	}
	for _, path := range paths {
		if pathExists(path) || pathExists(filepath.Dir(path)) {
			return true
		}
	}
	return false
}

func hasExistingClaudioHooks(agent Agent, scope string) bool {
	paths, err := agentConfigPaths(agent, scope)
	if err != nil {
		return false
	}
	fsys := afero.NewOsFs()
	for _, path := range paths {
		if !pathExists(path) {
			continue
		}
		settings, err := ReadSettingsFile(fsys, path)
		if err != nil {
			continue
		}
		if settingsContainClaudioHooks(settings) {
			return true
		}
	}
	return false
}

func agentConfigPaths(agent Agent, scope string) ([]string, error) {
	switch agent {
	case AgentClaude:
		return FindClaudeSettingsPaths(scope)
	case AgentCodex:
		return FindCodexHooksPaths(scope)
	case AgentGemini:
		return FindGeminiSettingsPaths(scope)
	case AgentQwen:
		return FindQwenSettingsPaths(scope)
	case AgentCopilot:
		return FindCopilotSettingsPaths(scope)
	default:
		return nil, fmt.Errorf("invalid concrete agent '%s'", agent)
	}
}

func settingsContainClaudioHooks(settings *SettingsMap) bool {
	if settings == nil {
		return false
	}
	hooksValue, ok := (*settings)["hooks"]
	if !ok {
		return false
	}
	hooksMap, ok := hooksValue.(map[string]interface{})
	if !ok {
		return false
	}
	for _, hookValue := range hooksMap {
		if IsClaudioHook(hookValue) {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
