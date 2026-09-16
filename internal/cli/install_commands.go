package cli

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// claudioCommandContent is the content of the claudio.md slash command file
const claudioCommandContent = `---
allowed-tools: Bash(claudio:*)
argument-hint: [volume 0.0-1.0 | mute | unmute | status]
description: Control Claudio audio feedback
---
Control Claudio audio using the claudio CLI.

Available commands:
- volume [0.0-1.0]: Set volume level
- mute: Disable audio persistently
- unmute: Enable audio persistently
- status: Show current settings

Run: claudio $ARGUMENTS
`

// claudioSkillContent is the content of the Codex skill file.
const claudioSkillContent = `---
name: claudio
description: Use this skill when the user wants to control Claudio audio feedback: set volume, mute, unmute, or check status through the claudio CLI.
---

Use the ` + "`claudio`" + ` CLI to control Claudio audio feedback.

Commands:
- ` + "`claudio volume <0.0-1.0>`" + `: Set volume level
- ` + "`claudio mute`" + `: Disable audio persistently
- ` + "`claudio unmute`" + `: Enable audio persistently
- ` + "`claudio status`" + `: Show current settings
`

type commandArtifactAgent string

const (
	commandArtifactAgentClaude      commandArtifactAgent = "claude"
	commandArtifactAgentCodex       commandArtifactAgent = "codex"
	commandArtifactAgentAntigravity commandArtifactAgent = "antigravity"
)

type commandArtifact struct {
	Agent           commandArtifactAgent
	Kind            string
	Directory       string
	Path            string
	Content         string
	RemoveDirectory bool
}

// newInstallCommandsCommand creates the install-commands subcommand
func newInstallCommandsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install-commands",
		Short: "Install agent command artifacts",
		Long: `Install command artifacts for controlling Claudio from supported coding agents.

For Claude Code, this command creates ~/.claude/commands/claudio.md.
For Codex, this command creates $HOME/.agents/skills/claudio/SKILL.md.
For Antigravity, this command creates ~/.gemini/config/skills/claudio/SKILL.md
and ~/.gemini/antigravity-cli/skills/claudio.md.

After Claude Code installation, you can use commands like:
  /claudio volume 0.5
  /claudio mute
  /claudio unmute
  /claudio status

After Codex installation, invoke the skill as $claudio.`,
		RunE: runInstallCommandsE,
	}

	cmd.Flags().StringP("agent", "a", "claude", "Target agent: 'claude' for Claude Code, 'codex' for OpenAI Codex, 'antigravity' for Google Antigravity")

	return cmd
}

// newUninstallCommandsCommand creates the uninstall-commands subcommand.
func newUninstallCommandsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall-commands",
		Short: "Uninstall agent command artifacts",
		Long: `Uninstall command artifacts created by install-commands.

For Claude Code, this removes ~/.claude/commands/claudio.md.
For Codex, this removes $HOME/.agents/skills/claudio/SKILL.md and the empty claudio skill directory.
For Antigravity, this removes ~/.gemini/config/skills/claudio/SKILL.md,
~/.gemini/antigravity-cli/skills/claudio.md, and the empty claudio skill directory.`,
		RunE: runUninstallCommandsE,
	}

	cmd.Flags().StringP("agent", "a", "claude", "Target agent: 'claude' for Claude Code, 'codex' for OpenAI Codex, 'antigravity' for Google Antigravity")

	return cmd
}

// runInstallCommandsE handles the install-commands subcommand execution
func runInstallCommandsE(cmd *cobra.Command, args []string) error {
	slog.Debug("install-commands started")

	agent, err := commandArtifactAgentFlag(cmd)
	if err != nil {
		return err
	}

	artifacts, err := resolveCommandArtifacts(agent)
	if err != nil {
		return err
	}

	if err := preflightCommandArtifacts(artifacts); err != nil {
		return err
	}
	for _, artifact := range artifacts {
		if err := installCommandArtifact(artifact); err != nil {
			return fmt.Errorf("failed to install %s: %w", artifact.Kind, err)
		}
		cmd.Printf("Installed %s for %s: %s\n", artifact.Kind, artifact.Agent.String(), artifact.Path)
	}
	cmd.Println()

	printCommandArtifactUsage(cmd, agent)

	slog.Info("command artifacts installed successfully", "agent", agent, "count", len(artifacts))

	return nil
}

