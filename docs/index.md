---
layout: default
title: "Claudio - Audio Feedback for Claude Code"
description: "Add contextual sound effects to your Claude Code sessions. Hear git commits, bash commands, and tool operations with intelligent audio feedback."
keywords: "Claude Code, audio feedback, sound effects, developer tools, AI assistant, command line interface"

# Open Graph / Facebook
og:type: "website"
og:title: "Claudio - Audio Feedback for Claude Code"
og:description: "Add contextual sound effects to your Claude Code sessions. Hear git commits, bash commands, and tool operations with intelligent audio feedback."
og:url: "https://claudio.click"
og:image: "https://claudio.click/assets/claudio-preview.png"

# Twitter Card
twitter:card: "summary_large_image"
twitter:title: "Claudio - Audio Feedback for Claude Code"
twitter:description: "Add contextual sound effects to your Claude Code sessions. Hear git commits, bash commands, and tool operations with intelligent audio feedback."
twitter:image: "https://claudio.click/assets/claudio-preview.png"

# Additional Meta
canonical_url: "https://claudio.click"
---

# Claudio

Your Claude Code sessions, now with sound effects.

Spending hours watching Claude Code work gets mind-numbing. Claudio fixes that by playing sounds when stuff happens. Claude runs `git commit`? You get a sound. File operation fails? Different sound. It's like having a very quiet DJ for your AI coding session.

The cool part is how smart it gets. When Claude runs `git commit -m "fix bug"`, Claudio doesn't just play a generic "bash command" sound. It knows Claude is doing git stuff, specifically a commit, and picks sounds accordingly. If that specific sound doesn't exist, it falls back through git → bash → generic success → default. Nobody wants silence when their AI assistant's code works.

```bash
go install claudio.click/cmd/claudio@latest
claudio install
```

That's it. The install command finds your Claude Code settings and adds the hooks automatically.

Or manually configure Claude Code:

```json
{
  "hooks": {
    "PostToolUse": "claudio",
    "PreToolUse": "claudio", 
    "UserPromptSubmit": "claudio"
  }
}
```

## How Fallback Works

**For tool completion** (PostToolUse), when Claude runs `git commit`:

```
1. Look for: success/git-commit-success.wav
2. Not found? Try: success/git-success.wav  
3. Still nothing? Try: success/bash-success.wav
4. Nope? Try: success/tool-complete.wav
5. Come on: success/success.wav
6. Fine: default.wav
```

**For tool startup** (PreToolUse), the fallback is more detailed:

```
1. Look for: loading/git-commit-start.wav
2. Not found? Try: loading/git-commit.wav  
3. Still nothing? Try: loading/git-start.wav
4. Nope? Try: loading/git.wav (command-only)
5. How about: loading/bash-start.wav
6. Maybe: loading/bash.wav  
7. OK: loading/tool-start.wav
8. Come on: loading/loading.wav
9. Fine: default.wav
```

You can be as specific or as lazy as you want with your sound pack. Got a sound for every git subcommand? Great. Just want success/error/loading sounds? That works too.

## What Actually Happens

**Before a tool runs** - plays "start" sounds
- `git-commit-start.wav` if you're committing
- `git-start.wav` or `bash-start.wav` fallbacks
- `loading.wav` if nothing else matches

**After success** - plays success sounds
- `git-commit-success.wav` → `git-success.wav` → `bash-success.wav` → etc.

**After errors** - plays error sounds  
- `git-error.wav` when git fails
- `bash-error.wav` for general bash failures
- `error.wav` when all else fails

**When you send Claude a message** - plays interaction sounds
- `prompt-submit.wav` → `interactive.wav`

The parsing is pretty clever. It knows that `npm install express` should trigger npm sounds, not just bash sounds. Same with docker, cargo, go test, whatever.

## Sounds Go Here

**By default, Claudio just works.** It automatically detects your platform and uses system sounds:

