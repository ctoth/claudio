---
layout: default
title: "Home"
---

# Claudio

Your Claude Code sessions, now with sound effects.

Spending hours watching Claude Code work gets mind-numbing. Claudio fixes that by playing sounds when stuff happens. Claude runs `git commit`? You get a sound. File operation fails? Different sound. It's like having a very quiet DJ for your AI coding session.

The cool part is how smart it gets. When Claude runs `git commit -m "fix bug"`, Claudio doesn't just play a generic "bash command" sound. It knows Claude is doing git stuff, specifically a commit, and picks sounds accordingly. If that specific sound doesn't exist, it falls back through git → bash → generic success → default. Nobody wants silence when their AI assistant's code works.

```bash
go install github.com/ctoth/claudio@latest
```

Then tell Claude Code about it in your config:

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

When Claude runs `git commit`:

```
1. Look for: success/git-commit-success.wav
2. Not found? Try: success/git-success.wav  
3. Still nothing? Try: success/bash-success.wav
4. Nope? Try: success/tool-complete.wav
5. Come on: success/success.wav
6. Fine: default.wav
```

You can be as specific or as lazy as you want with your sound pack. Got a sound for every git subcommand? Great. Just want success/error/loading sounds? That works too.

## What Actually Happens

**Before a tool runs** - plays "thinking" sounds
- `git-commit-thinking.wav` if you're committing
- `bash-thinking.wav` if it's just bash  
- `loading.wav` if nothing else matches

**After success** - plays success sounds
- `git-commit-success.wav` → `git-success.wav` → `bash-success.wav` → etc.

**After errors** - plays error sounds  
- `git-error.wav` when git fails
- `bash-error.wav` for general bash failures
- `error.wav` when all else fails

**When you send Claude a message** - plays interaction sounds
- `message-sent.wav` → `interactive.wav`

The parsing is pretty clever. It knows that `npm install express` should trigger npm sounds, not just bash sounds. Same with docker, cargo, go test, whatever.

## Sounds Go Here

Default soundpack lives in `/usr/local/share/claudio/default/`:

```
loading/     # stuff that's about to happen
success/     # stuff that worked  
error/       # stuff that didn't work
interactive/ # you did something
default.wav  # the sound of giving up
```

You can make your own soundpack. Just organize files the same way and point to it in the config.

## Config

Lives in `/etc/xdg/claudio/config.json`:

```json
{
  "volume": 0.5,
  "default_soundpack": "default", 
  "soundpack_paths": ["/usr/local/share/claudio/default"],
  "enabled": true,
  "log_level": "warn"
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
# Claudio plays: loading/git-thinking.wav, then success/git-success.wav

# Claude runs: git commit (but forgot to stage files)
# Claudio plays: loading/git-commit-thinking.wav, then error/git-commit-error.wav

# Claude runs: npm test (tests fail)
# Claudio plays: loading/npm-test-thinking.wav, then error/npm-test-error.wav
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

Uses malgo for audio because cross-platform audio in Go is otherwise a nightmare. Preloads sounds into memory so there's no delay when playing them. Supports WAV and MP3.

The command parsing knows about git, npm, docker, cargo, go, pip, yarn, kubectl, and more. It can extract subcommands so `docker build` gets different sounds than `docker run`.

Built with proper tests because I'm not insane. Follows XDG directory specs because I'm not a monster either.

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

**Crackling?** Lower the volume. The audio processing isn't perfect yet.

**Silence is golden?** `export CLAUDIO_ENABLED=false`

## The Point

Every `git push` deserves a victory sound. Every failed test deserves a sad trombone. Claudio delivers both.

---

*Built with TDD because tests matter and structured logging because debugging matters.*