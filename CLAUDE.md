# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claudio is a hook-based audio plugin for Claude Code that plays contextual sounds based on tool usage and events. It uses a 5-level fallback system to map events to sounds and supports custom soundpacks.

## Build and Development Commands

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
```

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
   - Supports WAV and MP3 decoding
   - Volume control with pre-processing to prevent crackling

4. **Configuration** (`internal/config/`)
   - XDG Base Directory compliant
   - Config file at `/etc/xdg/claudio/config.json`
   - Environment variable overrides: `CLAUDIO_VOLUME`, `CLAUDIO_SOUNDPACK`, etc.

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
  "log_level": "warn"
}
```

## Soundpack Structure

Soundpacks follow this directory structure:
```
soundpack-name/
├── default.wav
├── loading/
│   ├── loading.wav
│   ├── bash-thinking.wav
│   └── file-editing.wav
├── success/
│   ├── success.wav
│   ├── bash-success.wav
│   └── file-saved.wav
├── error/
│   ├── error.wav
│   └── tool-error.wav
└── interactive/
    ├── interactive.wav
    └── message-sent.wav
```

## Current Issues and Workarounds

1. **Audio Crackling**: The current memory-based implementation has some crackling. Volume is pre-processed during sound loading to minimize this.

2. **Config Auto-discovery**: Fixed by moving config from `/usr/local/share/claudio/` to `/etc/xdg/claudio/`

## Future Development (v2)

The codebase is prepared for subcommands structure:
- `claudio dev log` - development logging mode
- `claudio analyze soundpack [name]` - analyze coverage
- `claudio soundpack report` - generate reports

## Testing Guidelines

1. Always write tests first (TDD)
2. Use atomic commits with descriptive messages
3. Run the full test suite before committing
4. Test real audio playback with different events

## Important File Locations

- Hook Logger: `/root/code/claudio/cmd/hook-logger/` - captures real Claude Code hook JSON
- Test Sounds: `/root/code/claudio/sounds/` - test audio files
- Soundpack: `/usr/local/share/claudio/default/` - installed soundpack