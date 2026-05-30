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
claudio install
```

**Test what would be installed:**
```bash
claudio install --dry-run
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

## claudio analyze

Analyze sound usage patterns and identify missing sounds from tracking database.

### claudio analyze usage

Show actual sound playback statistics and patterns.

```bash
claudio analyze usage [flags]
```

#### Flags

**--days** `int` (default: 7)
: Number of days to analyze (0 = all time)

**--preset** `string`
: Date range preset: `today`, `yesterday`, `last-week`, `this-month`, `all-time`

**--tool** `string`
: Filter by specific tool name (e.g., `Edit`, `Bash`)

**--category** `string`
: Filter by sound category: `success`, `error`, `loading`, `interactive`

**--limit** `int` (default: 20)
: Maximum results to show

**--show-summary**
: Display usage summary statistics

**--show-chains**
: Show per-chain-type statistics (enhanced / posttool / simple) and the
  average lookup depth each chain produced.

> **Flag renamed.** This flag was called `--show-fallbacks` in releases
> before the Chunk 12 schema migration. The old flag is gone with no
> alias — scripts that referenced it need to switch to `--show-chains`.
> The data shape also changed: `--show-fallbacks` reported a level-index
> distribution that conflated three different chain shapes;
> `--show-chains` groups by chain type, which is the actually meaningful
> axis.

#### Examples

```bash
# Recent usage overview
claudio analyze usage

# Detailed analysis with chain-type stats
claudio analyze usage --show-summary --show-chains

# Check Edit tool performance last week
claudio analyze usage --tool Edit --preset last-week

# Error sound usage this month
claudio analyze usage --category error --preset this-month
```

### claudio analyze missing

Show sounds that were requested but not found in soundpack.

```bash
claudio analyze missing [flags]
```

#### Flags

**--days** `int` (default: 7)
: Number of days to analyze (0 = all time)

**--preset** `string`
: Date range preset: `today`, `yesterday`, `last-week`, `this-month`, `all-time`

**--tool** `string`
: Filter by specific tool name

**--category** `string`
: Filter by sound category

**--limit** `int` (default: 20)
: Maximum results to show

#### Examples

```bash
# Find missing sounds from last week
claudio analyze missing --preset last-week

# Missing error sounds for Bash tool
claudio analyze missing --tool Bash --category error

# All missing sounds ever
claudio analyze missing --preset all-time --limit 50
```

## claudio soundpack

Manage soundpacks: create templates, list discoverable packs, validate, install
local packs, switch the active pack, and manage git-backed packs. The source
of truth for these subcommands is `internal/cli/soundpack_command.go` and
`internal/cli/soundpack_git.go`.

### claudio soundpack init

Create a JSON soundpack template populated with every known sound key.

```bash
claudio soundpack init <name> [flags]
```

**--dir** `string` (default: ".")
: Output directory for the soundpack file

**--from-platform** `boolean` (default: false)
: Pre-fill mapping values from the current platform's embedded soundpack
  (windows / wsl / darwin)

The output file is `<dir>/<name>.json`. Fails if the file already exists.

### claudio soundpack list

List every discoverable soundpack — embedded platform packs, packs under XDG
data directories, and any packs reachable through `soundpack_paths`.

```bash
claudio soundpack list
```

Output is a tabular `NAME / TYPE / SOUNDS / PATH` listing.

### claudio soundpack validate

Validate a JSON soundpack file or a directory soundpack and print a coverage
report.

```bash
claudio soundpack validate <path>
```

Checks:
1. JSON structure parses as a soundpack
2. Referenced files exist
3. Coverage of known sound keys
4. File extensions are `.wav`, `.mp3`, or `.aiff`

Exit code is non-zero if broken references are found. Empty mappings are
informational, not errors.

### claudio soundpack install

Copy a local JSON file or directory soundpack into the XDG data directory and
add it to `soundpack_paths`.

```bash
claudio soundpack install <path> [flags]
```

**--default** `boolean` (default: false)
: Also set the installed soundpack as `default_soundpack`

**--skip-validate** `boolean` (default: false)
: Skip validation before installing

JSON files install to `<XDG_DATA_HOME>/claudio/<name>.json`; directories
install to `<XDG_DATA_HOME>/claudio/soundpacks/<name>/`.

### claudio soundpack use

Switch the active soundpack by updating `default_soundpack` in the config
file. The name must match an installed or embedded soundpack (see
`claudio soundpack list`).

```bash
claudio soundpack use <name>
```

### claudio soundpack add

Clone a git-backed soundpack into Claudio's managed data directory. The
playable soundpack path is added to `soundpack_paths` and recorded in
`soundpacks.json`.

```bash
claudio soundpack add <git-url> [flags]
```

**--name** `string`
: Name to install the soundpack under (defaults to a name derived from the URL)

**--ref** `string`
: Branch, tag, or commit to check out

**--subdir** `string`
: Directory or JSON file within the repository to use as the soundpack

**--default** `boolean` (default: false)
: Also set the soundpack as `default_soundpack`

**--skip-validate** `boolean` (default: false)
: Skip validation before adding

**--replace** `boolean` (default: false)
: Replace an existing managed git soundpack with the same name

