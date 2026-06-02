---
layout: default
title: "Examples"
---

# Examples

These examples are meant to be copied into a terminal and adjusted for your
paths.

## Install For Claude Code

```bash
go install claudio.click/cmd/claudio@latest
claudio install --agent claude --scope user
claudio status
```

Install only for one repository:

```bash
cd /path/to/project
claudio install --agent claude --scope project
```

## Install For Codex

```bash
go install claudio.click/cmd/claudio@latest
claudio install --agent codex --scope user
```

Then open Codex, run `/hooks`, and trust the Claudio hook.

Project-only Codex install:

```bash
cd /path/to/project
claudio install --agent codex --scope project
```

## Add Agent Control Commands

Claude Code:

```bash
claudio install-commands --agent claude
```

Then use:

```text
/claudio status
/claudio volume 0.35
/claudio mute
/claudio unmute
```

Codex:

```bash
claudio install-commands --agent codex
```

Then ask Codex to use `$claudio`.

## Manually Test A Hook Payload

Prompt event:

```bash
echo '{"session_id":"manual","cwd":".","hook_event_name":"UserPromptSubmit","prompt":"test"}' | claudio
```

Successful Bash event:

```bash
echo '{"session_id":"manual","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"git status"},"tool_response":{"stdout":"clean","stderr":"","interrupted":false}}' | claudio
```

Failed Bash event:

```bash
echo '{"session_id":"manual","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_input":{"command":"npm test"},"tool_response":{"stdout":"","stderr":"tests failed","interrupted":false}}' | claudio
```

Run without audio:

```bash
echo '{"session_id":"manual","cwd":".","hook_event_name":"Stop"}' | claudio --silent
```

## Tune Volume

Persist a quieter default:

```bash
claudio volume 0.25
```

Temporarily override one invocation:

```bash
CLAUDIO_VOLUME=0.8 claudio status
```

Disable and re-enable:

```bash
claudio mute
claudio status
claudio unmute
```

## Create A Minimal JSON Soundpack

Create files:

```bash
mkdir -p ~/sounds/claudio
# Put real .wav, .mp3, or .aiff files in this directory.
```

Create `~/sounds/claudio/minimal.json`:

```json
{
  "name": "minimal",
  "description": "Small soundpack with category-level fallbacks",
  "version": "1.0.0",
  "mappings": {
    "loading/loading.wav": "./loading.wav",
    "success/success.wav": "./success.wav",
    "error/error.wav": "./error.wav",
    "interactive/interactive.wav": "./interactive.wav",
    "completion/completion.wav": "./completion.wav",
    "system/system.wav": "./system.wav",
    "default.wav": "./default.wav"
  }
}
```

Validate and install:

```bash
claudio soundpack validate ~/sounds/claudio/minimal.json
claudio soundpack install ~/sounds/claudio/minimal.json --default
```

## Create A Directory Soundpack

```bash
mkdir -p my-pack/{loading,success,error,interactive,completion,system}
cp /path/to/default.wav my-pack/default.wav
cp /path/to/loading.wav my-pack/loading/loading.wav
cp /path/to/success.wav my-pack/success/success.wav
cp /path/to/error.wav my-pack/error/error.wav
cp /path/to/interactive.wav my-pack/interactive/interactive.wav
cp /path/to/completion.wav my-pack/completion/completion.wav
cp /path/to/system.wav my-pack/system/system.wav

claudio soundpack validate ./my-pack
claudio soundpack install ./my-pack --default
```

Add specific sounds as you learn what you miss:

```text
success/git-commit-success.wav
error/npm-test-error.wav
loading/go-test-start.wav
completion/agent-complete.wav
system/session-start.wav
```

## Use Tracking To Improve A Pack

After using Claudio for a while:

```bash
claudio analyze missing --preset all-time --limit 30
```

Add sounds for the highest-requested missing keys, validate, and reinstall:

```bash
claudio soundpack validate ./my-pack
claudio soundpack install ./my-pack --default
```

Inspect actual use:

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze usage --tool Bash --preset last-week
```

## Managed Git Soundpack

```bash
claudio soundpack add gh:owner/repo --name retro --default
claudio soundpack status retro
claudio soundpack update retro
```

If the pack lives below the repository root:

```bash
claudio soundpack add gh:owner/repo --subdir packs/retro --name retro --default
```

## Debug Logging

Enable debug logs:

```bash
export CLAUDIO_LOG_LEVEL=debug
```

Run a hook payload, then inspect:

```text
<XDG cache home>/claudio/logs/claudio.log
```

For a one-off run without editing config:

```bash
CLAUDIO_LOG_LEVEL=debug claudio status
```

## See Also

- [Installation](installation)
- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
