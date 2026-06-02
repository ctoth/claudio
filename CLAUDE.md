# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claudio is a hook-based audio plugin for Claude Code that plays contextual sounds based on tool usage and events. It uses three event-specific fallback chains (enhanced/posttool/simple) to map events to sounds and supports custom soundpacks.

## Build and Development Commands

The project's primary development host is Windows; bash variants are kept for
Linux/macOS contributors. Use the snippet that matches your shell.

### PowerShell (Windows)

```powershell
# Build the main binary
go build -o claudio.exe ./cmd/claudio

# Install to a directory on PATH
Copy-Item claudio.exe $env:USERPROFILE\bin\

# Run tests
go test ./...

# Run specific package tests with verbose output
go test ./internal/config -v
go test ./internal/cli -v

# Test audio playback manually
'{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | .\claudio.exe

# Run with different volumes
'...' | .\claudio.exe --volume 0.7

# Use silent mode for testing without audio
'...' | .\claudio.exe --silent

# Test file logging (Windows config null is "NUL")
'...' | .\claudio.exe --config NUL --silent
Get-Content $env:LOCALAPPDATA\claudio\logs\claudio.log

# Test with debug logging
$env:CLAUDIO_LOG_LEVEL='debug'; '...' | .\claudio.exe --config NUL --silent
```

### Bash (Linux / macOS / WSL)

```bash
# Build the main binary
go build -o claudio ./cmd/claudio

# Install to system PATH
sudo cp claudio /usr/local/bin/

# Run tests
go test ./...

# Run specific package tests with verbose output
go test ./internal/config -v
go test ./internal/cli -v

# Test audio playback manually
echo '{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | claudio

# Run with different volumes
echo '...' | claudio --volume 0.7

# Use silent mode for testing without audio
echo '...' | claudio --silent

# Test file logging (with default config that enables file logging)
echo '...' | claudio --config /dev/null --silent
cat ~/.cache/claudio/logs/claudio.log

# Test with debug logging
CLAUDIO_LOG_LEVEL=debug echo '...' | claudio --config /dev/null --silent
```

## Release Process

When releasing a new version of Claudio, follow this standardized process:

### Pre-Release Checklist
1. **Version Update**: Ensure `Version` constant in `internal/cli/cli.go` is updated
2. **Clean Workspace**: Ensure no uncommitted changes that shouldn't be released
3. **Remove Old Binaries**: Delete any existing binaries to ensure fresh build

### Release Steps (PowerShell)

```powershell
# 1. Clean build environment
Remove-Item -ErrorAction SilentlyContinue claudio.exe

# 2. Run full test suite
go test ./...

# 3. Build fresh binary
go build -o claudio.exe ./cmd/claudio

# 4. Test binary works
.\claudio.exe --version

# 5. Functional smoke test
'{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | .\claudio.exe --silent

# 6. Create and push git tag
$version = (Select-String -Path internal/cli/cli.go -Pattern 'const Version').Line -replace '.*"([^"]+)".*','$1'
git tag "v$version"
git push origin "v$version"

# 7. Clean up build artifacts
Remove-Item -ErrorAction SilentlyContinue claudio.exe
```

### Release Steps (bash)

```bash
# 1. Clean build environment
rm -f claudio

# 2. Run full test suite
go test ./...

# 3. Build fresh binary
go build -o claudio ./cmd/claudio

# 4. Test binary works
./claudio --version

# 5. Functional smoke test
echo '{"session_id":"test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}' | ./claudio --silent

# 6. Create and push git tag
git tag v$(grep 'const Version' internal/cli/cli.go | cut -d'"' -f2)
git push origin v$(grep 'const Version' internal/cli/cli.go | cut -d'"' -f2)

# 7. Clean up build artifacts
rm -f claudio
```

### Post-Release
- Verify tag appears on GitHub
- Test `go install claudio.click/cmd/claudio@latest` works
- Update any documentation referencing version numbers

## Architecture Overview

### Core Components

1. **Hook System** (`internal/hooks/`)
   - Parses Claude Code hook JSON from stdin
   - Extracts context including tool names, success/error states, and file types
   - Maps events to sound categories: loading, success, error, interactive

2. **Sound Mapping** (`internal/sounds/`)
   - Three event-specific fallback chains (see `internal/sounds/mapper.go`):
     - **Enhanced** (PreToolUse with a tool name) — 9 levels: hint, command-subcommand,
       command+suffix, command-only, original-tool+suffix, original-tool, operation,
       category, default.
     - **PostTool** (PostToolUse success/error with a tool name) — 6 levels: hint,
       command+suffix, original-tool+suffix, operation, category, default. (Skips
       the command-only level for semantic accuracy.)
     - **Simple** (UserPromptSubmit, Notification, Stop, SubagentStop, PreCompact,
       and other tool-less events) — 4 levels: hint, event-specific (operation),
       category, default.