// runUninstallCommandsE handles the uninstall-commands subcommand execution.
func runUninstallCommandsE(cmd *cobra.Command, args []string) error {
	slog.Debug("uninstall-commands started")

	agent, err := commandArtifactAgentFlag(cmd)
	if err != nil {
		return err
	}

	artifacts, err := resolveCommandArtifacts(agent)
	if err != nil {
		return err
	}

	if err := preflightCommandArtifacts(artifacts); err != nil {
		return err
	}
	removedCount := 0
	for _, artifact := range artifacts {
		removed, err := uninstallCommandArtifact(artifact)
		if err != nil {
			return fmt.Errorf("failed to uninstall %s: %w", artifact.Kind, err)
		}

		if removed {
			removedCount++
			cmd.Printf("Removed %s for %s: %s\n", artifact.Kind, artifact.Agent.String(), artifact.Path)
		} else {
			cmd.Printf("No %s for %s found at %s\n", artifact.Kind, artifact.Agent.String(), artifact.Path)
		}
	}

	slog.Info("command artifact uninstall completed", "agent", agent, "removed_count", removedCount, "count", len(artifacts))

	return nil
}

func commandArtifactAgentFlag(cmd *cobra.Command) (commandArtifactAgent, error) {
	agentStr, err := cmd.Flags().GetString("agent")
	if err != nil {
		return "", fmt.Errorf("failed to read agent flag: %w", err)
	}
	return parseCommandArtifactAgent(agentStr)
}

func parseCommandArtifactAgent(s string) (commandArtifactAgent, error) {
	switch commandArtifactAgent(s) {
	case commandArtifactAgentClaude, commandArtifactAgentCodex, commandArtifactAgentAntigravity:
		return commandArtifactAgent(s), nil
	default:
		return "", fmt.Errorf("invalid agent '%s': must be 'claude', 'codex', or 'antigravity'", s)
	}
}

func (a commandArtifactAgent) String() string { return string(a) }

