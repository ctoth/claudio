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

## Claude Code Hooks

Install user-wide Claude Code hooks:

```bash
claudio install --agent claude --scope user
```

Install hooks only for the current project:

```bash
claudio install --agent claude --scope project
```

User scope writes `~/.claude/settings.json` on every platform. On Windows that
resolves through the native user profile path.

Project scope writes `./.claude/settings.json`.

## Codex Hooks

Install user-wide Codex hooks:

```bash
claudio install --agent codex --scope user
```

Install hooks only for the current project:

```bash
claudio install --agent codex --scope project
```

User scope uses `$CODEX_HOME/hooks.json` when `CODEX_HOME` is set, otherwise
`~/.codex/hooks.json`.

Project scope writes `./.codex/hooks.json`.

After installing Codex hooks, run `/hooks` in Codex and trust the Claudio hook.

## Inspect Before Writing

Dry run:

```bash
claudio install --agent claude --scope user --dry-run
claudio install --agent codex --scope user --dry-run
```

Print the target path and mode:

```bash
claudio install --agent claude --scope user --print
```

Quiet mode:

```bash
claudio install --agent claude --scope user --quiet
```

The installer takes an advisory lock around the read, merge, write, and verify
cycle. It preserves non-Claudio hooks and replaces prior Claudio hook entries
with the current generated form.

## Installed Hook Sets

Claude Code defaults:

| Hook | Category |
| --- | --- |
| `PreToolUse` | loading |
| `PostToolUse` | success or error |
| `UserPromptSubmit` | interactive |
| `Notification` | interactive |
| `Stop` | completion |
| `SubagentStop` | completion |
| `PreCompact` | system |
| `SessionStart` | system |

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
claudio uninstall --agent claude --scope user
claudio uninstall --agent codex --scope user
```

Use `--dry-run`, `--print`, or `--quiet` the same way as `install`.

## Next

- [Configuration](configuration)
- [CLI Reference](cli-reference)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
