---
layout: default
title: "Configuration"
---

# Configuration

Claudio can be configured through configuration files, environment variables, and command-line flags.

## Configuration File

Claudio follows XDG Base Directory specifications for configuration:

**Default Location:** `/etc/xdg/claudio/config.json`

**Alternative Locations:**
- `$XDG_CONFIG_HOME/claudio/config.json` (if `XDG_CONFIG_HOME` is set)
- `~/.config/claudio/config.json` (fallback)

### Configuration Format

```json
{
  "volume": 0.7,
  "default_soundpack": "default",
  "soundpack_paths": ["/usr/local/share/claudio/default"],
  "enabled": true,
  "log_level": "warn"
}
```

### Configuration Options

**volume** `float` (default: 0.7)
: Audio playback volume from 0.0 (silent) to 1.0 (maximum)
: Values outside this range are clamped to valid limits

**default_soundpack** `string` (default: "default")
: Name of the soundpack to use
: Must correspond to a directory in one of the soundpack paths

**soundpack_paths** `array of strings` (default: XDG directories)
: List of directories to search for soundpacks
: Searched in order; first match wins
: If empty, uses XDG-compliant paths: `/usr/local/share/claudio/`, `~/.local/share/claudio/`

**enabled** `boolean` (default: true)
: Whether Claudio should produce audio output
: When false, Claudio processes hooks but plays no sounds

**log_level** `string` (default: "warn")
: Logging verbosity level
: Options: `debug`, `info`, `warn`, `error`
: `debug` provides extensive logging for troubleshooting

## Environment Variables

Environment variables override configuration file settings:

**CLAUDIO_VOLUME**
: Sets audio volume (0.0 to 1.0)
```bash
export CLAUDIO_VOLUME=0.5
```

**CLAUDIO_ENABLED**
: Enable/disable Claudio entirely
```bash
export CLAUDIO_ENABLED=false  # Disable audio
export CLAUDIO_ENABLED=true   # Enable audio
```

**CLAUDIO_SOUNDPACK**
: Default soundpack to use
```bash
export CLAUDIO_SOUNDPACK=custom
```

**CLAUDIO_LOG_LEVEL**
: Logging verbosity
```bash
export CLAUDIO_LOG_LEVEL=debug
```

**XDG_CONFIG_HOME**
: Override config directory location
```bash
export XDG_CONFIG_HOME=/custom/config/path
```

**XDG_DATA_HOME**
: Override data directory location (affects soundpack search)
```bash
export XDG_DATA_HOME=/custom/data/path
```

## Command-Line Flags

Command-line flags take highest precedence:

**Direct Hook Execution:**
```bash
# Override volume for single execution
echo '...' | claudio --volume 0.8

# Run in silent mode
echo '...' | claudio --silent

# Use specific soundpack
echo '...' | claudio --soundpack retro
```

**Install/Uninstall Commands:**
```bash
# Quiet installation
claudio install --quiet --scope user

# Standard installation (overwrites existing Claudio hooks)
claudio install --scope user
```

## Priority Order

Configuration values are resolved in this order (highest to lowest priority):

1. **Command-line flags** (immediate override)
2. **Environment variables** (session-specific)
3. **Configuration file** (persistent settings)
4. **Default values** (built-in fallbacks)

## Configuration Examples

### Development Setup

For development with debug logging:

```json
{
  "volume": 0.5,
  "default_soundpack": "dev",
  "enabled": true,
  "log_level": "debug"
}
```

### Quiet Environment

For shared workspaces or focused work:

```json
{
  "volume": 0.2,
  "default_soundpack": "minimal",
  "enabled": true,
  "log_level": "error"
}
```

### Custom Soundpack Setup

Using custom soundpacks:

```json
{
  "volume": 0.8,
  "default_soundpack": "cyberpunk",
  "soundpack_paths": [
    "/home/user/my-sounds",
    "/usr/local/share/claudio"
  ],
  "enabled": true,
  "log_level": "warn"
}
```

### Temporary Disable

Temporarily disable without changing config:

```bash
export CLAUDIO_ENABLED=false
# Claudio will process hooks but play no sounds
```

## Soundpack Path Resolution

Claudio searches for soundpacks in this order:

1. **Explicit paths** from `soundpack_paths` configuration
2. **XDG data directories:**
   - `$XDG_DATA_HOME/claudio/` (if set)
   - `~/.local/share/claudio/` (user-specific)
   - `/usr/local/share/claudio/` (system-wide)
   - `/usr/share/claudio/` (system-wide fallback)

For soundpack named "custom", Claudio looks for:
- `/path/to/custom/` directory containing sound files
- Must contain at least a `default.wav` file

## Validation

Claudio validates configuration on startup:

**Volume Validation:**
- Must be between 0.0 and 1.0
- Invalid values are clamped to valid range
- Logged as warning if clamping occurs

**Soundpack Validation:**
- Specified soundpack must exist in search paths
- Falls back to "default" soundpack if not found
- Warning logged if fallback occurs

**Path Validation:**
- Soundpack paths must be readable directories
- Invalid paths are silently ignored
- Warning logged if no valid paths found

## Troubleshooting Configuration

**Check current configuration:**
```bash
export CLAUDIO_LOG_LEVEL=debug
echo '...' | claudio
# Debug output shows resolved configuration values
```

**Verify soundpack paths:**
```bash
# List available soundpacks
ls /usr/local/share/claudio/
ls ~/.local/share/claudio/
```

**Test configuration changes:**
```bash
# Test with environment override
CLAUDIO_VOLUME=0.1 echo '...' | claudio

# Test with different soundpack
CLAUDIO_SOUNDPACK=minimal echo '...' | claudio
```

**Common Issues:**

- **No sound**: Check `enabled: true` and `volume > 0.0`
- **Wrong sounds**: Verify `default_soundpack` exists in `soundpack_paths`
- **Permission errors**: Ensure config file is readable, soundpack directories accessible

## See Also

- **[Installation Guide](/installation)** - Setting up Claudio
- **[Soundpacks](/soundpacks)** - Custom sound configuration
- **[CLI Reference](/cli-reference)** - Command-line options
- **[Troubleshooting](/troubleshooting)** - Common configuration issues