3. **Audio System** (`internal/audio/`)
   - Uses malgo (miniaudio Go wrapper) for cross-platform audio
   - Memory-based playback with pre-loaded sounds
   - Supports WAV, MP3, and AIFF decoding with comprehensive format detection
   - AIFF support includes 16/24/32-bit depths, mono/stereo, and magic byte detection
   - Volume control with pre-processing to prevent crackling

4. **Configuration** (`internal/config/`)
   - XDG Base Directory compliant
   - Config search order: `$XDG_CONFIG_HOME/claudio/config.json` first, then each
     directory in `$XDG_CONFIG_DIRS` (typically `/etc/xdg/claudio/config.json` on
     Linux/macOS). Windows uses the Windows-native XDG mapping; `/etc/xdg` is
     not checked there.
   - Environment variable precedence: CLI flag > env var > config file > default.
   - Production env vars (full reference in `docs/cli-reference.md`): `CLAUDIO_VOLUME`,
     `CLAUDIO_ENABLED`, `CLAUDIO_SOUNDPACK`, `CLAUDIO_LOG_LEVEL`,
     `CLAUDIO_AUDIO_BACKEND`, `CLAUDIO_FILE_LOGGING`, `CLAUDIO_SOUND_TRACKING`,
     `CLAUDIO_SOUND_TRACKING_DB`.
   - Test-only env vars (NEVER set in production): `CLAUDIO_DETACH_DISABLE`,
     `CLAUDIO_TEST_RECOGNIZE_GO_TEST`. Both weaken protections that exist for a
     reason; the CLI test suite is the only legitimate consumer.

5. **File Logging System** (`internal/config/`)
   - Simple idiomatic Go logging using `io.MultiWriter` + `lumberjack.v2`
   - Dual output: stderr + rotated log files
   - XDG-compliant log location: `~/.cache/claudio/logs/claudio.log`
   - Default enabled for hook-based usage (CLI flags hard to pass)
   - Graceful degradation: continues with stderr-only if file logging fails

### Key Design Decisions

1. **TDD Approach**: All components have comprehensive tests written first
2. **slog Logging**: Extensive structured logging throughout for debugging
3. **Memory-based Audio**: Pre-loads entire sound files to avoid streaming complexity

## Configuration

Config is loaded from XDG-compliant locations (see `internal/config/`). On
Linux/macOS the first hit is `$XDG_CONFIG_HOME/claudio/config.json` (typically
`~/.config/claudio/config.json`), then `/etc/xdg/claudio/config.json`. On
Windows it uses the Windows-native XDG mapping; `/etc/xdg` is never checked.

Default values (these are baked into `GetDefaultConfig`, not a literal file
shipped to users):

```json
{
  "volume": 0.5,
  "default_soundpack": "<platform-detected>",
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

`soundpack_paths` defaults to `[]` — XDG data dirs (with the hardcoded
`claudio/soundpacks/` subpath) are searched automatically.
`default_soundpack` is computed at runtime by platform detection (windows /
wsl / darwin embedded packs).

### File Logging Configuration

The file logging system supports the following configuration options:

- **`enabled`**: Boolean, whether file logging is active (default: `true`)
- **`filename`**: String, custom log file path (empty = XDG cache path)
- **`max_size_mb`**: Integer, max file size in MB before rotation (default: `10`)
- **`max_backups`**: Integer, max number of backup files to keep (default: `5`)
- **`max_age_days`**: Integer, max age in days before deletion (default: `30`)
- **`compress`**: Boolean, whether to compress rotated files (default: `true`)

When `filename` is empty (default), logs are written to `~/.cache/claudio/logs/claudio.log`.

## Soundpack Structure

Directory soundpacks use a category-based structure (e.g. `loading/`,
`success/`, `error/`, `interactive/`, `completion/`, `system/`). Each category
holds tool-specific or generic sounds resolved through the three fallback
chains documented under "Sound Mapping" above. Installed soundpacks live under
`~/.local/share/claudio/soundpacks/<id>/` (or the equivalent XDG data dir);
embedded platform packs (windows / wsl / darwin) are baked into the binary and
can be inspected via `claudio soundpack list`.

## Current Issues and Workarounds

1. **Audio Crackling**: The current memory-based implementation has some crackling.

## Shipped Subcommands

Beyond the default stdin-mode hook executor, claudio ships these subcommands
(registered in `internal/cli/cli.go`):

- `claudio install` / `claudio uninstall` — manage Claude/Codex/Antigravity hooks
- `claudio install-commands` / `claudio uninstall-commands` — manage the
  Claude Code `/claudio` slash command, Codex `$claudio` skill, or Antigravity
  skill/CLI command artifacts
- `claudio analyze usage` / `claudio analyze missing` — query the sound-tracking
  database for playback patterns and missing-sound gaps
- `claudio soundpack` — manage soundpacks:
  - `init` — create a JSON template
  - `list` — list discoverable soundpacks
  - `validate` — coverage report and broken-reference check
  - `install` — copy a local pack into the XDG data dir
  - `use` — switch the active soundpack
  - `add` / `update` / `remove` / `status` — manage git-backed soundpacks

## Development Practices - CRITICAL

### Test-Driven Development (TDD)
This project follows STRICT TDD practices:
1. **Write failing tests FIRST** - Never implement features before tests
2. **See tests fail** - Run tests to ensure they fail for the right reason
3. **Implement minimal code** - Write just enough to make tests pass
4. **Refactor** - Clean up while keeping tests green
5. **Comprehensive slog logging** - Add extensive logging in all implementations

Example TDD workflow:
```bash
# 1. Write failing test
vim internal/feature/feature_test.go
go test ./internal/feature -v  # See it fail

