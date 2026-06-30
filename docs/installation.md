---
layout: default
title: "Installation"
---

# Installation

Installation has two parts:

1. Install the `claudio` binary.
2. Register hooks for the agent you want Claudio to listen to.

## Install The Binary

```bash
go install claudio.click/cmd/claudio@latest
```

Make sure Go's binary directory is on `PATH`:

```bash
go env GOPATH
```

The binary normally lands in `$(go env GOPATH)/bin`.

## Auto Install

Install global hooks for every detected supported agent:

```bash
claudio install
```

This is the default: `--agent auto --scope global`. Claudio detects Claude
Code, Codex CLI, Gemini CLI, Qwen Code, and GitHub Copilot CLI from installed
commands, settings directories, or existing Claudio hook files. If more than one
supported agent is detected, each matching hook set is installed.

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

Global scope writes `~/.claude/settings.json` on every platform. On Windows that
resolves through the native user profile path.

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

After installing Codex hooks, run `/hooks` in Codex and trust the Claudio hook.

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

Project scope writes `./.github/copilot/settings.local.json`.

## Inspect Before Writing

Dry run:

```bash
claudio install --dry-run
claudio install --agent claude --scope global --dry-run
claudio install --agent codex --scope global --dry-run
claudio install --agent gemini --scope global --dry-run
claudio install --agent qwen --scope global --dry-run
claudio install --agent copilot --scope global --dry-run
```

Print the target path and mode:

```bash
claudio install --print
```

Quiet mode:

```bash
claudio install --quiet
```

The installer takes an advisory lock around the read, merge, write, and verify
cycle. It preserves non-Claudio hooks and replaces prior Claudio hook entries
with the current generated form.

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

`MessageDisplay` and `FileChanged` are registered but disabled by default.
Enable them manually only if you want audio for streamed text or broad file
change events.

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
| Claude Code | `~/.claude/commands/claudio.md` |
| Codex | `$HOME/.agents/skills/claudio/SKILL.md` |
| Antigravity | `~/.gemini/config/skills/claudio/SKILL.md` and `~/.gemini/antigravity-cli/skills/claudio.md` |

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

Use `--dry-run`, `--print`, or `--quiet` the same way as `install`.

## Next

- [Configuration](configuration)
- [CLI Reference](cli-reference)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
