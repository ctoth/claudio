# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claudio is a hook-based audio plugin for Claude Code that plays contextual sounds based on tool usage and events. It uses a 5-level fallback system to map events to sounds and supports custom soundpacks.

## Build and Development Commands

```bash
# Build the main binary
go build -o claudio .

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

# Test with debug logging to see file creation
CLAUDIO_LOG_LEVEL=debug echo '...' | claudio --config /dev/null --silent
```

## Release Process

When releasing a new version of Claudio, follow this standardized process:

### Pre-Release Checklist
1. **Version Update**: Ensure `Version` constant in `internal/cli/cli.go` is updated
2. **Clean Workspace**: Ensure no uncommitted changes that shouldn't be released
3. **Remove Old Binaries**: Delete any existing binaries to ensure fresh build

### Release Steps
```bash
# 1. Clean build environment
rm -f claudio

# 2. Run full test suite
go test ./...

# 3. Build fresh binary
go build -o claudio .

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
   - 5-level fallback system:
     1. Exact hint match: `category/hint.wav` (e.g., `success/bash-success.wav`)
     2. Tool-specific: `category/tool.wav` (e.g., `success/bash.wav`)
     3. Operation-specific: `category/operation.wav` (e.g., `success/tool-complete.wav`)
     4. Category-specific: `category/category.wav` (e.g., `success/success.wav`)
     5. Default: `default.wav`

3. **Audio System** (`internal/audio/`)
   - Uses malgo (miniaudio Go wrapper) for cross-platform audio
   - Memory-based playback with pre-loaded sounds
   - Supports WAV, MP3, and AIFF decoding with comprehensive format detection
   - AIFF support includes 16/24/32-bit depths, mono/stereo, and magic byte detection
   - Volume control with pre-processing to prevent crackling

4. **Configuration** (`internal/config/`)
   - XDG Base Directory compliant
   - Config file at `/etc/xdg/claudio/config.json`
   - Environment variable overrides: `CLAUDIO_VOLUME`, `CLAUDIO_SOUNDPACK`, etc.

5. **File Logging System** (`internal/config/`)
   - Simple idiomatic Go logging using `io.MultiWriter` + `lumberjack.v2`
   - Dual output: stderr + rotated log files
   - XDG-compliant log location: `~/.cache/claudio/logs/claudio.log`
   - Default enabled for hook-based usage (CLI flags hard to pass)
   - Graceful degradation: continues with stderr-only if file logging fails

### Key Design Decisions

1. **TDD Approach**: All components have comprehensive tests written first
2. **slog Logging**: Extensive structured logging throughout for debugging
3. **Simple v1 CLI**: Focused on core functionality, saving rich features for v2
4. **Memory-based Audio**: Pre-loads entire sound files to avoid streaming complexity

## Configuration

Default config location: `/etc/xdg/claudio/config.json`
```json
{
  "volume": 0.5,
  "default_soundpack": "default",
  "soundpack_paths": ["/usr/local/share/claudio/default"],
  "enabled": true,
  "log_level": "warn",
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

Soundpacks use a category-based directory structure with 4 main categories:
- `loading/` - PreToolUse events and thinking sounds
- `success/` - PostToolUse success events  
- `error/` - PostToolUse error events and failures
- `interactive/` - UserPromptSubmit, Notification events

Each category can contain tool-specific sounds (e.g., `bash-thinking.wav`, `read-success.wav`) that follow the 5-level fallback system. Use `ls /usr/local/share/claudio/default/` to see current available sounds.

## Current Issues and Workarounds

1. **Audio Crackling**: The current memory-based implementation has some crackling.

2. **Config Auto-discovery**: Fixed by moving config from `/usr/local/share/claudio/` to `/etc/xdg/claudio/`

## Future Development (v2)

The codebase is prepared for subcommands structure:
- `claudio dev log` - development logging mode
- `claudio analyze soundpack [name]` - analyze coverage
- `claudio soundpack report` - generate reports

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

- Hook Logger: `/root/code/claudio/cmd/hook-logger/` - captures real Claude Code hook JSON
- Test Sounds: `/root/code/claudio/sounds/` - test audio files
- Soundpack: `/usr/local/share/claudio/default/` - installed soundpack
- Log Files: `~/.cache/claudio/logs/claudio.log` - default file logging location

## File Logging System Details

### Implementation Architecture

The file logging system follows Go best practices using standard library components:

- **`io.MultiWriter`**: Combines stderr and file output streams
- **`lumberjack.v2`**: Handles log rotation, compression, and cleanup
- **`slog`**: Structured logging with configurable levels
- **XDG Base Directory**: Compliant cache directory usage

### Key Features

1. **Dual Output**: All logs go to both stderr (for immediate feedback) and file (for persistence)
2. **Automatic Rotation**: When log file reaches 10MB, creates numbered backups
3. **Cleanup**: Automatically removes logs older than 30 days or beyond 5 backup files
4. **Compression**: Rotated log files are gzipped to save disk space
5. **Directory Creation**: Automatically creates log directory if it doesn't exist
6. **Graceful Degradation**: Falls back to stderr-only if file operations fail

### Default Behavior

- **Hook Usage**: File logging defaults to **enabled** because CLI flags are difficult to pass in hook scenarios
- **Log Location**: Uses XDG cache directory (`~/.cache/claudio/logs/claudio.log`) when no custom path specified
- **Log Level**: Respects existing `log_level` configuration for both stderr and file output
- **No Regression**: All existing slog calls work unchanged

### Testing

The system includes comprehensive TDD tests covering:
- Configuration parsing and validation
- XDG path resolution
- MultiWriter setup and teardown
- End-to-end CLI integration
- Error handling and graceful degradation