func resolveCommandArtifacts(agent commandArtifactAgent) ([]commandArtifact, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	switch agent {
	case commandArtifactAgentClaude:
		commandsDir := filepath.Join(homeDir, ".claude", "commands")
		return []commandArtifact{{
			Agent:     agent,
			Kind:      "slash command",
			Directory: commandsDir,
			Path:      filepath.Join(commandsDir, "claudio.md"),
			Content:   claudioCommandContent,
		}}, nil
	case commandArtifactAgentCodex:
		skillDir := filepath.Join(homeDir, ".agents", "skills", "claudio")
		return []commandArtifact{{
			Agent:           agent,
			Kind:            "skill",
			Directory:       skillDir,
			Path:            filepath.Join(skillDir, "SKILL.md"),
			Content:         claudioSkillContent,
			RemoveDirectory: true,
		}}, nil
	case commandArtifactAgentAntigravity:
		agentSkillDir := filepath.Join(homeDir, ".gemini", "config", "skills", "claudio")
		cliSkillDir := filepath.Join(homeDir, ".gemini", "antigravity-cli", "skills")
		return []commandArtifact{
			{
				Agent:           agent,
				Kind:            "Antigravity global skill",
				Directory:       agentSkillDir,
				Path:            filepath.Join(agentSkillDir, "SKILL.md"),
				Content:         claudioSkillContent,
				RemoveDirectory: true,
			},
			{
				Agent:     agent,
				Kind:      "Antigravity CLI slash command",
				Directory: cliSkillDir,
				Path:      filepath.Join(cliSkillDir, "claudio.md"),
				Content:   claudioSkillContent,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported agent %q", agent)
	}
}

// installCommandsToPath creates the commands directory and writes claudio.md.
func installCommandsToPath(commandsDir, claudioMdPath string) error {
	return installCommandArtifact(commandArtifact{
		Agent:     commandArtifactAgentClaude,
		Kind:      "slash command",
		Directory: commandsDir,
		Path:      claudioMdPath,
		Content:   claudioCommandContent,
	})
}

// Check every destination before a known conflict can cause a partial update.
// Each mutation rechecks its own file to catch changes after the preflight.
func preflightCommandArtifacts(artifacts []commandArtifact) error {
	for _, artifact := range artifacts {
		data, err := os.ReadFile(artifact.Path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot inspect command artifact %s: %w", artifact.Path, err)
		}
		if !bytes.Equal(data, []byte(artifact.Content)) {
			return fmt.Errorf("refusing to change existing customized command artifact: %s", artifact.Path)
		}
	}
	return nil
}

func installCommandArtifact(artifact commandArtifact) error {
	slog.Debug("installing command artifact", "agent", artifact.Agent, "dir", artifact.Directory, "file", artifact.Path)

	err := os.MkdirAll(artifact.Directory, 0755)
	if err != nil {
		slog.Error("failed to create command artifact directory", "path", artifact.Directory, "error", err)
		return fmt.Errorf("failed to create command artifact directory: %w", err)
	}

	slog.Debug("command artifact directory ready", "path", artifact.Directory)

	existing, err := os.ReadFile(artifact.Path)
	if err == nil {
		if bytes.Equal(existing, []byte(artifact.Content)) {
			return nil
		}
		return fmt.Errorf("refusing to overwrite existing or customized command artifact %s", artifact.Path)
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("failed to inspect command artifact: %w", err)
	}
	file, err := os.OpenFile(artifact.Path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to create command artifact: %w", err)
	}
	_, writeErr := file.WriteString(artifact.Content)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(artifact.Path)
		if writeErr != nil {
			return fmt.Errorf("failed to write command artifact: %w", writeErr)
		}
		return fmt.Errorf("failed to close command artifact: %w", closeErr)
	}

	slog.Debug("command artifact written successfully", "path", artifact.Path)

	return nil
}

func uninstallCommandArtifact(artifact commandArtifact) (bool, error) {
	slog.Debug("uninstalling command artifact", "agent", artifact.Agent, "file", artifact.Path)

	existing, err := os.ReadFile(artifact.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect command artifact: %w", err)
	}
	if !bytes.Equal(existing, []byte(artifact.Content)) {
		return false, fmt.Errorf("refusing to remove customized command artifact %s", artifact.Path)
	}
	err = os.Remove(artifact.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to remove command artifact: %w", err)
	}

	if artifact.RemoveDirectory {
		if err := os.Remove(artifact.Directory); err != nil && !os.IsNotExist(err) {
			slog.Debug("command artifact directory left in place", "path", artifact.Directory, "error", err)
		}
	}

	return true, nil
}

func printCommandArtifactUsage(cmd *cobra.Command, agent commandArtifactAgent) {
	switch agent {
	case commandArtifactAgentCodex:
		cmd.Printf("You can now use $claudio in Codex to control Claudio.\n")
	case commandArtifactAgentAntigravity:
		cmd.Printf("You can now use /claudio in Antigravity CLI.\n")
		cmd.Printf("Antigravity agents can also select the claudio skill when audio control is requested.\n")
	default:
		cmd.Printf("You can now use /claudio in Claude Code:\n")
		cmd.Printf("  /claudio volume 0.5   - Set volume to 50%%\n")
		cmd.Printf("  /claudio mute         - Disable audio\n")
		cmd.Printf("  /claudio unmute       - Enable audio\n")
		cmd.Printf("  /claudio status       - Show current settings\n")
	}
}
