---
layout: default
title: "Soundpacks"
---

# Soundpacks

A soundpack maps Claudio sound keys to audio files. Soundpacks can be
directories, JSON files, or git repositories managed by Claudio.

Supported audio formats are:

- WAV
- MP3
- AIFF

## Categories

Claudio currently maps events into these categories:

| Category | Typical events | Directory |
| --- | --- | --- |
| Loading | `PreToolUse`, `SubagentStart` | `loading/` |
| Success | successful `PostToolUse` | `success/` |
| Error | failed `PostToolUse` | `error/` |
| Interactive | prompts, notifications, permission requests | `interactive/` |
| Completion | `Stop`, `SubagentStop` | `completion/` |
| System | session start and compaction | `system/` |

`default.wav` is the final fallback for every chain.

## Directory Soundpacks

Directory soundpacks use the sound key as a relative file path.

```text
my-pack/
  default.wav
  loading/
    git-commit-start.wav
    git-start.wav
    bash-start.wav
    loading.wav
  success/
    git-commit-success.wav
    git-success.wav
    bash-success.wav
    success.wav
  error/
    git-commit-error.wav
    git-error.wav
    bash-error.wav
    error.wav
  interactive/
    message-sent.wav
    notification.wav
    permission-request.wav
    interactive.wav
  completion/
    agent-complete.wav
    subagent-complete.wav
    completion.wav
  system/
    session-start.wav
    compacting.wav
    post-compact.wav
    system.wav
```

Install a directory pack:

```bash
claudio soundpack validate ./my-pack
claudio soundpack install ./my-pack --default
```

Directory packs are copied to:

```text
<XDG_DATA_HOME>/claudio/soundpacks/<name>/
```

## JSON Soundpacks

JSON soundpacks map sound keys to files anywhere on disk. Paths can be absolute
or relative to the JSON file.

```json
{
  "name": "system-sounds",
  "description": "Small pack using existing local sounds",
  "version": "1.0.0",
  "mappings": {
    "success/success.wav": "./sounds/success.wav",
    "error/error.wav": "/home/me/sounds/error.mp3",
    "loading/loading.wav": "./sounds/loading.wav",
    "interactive/message-sent.wav": "./sounds/message-sent.aiff",
    "default.wav": "./sounds/default.wav"
  }
}
```

Create a template:

```bash
claudio soundpack init my-pack
```

Pre-fill the template with the current platform defaults:

```bash
claudio soundpack init my-pack --from-platform
```

Install a JSON pack:

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack install ./my-pack.json --default
```

JSON packs are copied to:

```text
<XDG_DATA_HOME>/claudio/<name>.json
```

## Managed Git Soundpacks

Managed git soundpacks are cloned into Claudio's data directory and recorded in
the managed soundpack registry. Claudio adds the playable subpath to
`soundpack_paths`, so runtime resolution uses the same loader as local packs.

```bash
claudio soundpack add https://github.com/owner/repo --name my-pack --default
claudio soundpack add gh:owner/repo --subdir packs/minimal --name minimal
```

Update:

```bash
claudio soundpack update my-pack
claudio soundpack update --all
```

Remove:

```bash
claudio soundpack remove my-pack
```

Inspect:

```bash
claudio soundpack status
claudio soundpack status my-pack
```

## Fallback Chains

Fallback chains are ordered from most specific to least specific. The first
existing sound wins.

### PreToolUse

For `git commit` started through the Bash tool:

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

### PostToolUse

For a successful `git commit`:

```text
success/git-commit-success.wav
success/git-success.wav
success/bash-success.wav
success/tool-complete.wav
success/success.wav
default.wav
```

For a failed `git commit`, the category changes:

```text
error/git-commit-error.wav
error/git-error.wav
error/bash-error.wav
error/tool-complete.wav
error/error.wav
default.wav
```

### Simple Events

For `UserPromptSubmit`:

```text
interactive/message-sent.wav
interactive/prompt-submit.wav
interactive/interactive.wav
default.wav
```

For `Stop`:

```text
completion/agent-complete.wav
completion/stop.wav
completion/completion.wav
default.wav
```

For `PreCompact`:

```text
system/compacting.wav
system/pre-compact.wav
system/system.wav
default.wav
```

## Command Parsing

For Bash tool events, Claudio parses the command string and recognizes common
subcommands for tools such as:

- `git`
- `npm`
- `docker`
- `cargo`
- `go`
- `pip`
- `yarn`
- `kubectl`

Unknown commands are handled conservatively. Claudio still tries command-level
sounds such as `loading/systemctl-start.wav` when the words look like a command
and subcommand rather than file paths or URLs.

MCP tool names beginning with `mcp__` are normalized to `mcp` for sound lookup.

## Validation

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack validate ./my-pack
```

Validation reports:

- Total known-key coverage
- Coverage by category
- Broken JSON references
- Unsupported file extensions
- Empty mappings

Broken references fail validation. Empty mappings do not.

## Use Tracking To Improve A Pack

Enable tracking, use Claudio normally, then inspect missing sounds:

```bash
claudio analyze missing --preset all-time --limit 50
```

The most frequent missing keys are usually the best next sounds to add.

## See Also

- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Examples](examples)
- [Troubleshooting](troubleshooting)
