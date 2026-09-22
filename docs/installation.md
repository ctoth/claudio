---
layout: default
title: "Installation"
---

# Installation

Installation has two parts:

1. Install the `claudio` binary.
2. Register hooks for the agent you want Claudio to listen to.

## Install The Binary

### Prebuilt Binary

Download the matching asset from [GitHub Releases](https://github.com/ctoth/claudio/releases/latest):

| Platform | Asset |
| --- | --- |
| Windows x64 | `claudio-windows-amd64.exe` |
| Linux x64 | `claudio-linux-amd64` |
| macOS Apple Silicon | `claudio-darwin-arm64` |

Rename it to `claudio` (`claudio.exe` on Windows) and place it in a directory
on `PATH`. On Linux and macOS, run `chmod +x` on the downloaded binary.
Release binaries are built with `CGO_ENABLED=0` and include the native audio
backend.

Put the binary where it will stay before you install hooks. Hooks record the
absolute path of the `claudio` that installed them, so if you move the binary
later, run `claudio install` again from the new location.

### Build From Source

Claudio requires Go 1.25.13 or newer. With Go's default automatic toolchain
selection, commands run from this repository use the Go 1.26.6 toolchain
declared in `go.mod`. No C compiler is needed; native audio builds with
`CGO_ENABLED=0`.

```bash
go install claudio.click/cmd/claudio@latest
```

The binary lands in `$(go env GOBIN)`, or `$(go env GOPATH)/bin` when `GOBIN`
is unset. Make sure that directory is on `PATH`:

```bash
go env GOBIN GOPATH
```

### Audio Runtime

The default `auto` backend picks `oto`, Claudio's built-in player, on Windows,
macOS, and Linux. Under WSL it picks `system_command` instead when one of
`paplay`, `ffplay`, `aplay`, or `afplay` is on `PATH`, and falls back to `oto`
otherwise.

On Linux, Oto first connects to a PulseAudio-compatible server (PulseAudio or
PipeWire's Pulse layer), honoring `PULSE_SERVER`. If none is reachable it falls
back to ALSA, which needs `libasound.so.2` at runtime. Neither path needs
development headers to build.

`claudio status` shows which backend `auto` resolved to:

```text
  audio backend:  auto -> oto (available; playback not tested)
```

"Available" means the backend is compiled in, or for `system_command` that a
player is on `PATH`. It does not open the audio device.

## Auto Install

Install global hooks for every detected supported agent:

```bash
claudio install
```

This is the default: `--agent auto --scope global`. An agent counts as
detected when its command (`claude`, `codex`, `gemini`, `qwen`, or `copilot`)
is on `PATH`, its settings directory exists, or its settings file already
contains Claudio hooks. Every detected agent gets its hook set. If nothing is
detected, the command fails and asks you to pick an agent with `--agent`.

To force every supported hook target:

```bash
claudio install --agent all --scope global
```

Use project scope only when hooks should live under the current repository:

```bash
claudio install --scope project
```

## Claude Code Hooks

Install global Claude Code hooks:

```bash
claudio install --agent claude --scope global
```

Install hooks only for the current project:

```bash
claudio install --agent claude --scope project
```

Global scope writes `~/.claude/settings.json`, or
`$CLAUDE_CONFIG_DIR/settings.json` when `CLAUDE_CONFIG_DIR` is set, matching
where Claude Code reads its settings. On Windows `~` is `%USERPROFILE%`.

Project scope writes `./.claude/settings.json`.

## Codex Hooks

Install global Codex hooks:

```bash
claudio install --agent codex --scope global
```

Install hooks only for the current project:

```bash
claudio install --agent codex --scope project
```

Global scope uses `$CODEX_HOME/hooks.json` when `CODEX_HOME` is set, otherwise
`~/.codex/hooks.json`.

Project scope writes `./.codex/hooks.json`.

Each Codex hook entry carries both a POSIX command and a PowerShell command,
so the same `hooks.json` works on Windows and Unix.

After installing Codex hooks, run `/hooks` in Codex and trust the Claudio hook.
Codex does not run untrusted hooks.

## Gemini Hooks

Install global Gemini hooks:

```bash
claudio install --agent gemini --scope global
```

Install hooks only for the current project:

```bash
claudio install --agent gemini --scope project
```

Global scope writes `~/.gemini/settings.json`.

Project scope writes `./.gemini/settings.json`.

## Qwen Code Hooks

Install global Qwen Code hooks:

```bash
claudio install --agent qwen --scope global
```

Install hooks only for the current project:

```bash
claudio install --agent qwen --scope project
```

Global scope writes `~/.qwen/settings.json`.

Project scope writes `./.qwen/settings.json`.

## GitHub Copilot CLI Hooks

Install global GitHub Copilot CLI hooks:

```bash
claudio install --agent copilot --scope global
```

Install hooks only for the current project:

```bash
claudio install --agent copilot --scope project
```

Global scope writes `~/.copilot/settings.json`, or `$COPILOT_HOME/settings.json`
when `COPILOT_HOME` is set.

Project scope writes `./.github/copilot/settings.local.json`. If that file
does not exist but `./.github/copilot/settings.json` does, Claudio writes to the
existing file instead.

## Inspect Before Writing

Dry run, which prints the target settings path and the hooks that would be
installed:

```bash
claudio install --dry-run
claudio install --agent claude --scope global --dry-run
claudio install --agent codex --scope global --dry-run
claudio install --agent gemini --scope global --dry-run
claudio install --agent qwen --scope global --dry-run
claudio install --agent copilot --scope global --dry-run
```

Print only the target agent and settings path:

```bash
claudio install --print
```

Quiet mode:

```bash
claudio install --quiet
```

The installer holds an advisory lock while it reads, merges, writes, and
verifies the settings file. Hooks from other tools are left alone; earlier
Claudio entries are replaced with the current form, so reinstalling is safe.

## Installed Hook Sets

Claude Code defaults:

| Hook | Category |
| --- | --- |
| `SessionStart` | system |
| `Setup` | system |
| `UserPromptSubmit` | interactive |
| `UserPromptExpansion` | interactive |
| `PreToolUse` | loading |
| `PermissionRequest` | interactive |
| `PermissionDenied` | error |
| `PostToolUse` | success or error |
| `PostToolUseFailure` | error |
| `PostToolBatch` | success |
| `Notification` | interactive |
| `SubagentStart` | loading |
| `SubagentStop` | completion |
| `TaskCreated` | loading |
| `TaskCompleted` | completion |
| `Stop` | completion |
| `StopFailure` | error |
| `TeammateIdle` | interactive |
| `InstructionsLoaded` | system |
| `ConfigChange` | system |
| `CwdChanged` | system |
| `WorktreeCreate` | system |
| `WorktreeRemove` | system |
| `PreCompact` | system |
| `PostCompact` | system |
| `Elicitation` | interactive |
| `ElicitationResult` | interactive |
| `SessionEnd` | interactive |

`MessageDisplay` and `FileChanged` are known to Claudio but not installed.
Add them to your settings by hand only if you want audio for streamed text or
broad file change events.

Codex defaults:

| Hook | Category |
| --- | --- |
| `PreToolUse` | loading |
| `PostToolUse` | success or error |
| `UserPromptSubmit` | interactive |
| `Stop` | completion |
| `SubagentStop` | completion |
| `SubagentStart` | loading |
| `PreCompact` | system |
| `PostCompact` | system |
| `SessionStart` | system |
| `PermissionRequest` | interactive |

Gemini defaults:

| Hook | Category |
| --- | --- |
| `BeforeTool` | loading |
| `AfterTool` | success or error |
| `BeforeAgent` | interactive |
| `AfterAgent` | completion |
| `BeforeModel` | silent no-op |
| `AfterModel` | silent no-op |
| `BeforeToolSelection` | silent no-op |
| `SessionStart` | system |
| `SessionEnd` | interactive |
| `Notification` | interactive |
| `PreCompress` | system |

Qwen Code defaults:

| Hook | Category |
| --- | --- |
| `PreToolUse` | loading |
| `PostToolUse` | success |
| `PostToolUseFailure` | error |
| `UserPromptSubmit` | interactive |
| `SessionStart` | system |
| `SessionEnd` | interactive |
| `Stop` | completion |
| `StopFailure` | error |
| `SubagentStart` | loading |
| `SubagentStop` | completion |
| `PreCompact` | system |
| `PostCompact` | system |
| `Notification` | interactive |
| `PermissionRequest` | interactive |
| `TodoCreated` | loading |
| `TodoCompleted` | completion |

GitHub Copilot CLI defaults:

| Hook | Category |
| --- | --- |
| `PreToolUse` | loading |
| `PostToolUse` | success |
| `PostToolUseFailure` | error |
| `UserPromptSubmit` | interactive |
| `SessionStart` | system |
| `SessionEnd` | interactive |
| `Stop` | completion |
| `subagentStart` | loading |
| `SubagentStop` | completion |
| `PreCompact` | system |
| `Notification` | interactive |
| `PermissionRequest` | interactive |
| `ErrorOccurred` | error |

## Optional Agent Commands

These commands install control artifacts so you can ask an agent to adjust
Claudio without leaving the session.

```bash
claudio install-commands --agent claude
claudio install-commands --agent codex
claudio install-commands --agent antigravity
```

Artifacts:

| Agent | Installed artifact |
| --- | --- |
| Claude Code | `~/.claude/commands/claudio.md` (`$CLAUDE_CONFIG_DIR/commands/claudio.md` when set) |
| Codex | `$HOME/.agents/skills/claudio/SKILL.md` |
| Antigravity | `~/.gemini/config/skills/claudio/SKILL.md` and `~/.gemini/antigravity-cli/skills/claudio.md` |

If an artifact already exists and you have edited it, `install-commands`
refuses to overwrite it and `uninstall-commands` refuses to delete it. Move
your copy aside first if you want the stock version back.

Remove them with:

```bash
claudio uninstall-commands --agent claude
claudio uninstall-commands --agent codex
claudio uninstall-commands --agent antigravity
```

## Verify

Check effective config:

```bash
claudio status
```

Run one hook payload manually:

```bash
echo '{"session_id":"test","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git status"},"tool_response":{"stdout":"ok","stderr":"","interrupted":false}}' | claudio
```

If you do not want audio during a test run:

```bash
echo '{"session_id":"test","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"ok","stderr":"","interrupted":false}}' | claudio --silent
```

## Uninstall Hooks

```bash
claudio uninstall --agent all --scope global
claudio uninstall --agent claude --scope global
claudio uninstall --agent codex --scope global
claudio uninstall --agent gemini --scope global
claudio uninstall --agent qwen --scope global
claudio uninstall --agent copilot --scope global
```

Like `install`, `uninstall` defaults to `--agent auto --scope global` and
accepts `--dry-run`, `--print`, and `--quiet`.

## Next

- [Configuration](configuration)
- [CLI Reference](cli-reference)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
