---
layout: default
title: "CLI Reference"
---

# CLI Reference

Complete documentation for all Claudio commands and options.

## claudio install

Installs Claudio hooks into Claude Code settings to enable audio feedback.

### Usage

```bash
claudio install [flags]
```

### Flags

**--scope, -s** `string` (default: "user")
: Installation scope for hooks
: **Options:** `user`, `project`
: **user:** Install in personal Claude Code settings (affects all sessions)
: **project:** Install in current project's Claude Code settings (project-specific)

**--dry-run, -d** `boolean` (default: false)
: Show what would be done without making changes (simulation mode)
: Useful for testing installation before committing changes


**--quiet, -q** `boolean` (default: false)
: Suppress output (no progress messages)
: Shows only essential information and errors

**--print, -p** `boolean` (default: false)
: Print configuration that would be written
: Shows installation details without performing installation

### Examples

**Basic installation:**
```bash
claudio install --scope user
```

**Test what would be installed:**
```bash
claudio install --dry-run --scope user
```

**Standard installation (overwrites existing Claudio hooks):**
```bash
claudio install --scope user
```

**Project-specific installation:**
```bash
claudio install --scope project
```

**Silent installation:**
```bash
claudio install --quiet --scope user
```

**Show configuration details:**
```bash
claudio install --print --scope user
```

### Installation Process

The install command performs these steps:

1. **Find Claude Code settings** - Locates settings file for specified scope
2. **Read existing settings** - Safely reads current configuration with file locking
3. **Generate Claudio hooks** - Creates hooks for PreToolUse, PostToolUse, UserPromptSubmit
4. **Merge configurations** - Combines Claudio hooks with existing settings
5. **Write updated settings** - Saves merged configuration with file locking
6. **Verify installation** - Confirms all hooks were installed correctly

## claudio uninstall

Removes Claudio hooks from Claude Code settings to disable audio feedback.

### Usage

```bash
claudio uninstall [flags]
```

### Flags

**--scope, -s** `string` (default: "user")
: Uninstall scope for hook removal
: **Options:** `user`, `project`
: **user:** Remove from personal Claude Code settings
: **project:** Remove from current project's Claude Code settings

**--dry-run, -d** `boolean` (default: false)
: Show what would be removed without making changes (simulation mode)
: Lists hooks that would be removed


**--quiet, -q** `boolean` (default: false)
: Suppress output (no progress messages)
: Shows only essential information and errors

**--print, -p** `boolean` (default: false)
: Print hooks that would be removed
: Shows uninstall details without performing removal

### Examples

**Basic uninstall:**
```bash
claudio uninstall --scope user
```

**Test what would be removed:**
```bash
claudio uninstall --dry-run --scope user
```

**Standard removal:**
```bash
claudio uninstall --scope user
```

**Project-specific removal:**
```bash
claudio uninstall --scope project
```

**Show what hooks would be removed:**
```bash
claudio uninstall --print --scope user
```

### Uninstall Process

The uninstall command performs these steps:

1. **Find Claude Code settings** - Locates settings file for specified scope
2. **Read existing settings** - Safely reads current configuration
3. **Detect Claudio hooks** - Identifies which hooks belong to Claudio
4. **Remove Claudio hooks** - Removes only Claudio-specific hook entries
5. **Write updated settings** - Saves configuration with Claudio hooks removed
6. **Verify removal** - Confirms all Claudio hooks were successfully removed

## claudio (hook execution)

Claudio can also be called directly as a hook processor (this is what Claude Code calls automatically).

### Usage

```bash
echo '{"hook_event_name":"PostToolUse",...}' | claudio [flags]
```

### Hook Execution Flags

**--volume** `float` (default: from config)
: Audio playback volume (0.0 to 1.0)
: Overrides configuration file volume setting

**--silent** `boolean` (default: false)
: Run in silent mode (no audio output)
: Useful for testing without sound

**--soundpack** `string` (default: from config)
: Soundpack to use for audio
: Overrides configuration file soundpack setting

### Environment Variables

Claudio respects these environment variables:

**CLAUDIO_VOLUME**
: Sets audio volume (0.0 to 1.0)
: Example: `export CLAUDIO_VOLUME=0.7`

**CLAUDIO_ENABLED**
: Enable/disable Claudio entirely
: Example: `export CLAUDIO_ENABLED=false`

**CLAUDIO_SOUNDPACK**
: Default soundpack to use
: Example: `export CLAUDIO_SOUNDPACK=custom`

**CLAUDIO_LOG_LEVEL**
: Logging verbosity
: Options: `debug`, `info`, `warn`, `error`
: Example: `export CLAUDIO_LOG_LEVEL=debug`

## Global Options

These options work with all commands:

**--help, -h**
: Show help for the command

**--version**
: Show Claudio version information

## Exit Codes

- **0**: Success
- **1**: General error (invalid arguments, file not found, etc.)
- **2**: Configuration error (invalid config file, missing settings)
- **3**: Audio system error (no audio device, playback failure)

## Settings File Locations

Claudio automatically discovers Claude Code settings:

**User Scope:**
- macOS: `~/Library/Application Support/claude-code/settings.json`
- Linux: `~/.config/claude-code/settings.json`  
- Windows: `%APPDATA%\claude-code\settings.json`

**Project Scope:**
- `.claude-code/settings.json` in current directory or parent directories

## Hook Events

Claudio responds to these Claude Code hook events:

**PreToolUse**
: Triggered before Claude Code runs a tool
: Plays "thinking" or "loading" sounds

**PostToolUse**
: Triggered after Claude Code completes a tool
: Plays "success" or "error" sounds based on tool result

**UserPromptSubmit**
: Triggered when you send a message to Claude Code
: Plays "interactive" or "message-sent" sounds

## See Also

- **[Installation Guide](/installation)** - Step-by-step setup
- **[Configuration](/configuration)** - Config file and environment options
- **[Soundpacks](/soundpacks)** - Sound customization and fallback system
- **[Troubleshooting](/troubleshooting)** - Common issues and solutions