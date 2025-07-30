---
layout: default
title: "Home"
---

# Claudio

**Your Claude Code sessions, now with sound effects.**

Spending hours watching Claude Code work can be mind-numbing. Claudio fixes that by playing contextual sounds when things happen. Claude runs `git commit`? You get a sound. File operation fails? Different sound. It's like having a very quiet DJ for your AI coding session.

## Quick Start

Install Claudio in two simple steps:

```bash
# 1. Install the binary
go install github.com/ctoth/claudio@latest

# 2. Set up Claude Code integration
claudio install --scope user
```

That's it! Claudio will now play sounds for your Claude Code tool usage.

## Smart Sound Mapping

The cool part is how smart it gets. When Claude runs `git commit -m "fix bug"`, Claudio doesn't just play a generic "bash command" sound. It knows Claude is doing git stuff, specifically a commit, and picks sounds accordingly.

**5-Level Fallback System:**
1. Look for: `success/git-commit-success.wav`
2. Not found? Try: `success/git-success.wav`  
3. Still nothing? Try: `success/bash-success.wav`
4. Nope? Try: `success/tool-complete.wav`
5. Come on: `success/success.wav`
6. Fine: `default.wav`

This means you can be as specific or as lazy as you want with your sound pack. Got a sound for every git subcommand? Great. Just want success/error/loading sounds? That works too.

## Key Features

- **Context-Aware Sounds**: Different sounds for git, npm, docker, and 20+ other tools
- **Easy CLI Installation**: Simple `claudio install` command handles Claude Code integration
- **Smart Fallback System**: 5-level hierarchy ensures you always get appropriate audio feedback
- **Zero Latency**: Memory-based audio playback with no delay
- **Customizable**: Create your own soundpacks or use the defaults

## What Gets Sounds?

**Before a tool runs** - "thinking" sounds:
- `git-commit-thinking.wav` when committing
- `bash-thinking.wav` for general bash  
- `loading.wav` if nothing else matches

**After success** - success sounds:
- `git-commit-success.wav` → `git-success.wav` → `bash-success.wav` → etc.

**After errors** - error sounds:  
- `git-error.wav` when git fails
- `bash-error.wav` for general bash failures
- `error.wav` when all else fails

**When you send Claude a message** - interaction sounds:
- `message-sent.wav` → `interactive.wav`

## Next Steps

- **[Installation Guide](/installation)** - Detailed setup instructions
- **[CLI Reference](/cli-reference)** - Complete command documentation  
- **[Soundpacks](/soundpacks)** - Learn about the fallback system and custom sounds
- **[Configuration](/configuration)** - Customize volume, paths, and behavior

---

*Built with TDD because tests matter and structured logging because debugging matters.*