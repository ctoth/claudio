package cli

import (
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"claudio.click/internal/install"
)

// hookTargetFlags are the flags install and uninstall share: which agent
// settings to touch (--scope, --agent) and how to report it (--dry-run,
// --quiet, --print).
type hookTargetFlags struct {
	scope  string
	agent  string
	dryRun bool
	quiet  bool
	print  bool
}

// hookTargetHelp is the verb-specific help text for the shared flags.
type hookTargetHelp struct {
	scope  string // lead-in for --scope, e.g. "Installation scope"
	dryRun string
	print  string
}

func (f *hookTargetFlags) register(cmd *cobra.Command, help hookTargetHelp) {
	flags := cmd.Flags()
	flags.StringVarP(&f.scope, "scope", "s", install.ScopeGlobal, help.scope+": 'global' for user-wide settings, 'project' for project-specific settings")
	flags.StringVarP(&f.agent, "agent", "a", string(install.AgentAuto), "Target agent: 'auto', 'claude', 'codex', 'gemini', 'qwen', 'copilot', or 'all'")
	flags.BoolVarP(&f.dryRun, "dry-run", "d", false, help.dryRun)
	flags.BoolVarP(&f.quiet, "quiet", "q", false, "Suppress output (no progress messages)")
	flags.BoolVarP(&f.print, "print", "p", false, help.print)
}

// resolve validates --scope and --agent and returns the normalized scope
// with the agent settings files it selects.
func (f *hookTargetFlags) resolve() (string, []install.AgentTarget, error) {
	scope, err := install.NormalizeScope(f.scope)
	if err != nil {
		return "", nil, err
	}
	agent, err := install.ParseAgent(f.agent)
	if err != nil {
		return "", nil, err
	}
	targets, err := install.ResolveAgentTargets(agent, scope)
	if err != nil {
		return "", nil, err
	}
	slog.Debug("resolved hook targets", "scope", scope, "agent", agent, "count", len(targets),
		"dry_run", f.dryRun, "quiet", f.quiet, "print", f.print)
	return scope, targets, nil
}

// printHookTargets writes the --print report shared by install and
// uninstall; perTarget adds verb-specific lines after each target.
func (f *hookTargetFlags) printHookTargets(cmd *cobra.Command, verb, scope string, targets []install.AgentTarget, perTarget func(install.AgentTarget)) {
	if f.dryRun {
		cmd.Printf("PRINT: DRY-RUN %s configuration for scope: %s\n", verb, scope)
		cmd.Printf("  Mode: Simulation (no changes will be made)\n")
	} else {
		cmd.Printf("PRINT: %s configuration for scope: %s\n", upperFirst(verb), scope)
	}
	if f.quiet {
		cmd.Printf("  Output: Quiet mode (minimal messages)\n")
	}
	cmd.Printf("  Scope: %s\n", scope)
	for _, target := range targets {
		cmd.Printf("  Target agent: %s\n", target.Agent)
		cmd.Printf("  Settings Path: %s\n", target.ConfigPath)
		if perTarget != nil {
			perTarget(target)
		}
	}
}

// printTarget writes the per-target progress lines unless --quiet.
func (f *hookTargetFlags) printTarget(cmd *cobra.Command, target install.AgentTarget) {
	if f.quiet {
		return
	}
	cmd.Printf("Target agent: %s\n", target.Agent)
	cmd.Printf("Settings path: %s\n", target.ConfigPath)
}

// upperFirst upper-cases the first byte of an ASCII word.
func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// applyToTargets prints the progress banner, then runs apply on each
// target after its progress lines, stopping at the first failure.
func (f *hookTargetFlags) applyToTargets(cmd *cobra.Command, progress, scope string, targets []install.AgentTarget, apply func(install.AgentTarget) error) error {
	if !f.quiet {
		cmd.Printf("%s Claudio hooks for %s scope...\n", progress, scope)
	}
	for _, target := range targets {
		f.printTarget(cmd, target)
		if err := apply(target); err != nil {
			return err
		}
	}
	return nil
}
