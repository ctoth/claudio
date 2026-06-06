---
layout: default
title: "Claudio"
description: "Hook-based audio feedback for Claude Code, OpenAI Codex CLI, Gemini CLI, Qwen Code, and GitHub Copilot CLI."
keywords: "Claude Code, Codex CLI, Gemini CLI, Qwen Code, GitHub Copilot CLI, hooks, audio feedback, soundpacks, developer tools"
canonical_url: "https://claudio.click"
---

# Claudio

Claudio adds contextual audio feedback to coding-agent sessions. It runs as a
hook command, reads the hook event JSON from stdin, chooses a sound through a
fallback chain, starts playback in the background, and returns control to the
agent quickly.

It is built for long agent runs where watching the terminal is wasteful but
silent failure is also bad. A prompt submission, tool start, successful edit,
failed test, permission request, context compaction, and final response can all
sound different.

## Quick Start

```bash
go install claudio.click/cmd/claudio@latest
claudio install
claudio status
```

`claudio install` uses `--agent auto --scope global` by default. It detects
Claude Code, Codex CLI, Gemini CLI, Qwen Code, and GitHub Copilot CLI, then
installs hooks for the agents it finds.

For Codex, trust the hook after installation:

```bash
/hooks
```

## What Claudio Installs

`claudio install` writes hook entries into the selected agent's settings file
and preserves non-Claudio hooks.

| Agent | Command | Global settings | Project settings |
| --- | --- | --- | --- |
| Claude Code | `claudio install --agent claude` | `~/.claude/settings.json` | `./.claude/settings.json` |
| Codex | `claudio install --agent codex` | `$CODEX_HOME/hooks.json` or `~/.codex/hooks.json` | `./.codex/hooks.json` |
| Gemini | `claudio install --agent gemini` | `~/.gemini/settings.json` | `./.gemini/settings.json` |
| Qwen Code | `claudio install --agent qwen` | `~/.qwen/settings.json` | `./.qwen/settings.json` |
| GitHub Copilot CLI | `claudio install --agent copilot` | `~/.copilot/settings.json` | `./.github/copilot/settings.local.json` |

Antigravity support is command-artifact only:

```bash
claudio install-commands --agent antigravity
```

## Event Coverage

Claude Code installs these default-enabled hooks:

- `PreToolUse`
- `PostToolUse`
- `UserPromptSubmit`
- `Notification`
- `Stop`
- `SubagentStop`
- `PreCompact`
- `SessionStart`

Codex installs these default-enabled hooks:

- `PreToolUse`
- `PostToolUse`
- `UserPromptSubmit`
- `Stop`
- `SubagentStop`
- `SubagentStart`
- `PreCompact`
- `PostCompact`
- `SessionStart`
- `PermissionRequest`

Gemini installs these default-enabled hooks:

- `BeforeTool`
- `AfterTool`
- `BeforeAgent`
- `AfterAgent`
- `BeforeModel` (silent no-op)
- `AfterModel` (silent no-op)
- `BeforeToolSelection` (silent no-op)
- `SessionStart`
- `SessionEnd`
- `Notification`
- `PreCompress`

Qwen Code installs these default-enabled hooks:

- `PreToolUse`
- `PostToolUse`
- `PostToolUseFailure`
- `UserPromptSubmit`
- `SessionStart`
- `SessionEnd`
- `Stop`
- `StopFailure`
- `SubagentStart`
- `SubagentStop`
- `PreCompact`
- `PostCompact`
- `Notification`
- `PermissionRequest`

## How Sound Selection Works

For a Bash tool call like `git commit -m "fix"`, Claudio extracts `git` as the
command and `commit` as the subcommand. It then walks a most-specific to
least-specific chain.

Pre-tool loading chain:

```text
loading/git-commit-start.wav
loading/git-commit.wav
loading/git-start.wav
loading/git.wav
loading/bash-start.wav
loading/bash.wav
loading/tool-start.wav
loading/loading.wav
default.wav
```

Post-tool success or failure chain:

```text
success/git-commit-success.wav
success/git-success.wav
success/bash-success.wav
success/tool-complete.wav
success/success.wav
default.wav
```

Simple events such as prompts, notifications, completion, and compaction use
event-specific chains under `interactive/`, `completion/`, or `system/`.

## Everyday Control

```bash
claudio status
claudio volume 0.5
claudio mute
claudio unmute
claudio soundpack list
claudio analyze missing
```

Install an optional in-agent control command:

```bash
claudio install-commands --agent claude
claudio install-commands --agent codex
```

Claude Code gets `/claudio`. Codex gets a `$claudio` skill.

## Configuration

Claudio works without a config file. Defaults are platform-aware: Windows,
macOS, WSL, and Linux get embedded or system-backed sound mappings when
available.

Persistent configuration lives at the first XDG config path, normally
`~/.config/claudio/config.json` on Unix-like systems and the platform XDG
equivalent on Windows.

The fastest way to inspect the active result is:

```bash
claudio status
```

See [Configuration](configuration) for every field and override.

## Documentation

- [Installation](installation)
- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Soundpacks](soundpacks)
- [Examples](examples)
- [Troubleshooting](troubleshooting)