# 2. Implement feature
vim internal/feature/feature.go
go test ./internal/feature -v  # See it pass

# 3. Commit atomically
git add internal/feature/feature_test.go
git add internal/feature/feature.go
git commit -m "TDD: Implement feature X with comprehensive tests"
```

### Atomic Commits
Every commit should be:
1. **Self-contained** - One logical change per commit
2. **Tested** - Never commit without tests passing
3. **Specific** - Stage individual files, never use `git add .`
4. **Descriptive** - Clear commit messages explaining what and why

Commit message format:
```
TDD: Short description of what was implemented

- Bullet points for details
- What tests were added
- What functionality was implemented
- Any important design decisions
```

## Important File Locations

- Hook Logger: `cmd/hook-logger/` — captures real Claude Code hook JSON for debugging
- Embedded Soundpacks: `internal/config/embedded_soundpacks/` (windows.json, wsl.json, darwin.json)
- User-installed soundpacks: `~/.local/share/claudio/soundpacks/<id>/` (XDG data dir; Windows uses Windows-native XDG mapping)
- Log Files: `~/.cache/claudio/logs/claudio.log` — default file logging location

## File Logging System Details

### Implementation Architecture

The file logging system follows Go best practices using standard library components:

- **`io.MultiWriter`**: Combines stderr and file output streams
- **`lumberjack.v2`**: Handles log rotation, compression, and cleanup
- **`slog`**: Structured logging with configurable levels
- **XDG Base Directory**: Compliant cache directory usage

### Key Features

1. **Asymmetric Dual Output**: Logs are written to BOTH stderr and the rotated log
   file, but at different levels. The stderr handler is **hardcoded to
   `slog.LevelError`** (see `setupLogging` in `internal/cli/cli.go` around line
   725) — only ERROR records ever reach stderr regardless of `log_level`. The
   file handler honors the configured `log_level`. To see debug/info/warn
   output, tail the log file (e.g. `tail -F ~/.cache/claudio/logs/claudio.log`);
   `CLAUDIO_LOG_LEVEL=debug` on its own will not produce extra stderr output.
2. **Automatic Rotation**: When log file reaches 10MB, creates numbered backups
3. **Cleanup**: Automatically removes logs older than 30 days or beyond 5 backup files
4. **Compression**: Rotated log files are gzipped to save disk space
5. **Directory Creation**: Automatically creates log directory if it doesn't exist
6. **Graceful Degradation**: Falls back to stderr-only if file operations fail

### Default Behavior

- **Hook Usage**: File logging defaults to **enabled** because CLI flags are difficult to pass in hook scenarios
- **Log Location**: Uses XDG cache directory (`~/.cache/claudio/logs/claudio.log`) when no custom path specified
- **Log Level**: `log_level` controls the FILE handler. The stderr handler stays
  at ERROR — debug output requires tailing the log file.
- **No Regression**: All existing slog calls work unchanged

### Testing

The system includes comprehensive TDD tests covering:
- Configuration parsing and validation
- XDG path resolution
- MultiWriter setup and teardown
- End-to-end CLI integration
- Error handling and graceful degradation