- **macOS:** Uses built-in system sounds like Glass.aiff, Purr.aiff, Hero.aiff
- **Windows:** Uses Windows Media sounds (Windows Ding.wav, Windows Error.wav, etc.)  
- **WSL:** Maps to Windows sounds via `/mnt/c/Windows/Media/`
- **Linux:** Falls back to basic directory soundpack

No setup required - install and sounds work immediately.

If you want custom sounds, the default directory structure is:

```
loading/     # stuff that's about to happen
success/     # stuff that worked  
error/       # stuff that didn't work
interactive/ # you did something
default.wav  # the sound of giving up
```

You can make your own soundpack two ways:

**Directory soundpack:** Organize files the same way and point to it in the config.

**JSON soundpack:** Map virtual sound paths to any actual files on your system:

```json
{
  "name": "my-custom-sounds",
  "description": "Maps to my favorite sounds",
  "mappings": {
    "success/git-commit-success.wav": "/System/Library/Sounds/Glass.aiff",
    "success/bash-success.wav": "/usr/share/sounds/alsa/Front_Right.wav", 
    "error/bash-error.wav": "/mnt/c/Windows/Media/Windows Error.wav",
    "loading/loading.wav": "/home/user/my-sounds/thinking.mp3",
    "default.wav": "/usr/share/sounds/alsa/Front_Center.wav"
  }
}
```

This way your sounds can be anywhere on the filesystem - no need to copy or reorganize files.

## Config

Lives in `/etc/xdg/claudio/config.json`:

```json
{
  "volume": 0.5,
  "default_soundpack": "default", 
  "soundpack_paths": [],
  "enabled": true,
  "log_level": "warn",
  "audio_backend": "auto",
  "file_logging": {
    "enabled": true,
    "filename": "",
    "max_size_mb": 10,
    "max_backups": 5,
    "max_age_days": 30,
    "compress": true
  }
}
```

Or use environment variables if you're into that:

```bash
export CLAUDIO_VOLUME=0.7
export CLAUDIO_ENABLED=false  # for when you need quiet
```

Command line works too:
```bash
claudio --volume 0.8
claudio --silent  # just pretend to work
```

## Examples That Actually Work

```bash
# Claude runs: git status
# Claudio plays: loading/git-start.wav, then success/git-success.wav

# Claude runs: git commit (but forgot to stage files)
# Claudio plays: loading/git-commit-start.wav, then error/git-commit-error.wav

# Claude runs: npm test (tests fail)
# Claudio plays: loading/npm-test-start.wav, then error/npm-test-error.wav
```

The JSON that Claude Code sends looks like this:
```json
{
  "hook_event_name": "PostToolUse",
  "tool_name": "Bash", 
  "tool_input": {"command": "git commit -m 'fix'"},
  "tool_response": {"stdout": "committed", "stderr": ""}
}
```

Claudio parses that, figures out you ran git commit, checks if it worked (no stderr), and plays the right success sound.

## Technical Stuff

Uses malgo for audio because cross-platform audio in Go is otherwise a nightmare. Preloads sounds into memory so there's no delay when playing them. Supports WAV, MP3, and AIFF formats.

The command parsing knows about git, npm, docker, cargo, go, pip, yarn, kubectl, and more. It can extract subcommands so `docker build` gets different sounds than `docker run`.

Follows XDG directory specs and includes comprehensive test coverage.

## Building It

```bash
go build -o claudio .
```

Tests:
```bash
go test ./...
```

Debug what's happening:
```bash
export CLAUDIO_LOG_LEVEL=debug
echo '...' | claudio
```

## Problems?

**No sound?** Check your volume isn't zero and your audio system works.

**Wrong sounds?** Turn on debug logging and see what fallback chain it's using.

**Something broken?** Logs are at `~/.cache/claudio/logs/claudio.log`

**Silence is golden?** `export CLAUDIO_ENABLED=false`

## The Point

Every `git push` deserves a victory sound. Every failed test deserves a sad trombone. Claudio delivers both.

---

*Built with comprehensive test coverage and structured logging.*