GitHub shorthand `gh:owner/repo` is accepted in place of the full URL.

### claudio soundpack update

Update one or all managed git soundpacks.

```bash
claudio soundpack update [name] [flags]
```

**--all** `boolean` (default: false)
: Update every managed git soundpack (mutually exclusive with `[name]`)

**--force** `boolean` (default: false)
: Discard local changes in the managed clone before updating

### claudio soundpack remove

Remove a managed git soundpack: deletes the clone, removes the registry
entry, and updates `soundpack_paths`.

```bash
claudio soundpack remove <name> [flags]
```

**--keep-files** `boolean` (default: false)
: Remove registry/config entries but leave the clone on disk

**--force** `boolean` (default: false)
: Remove registry/config entries even if clone deletion fails

### claudio soundpack status

Show the registry/clone status of one or all managed git soundpacks.

```bash
claudio soundpack status [name]
```

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

**--config** `string` (default: "")
: Path to a config file to use instead of XDG discovery
: Useful for tests and ad-hoc invocations; pass `/dev/null` (or `NUL` on
  Windows) to skip all config files and run with defaults

**--volume** `float` (default: from config)
: Audio playback volume (0.0 to 1.0)
: Overrides configuration file volume setting

**--silent** `boolean` (default: false)
: Run in silent mode (no audio output)
: Useful for testing without sound

**--soundpack** `string` (default: from config)
: Soundpack to use for audio
: Overrides configuration file soundpack setting

**--source** `string` (default: "claude")
: Hook source: `claude` / `codex` (stdin JSON carries the event) or `antigravity`

**--hook-event** `string` (default: "")
: Hook event name for sources whose payload omits it (Antigravity)

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
: Note: stderr is hardcoded to ERROR. This variable controls only the rotated
  log file (`~/.cache/claudio/logs/claudio.log` by default). Tail the file to
  see debug/info/warn output.

**CLAUDIO_AUDIO_BACKEND**
: Select the audio playback backend
: Options: `auto`, `malgo`, `system_command`, `fake`
: Example: `export CLAUDIO_AUDIO_BACKEND=system_command`
: `auto` picks the best available backend for the platform (the malgo
  CGO backend when present, otherwise the system-command backend that
  shells out to `paplay`/`ffplay`/`afplay`/`aplay`). `fake` is a
  no-audio backend used by the CLI test suite — production setting it
  silently produces zero playbacks.

**CLAUDIO_FILE_LOGGING**
: Enable or disable file logging at process startup
: Accepted values: `true`, `false`, `1`, `0`, `yes`, `no`
: Example: `export CLAUDIO_FILE_LOGGING=false`
: Overrides the `file_logging.enabled` setting in `config.json`. Useful
  for ad-hoc runs that should keep `~/.cache/claudio/logs/` untouched
  without editing the config.

**CLAUDIO_SOUND_TRACKING**
: Enable sound-tracking SQLite recording
: Accepted values: `true`, `false`
: Example: `export CLAUDIO_SOUND_TRACKING=true`
: When enabled, each resolved hook event writes one row to the sounds
  database. Required for `claudio analyze` to have any data.

**CLAUDIO_SOUND_TRACKING_DB**
: Override the sounds database path
: Example: `export CLAUDIO_SOUND_TRACKING_DB=/var/lib/claudio/sounds.db`
: Default is `~/.cache/claudio/sounds.db` (XDG cache dir).

#### Test-only environment variables

These exist solely so the test suite can drive code paths that should
never trip during real usage. Setting them in production weakens or
disables protections — do not do that.

**CLAUDIO_DETACH_DISABLE** (test-only)
: Disables the detach-to-background step at process start so tests can
  run claudio synchronously and observe its output.
: Set to `1` to disable detach.
: Production note: do not set. The detach step exists so the Claude
  Code hook returns control to the editor without waiting on audio
  playback; disabling it makes the hook block.

**CLAUDIO_TEST_RECOGNIZE_GO_TEST** (test-only)
: Makes the executable-name recognizer accept `*.test` and `*.test.exe`
  basenames so the install end-to-end test can install hooks that
  point at the running go-test binary.
: Set to `1` to enable.
: Production note: do not set. With this enabled, any unrelated
  `*.test` binary on `PATH` would be misclassified as claudio by the
  install/uninstall hook-detection paths.

**XDG_CONFIG_HOME**
: Override the XDG config directory used for `config.json` discovery
: Example: `export XDG_CONFIG_HOME=/custom/config/path`

**XDG_DATA_HOME**
: Override the XDG data directory used for soundpack discovery (Claudio
  appends the literal `claudio/soundpacks/<id>` subpath)
: Example: `export XDG_DATA_HOME=/custom/data/path`

**XDG_CACHE_HOME**
: Override the XDG cache directory used for the rotated log file
: Example: `export XDG_CACHE_HOME=/custom/cache/path`

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

Claudio writes Claude Code hooks to:

**User Scope:** `~/.claude/settings.json` (Windows: `%USERPROFILE%\.claude\settings.json`).

**Project Scope:** `./.claude/settings.json` in the current working directory.

See `internal/install/claude_settings.go` for the canonical resolution logic.

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