---
layout: default
title: "Installation"
---

# Installation

Claudio installation is a two-step process: install the binary, then configure Claude Code integration.

## Step 1: Install the Binary

Install Claudio using Go's package manager:

```bash
go install claudio.click/cmd/claudio@latest
```

This downloads and builds Claudio, placing the binary in your `$GOPATH/bin` directory (usually `~/go/bin`). Make sure this directory is in your system PATH.

## Step 2: Configure Claude Code Integration

Claudio provides a simple CLI command to automatically configure Claude Code hooks:

```bash
claudio install --scope user
```

This command:
- Finds your Claude Code settings file
- Adds Claudio hooks for `PreToolUse`, `PostToolUse`, and `UserPromptSubmit` events
- Preserves any existing hooks
- Creates a backup of your settings before making changes

### Installation Scopes

You can install Claudio at different scopes:

**User Scope (Recommended)**
```bash
claudio install --scope user
```
Installs hooks in your personal Claude Code settings. Affects all Claude Code sessions for your user account.

**Project Scope**
```bash
claudio install --scope project
```
Installs hooks in the current project's Claude Code settings. Only affects Claude Code when working in this specific project directory.

### Installation Options

**Dry Run Mode**
```bash
claudio install --dry-run --scope user
```
Shows what would be installed without making any changes. Perfect for testing.

**Standard Installation**
```bash
claudio install --scope user
```
Installs or updates Claudio hooks (automatically overwrites existing Claudio hooks).

**Quiet Mode**
```bash
claudio install --quiet --scope user
```
Installs with minimal output messages.

**Print Configuration**
```bash
claudio install --print --scope user
```
Shows the configuration details that would be written.

## Verification

After installation, verify Claudio is working:

1. **Check the installation:**
   ```bash
   claudio install --dry-run --scope user
   ```
   Should show that Claudio hooks are already installed.

2. **Test audio playback:**
   ```bash
   echo '{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | claudio
   ```
   You should hear a success sound.

3. **Test with Claude Code:**
   Run any Claude Code command that uses tools. You should hear audio feedback when tools start and complete.

## Claude Code Settings Location

Claudio automatically finds your Claude Code settings in these locations:

**User Settings:**
- macOS: `~/Library/Application Support/claude-code/settings.json`
- Linux: `~/.config/claude-code/settings.json`
- Windows: `%APPDATA%\claude-code\settings.json`

**Project Settings:**
- `.claude-code/settings.json` in your project directory

## What Gets Installed

The installation adds these hooks to your Claude Code settings:

```json
{
  "hooks": {
    "PreToolUse": "claudio",
    "PostToolUse": "claudio",
    "UserPromptSubmit": "claudio"
  }
}
```

These hooks tell Claude Code to call Claudio:
- **PreToolUse**: Before running any tool (plays "thinking" sounds)
- **PostToolUse**: After tool completion (plays success/error sounds)
- **UserPromptSubmit**: When you send a message (plays interaction sounds)

## Troubleshooting Installation

**"claudio: command not found"**
- Ensure `~/go/bin` is in your PATH
- Run `go env GOPATH` to find your Go workspace
- Add `$GOPATH/bin` to your PATH in `.bashrc` or `.zshrc`

**"No Claude Code settings found"**
- Make sure Claude Code is installed and has been run at least once
- Try specifying the other scope: `--scope project` vs `--scope user`
- Check that you have permission to write to the settings directory

**Installation succeeds but no sounds**
- Check your system audio volume
- Verify audio system is working with other applications
- Run Claudio with debug logging: `export CLAUDIO_LOG_LEVEL=debug`

See the [Troubleshooting](/troubleshooting) page for more detailed solutions.

## Next Steps

- **[CLI Reference](/cli-reference)** - Complete command documentation
- **[Configuration](/configuration)** - Customize volume and behavior
- **[Soundpacks](/soundpacks)** - Learn about sound customization