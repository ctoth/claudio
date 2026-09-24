package install

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	captainhook "github.com/ctoth/captain-hook"
)

// executableRecognizer decides whether a basename refers to the claudio
// executable. Production matches only claudio and claudio.exe. End-to-end
// install tests that thread the go test binary path through
// GetExecutablePath need the recognizer to accept names like
// "cli.test.exe"; those tests opt in by setting
// CLAUDIO_TEST_RECOGNIZE_GO_TEST=1 via t.Setenv. The env var is read at
// call time so the production binary never imports the testing package
// for a test-only seam.
var executableRecognizer = func(name string) bool {
	if name == "claudio" || name == "claudio.exe" {
		return true
	}
	if os.Getenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST") == "1" {
		if strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe") {
			return true
		}
	}
	return false
}

// powerShellSingleQuoteEscaper doubles every character PowerShell treats as a
// single quote: the ASCII apostrophe and U+2018 through U+201B.
var powerShellSingleQuoteEscaper = strings.NewReplacer(
	"'", "''",
	"\u2018", "\u2018\u2018",
	"\u2019", "\u2019\u2019",
	"\u201a", "\u201a\u201a",
	"\u201b", "\u201b\u201b",
)

// GenerateHookSpecs returns the agent's claudio hooks in captain-hook's
// representation, one spec per enabled hook in registry order.
func GenerateHookSpecs(executablePath string, agent Agent) ([]captainhook.HookSpec, error) {
	spec, err := agent.concreteSpec()
	if err != nil {
		return nil, err
	}
	hooks := agent.EnabledHooks()
	specs := make([]captainhook.HookSpec, 0, len(hooks))
	for _, hook := range hooks {
		specs = append(specs, spec.hookSpec(executablePath, hook.Name))
	}
	slog.Debug("generated claudio hook specs", "agent", agent, "hook_count", len(specs))
	return specs, nil
}

// hookSpec returns the captain-hook spec for one of the agent's events.
func (s agentSpec) hookSpec(executablePath, event string) captainhook.HookSpec {
	hs := captainhook.HookSpec{
		Event:   event,
		Matcher: s.matcher,
		Command: s.hookCommand(executablePath, event),
		Flat:    s.shape == shapeFlatCommands,
	}
	if s.powerShellCommand {
		hs.Command, hs.CommandWindows = powerShellHookCommands(executablePath)
	}
	extra := make(map[string]any)
	if s.commandName != "" {
		extra["name"] = s.commandName
	}
	if s.timeoutSec > 0 {
		extra["timeoutSec"] = s.timeoutSec
	}
	if len(extra) > 0 {
		hs.Extra = extra
	}
	return hs
}

// powerShellHookCommands returns the portable command and the Windows
// PowerShell override for an agent that runs hooks through PowerShell on
// Windows. Both use forward slashes. PowerShell double quotes expand dollar
// signs and backticks in paths, so the override single-quotes the path.
func powerShellHookCommands(executablePath string) (command, commandWindows string) {
	executablePath = strings.ReplaceAll(executablePath, `\`, "/")
	return quoteCommandArg(executablePath),
		"& '" + powerShellSingleQuoteEscaper.Replace(executablePath) + "'"
}

// hookCommand returns the command string the agent runs for hookName.
func (s agentSpec) hookCommand(executablePath, hookName string) string {
	if !s.hookAgentFlag {
		return executablePath
	}
	command := quoteCommandArg(executablePath) + " --hook-agent " + string(s.agent)
	if s.eventFlagHooks[hookName] {
		command += " --hook-event " + hookName
	}
	return command
}

func quoteCommandArg(arg string) string {
	if !strings.ContainsAny(arg, " \t\r\n\"") {
		return arg
	}
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

// IsClaudioCommandString reports whether a command string refers to the
// claudio executable. It is the identity captain-hook uses to find
// claudio's commands, so install, uninstall and detection all share one
// recognizer.
func IsClaudioCommandString(cmdStr string) bool {
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return false
	}

	// Preserve support for legacy unquoted Windows paths with spaces, such as
	// C:\Program Files\claudio.exe, by trying the full string before treating
	// whitespace as argument separation.
	if executableRecognizer(commandBasename(stripSurroundingQuotes(cmdStr))) {
		return true
	}

	executable, _ := leadingCommandToken(cmdStr)
	return executableRecognizer(commandBasename(executable))
}

func stripSurroundingQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func leadingCommandToken(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}

	if s[0] == '"' || s[0] == '\'' {
		quote := s[0]
		for i := 1; i < len(s); i++ {
			if s[i] == quote {
				return s[1:i], true
			}
		}
		return stripSurroundingQuotes(s), true
	}

	if i := strings.IndexAny(s, " \t\r\n"); i >= 0 {
		return s[:i], true
	}
	return s, true
}

// commandBasename returns the final path segment of a command string,
// splitting on both '/' and '\' regardless of the host OS. filepath.Base
// only honors the running platform's separator, so on Linux/macOS it left
// a Windows-style hook command like `C:\Program Files\claudio.exe` intact
// and the recognizer never saw the bare `claudio.exe`. settings.json is
// portable data — a hook authored on Windows can be inspected on Linux and
// vice versa — so recognition must not depend on the reader's OS.
func commandBasename(cmdStr string) string {
	if i := strings.LastIndexAny(cmdStr, `/\`); i >= 0 {
		return cmdStr[i+1:]
	}
	return cmdStr
}

// GetExecutablePath returns the running executable's path with forward
// slashes, so the path works in bash (Claude Code passes hook commands
// through /usr/bin/bash on all platforms).
func GetExecutablePath() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(p), nil
}
