---
layout: default
title: "Configuration"
---

# Configuration

Claudio works with no config file. When no file exists it uses platform-aware
defaults, including embedded platform soundpacks where available.

Configuration is resolved in this order:

1. CLI flags for a single invocation
2. Environment variables
3. The first XDG config file found
4. Built-in defaults

Use `claudio status` to see the effective runtime values after environment
overrides.

## Config File

Claudio searches XDG config paths for `claudio/config.json`. The first existing
file wins.

Typical user path:

```text
~/.config/claudio/config.json
```

On Windows, Claudio uses the Windows-native XDG location supplied by the XDG
library, not a hardcoded Unix path.

Commands that persist settings, such as `claudio volume`, `claudio mute`,
`claudio unmute`, and `claudio soundpack use`, write to the first user config
path.

## Full Example

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
  },
  "sound_tracking": {
    "enabled": true,
    "database_path": ""
  }
}
```

Every field is optional when defaults are acceptable, but `default_soundpack`
must not be empty if you set it.

## Fields

| Field | Default | Meaning |
| --- | --- | --- |
| `volume` | `0.5` | Playback volume from `0.0` to `1.0`. Invalid file values fail validation. |
| `default_soundpack` | platform-specific | Soundpack name, path, managed git name, or embedded platform id. |
| `soundpack_paths` | `[]` | Extra JSON files or directories to search in addition to XDG soundpack paths. |
| `enabled` | `true` | When false, Claudio processes hooks but plays no audio. |
| `log_level` | `warn` | `debug`, `info`, `warn`, or `error`. |
| `audio_backend` | `auto` | `auto`, `malgo`, or `system_command`. `fake` exists for tests. |
| `file_logging` | enabled | Rotated file logging configuration. |
| `sound_tracking` | enabled | SQLite tracking for usage and missing-sound analysis. |

## Environment Variables

| Variable | Effect |
| --- | --- |
| `CLAUDIO_VOLUME` | Overrides `volume`. |
| `CLAUDIO_ENABLED` | Overrides `enabled`. Accepts Go boolean forms such as `true`, `false`, `1`, and `0`. |
| `CLAUDIO_SOUNDPACK` | Overrides `default_soundpack`. |
| `CLAUDIO_LOG_LEVEL` | Overrides `log_level`. |
| `CLAUDIO_AUDIO_BACKEND` | Overrides `audio_backend` when the value is valid. |
| `CLAUDIO_FILE_LOGGING` | Enables or disables file logging for the process. |
| `CLAUDIO_SOUND_TRACKING` | Enables or disables tracking for the process. |
| `CLAUDIO_SOUND_TRACKING_DB` | Sets the tracking database path. |
| `XDG_CONFIG_HOME` | Changes user config discovery. |
| `XDG_DATA_HOME` | Changes user soundpack and managed soundpack storage. |
| `XDG_CACHE_HOME` | Changes log, tracking, and extracted embedded-sound cache storage. |

## CLI Flags

These global flags apply to direct hook execution:

```bash
claudio --config /path/to/config.json
claudio --volume 0.8
claudio --soundpack my-pack
claudio --silent
```

They are transient. They do not rewrite `config.json`.

## Persistent Controls

```bash
claudio volume          # print persisted volume
claudio volume 0.35     # persist new volume
claudio mute            # set enabled=false
claudio unmute          # set enabled=true
claudio status          # print effective config
```

If an environment variable is set, it still wins at runtime. For example,
`CLAUDIO_VOLUME=1.0` overrides a persisted `volume` of `0.35`.

## Soundpack Search

Directory soundpacks are searched under:

```text
<XDG data home>/claudio/soundpacks/<name>
<XDG data dir>/claudio/soundpacks/<name>
```

JSON soundpacks and arbitrary soundpack directories can also be added directly
to `soundpack_paths`. The soundpack install commands update this list for you.

## Logging

Stderr logging is intentionally quiet. Debug and info logs are written to the
rotated file logger when file logging is enabled.

Default log path:

```text
<XDG cache home>/claudio/logs/claudio.log
```

To debug one session:

```bash
CLAUDIO_LOG_LEVEL=debug claudio status
```

For hook playback debugging, inspect the log file after the hook fires.

## Tracking

Tracking records sound lookup chains in SQLite. It is on by default.

Default database path:

```text
<XDG cache home>/claudio/sounds.db
```

Useful reports:

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze missing --preset all-time --limit 50
```

Disable tracking:

```json
{
  "sound_tracking": {
    "enabled": false
  }
}
```

or:

```bash
CLAUDIO_SOUND_TRACKING=false claudio status
```

## Test-Only Environment Variables

These are for Claudio's own test suite. Do not set them in normal use.

| Variable | Purpose |
| --- | --- |
| `CLAUDIO_DETACH_DISABLE` | Runs hook processing synchronously for tests. |
| `CLAUDIO_TEST_RECOGNIZE_GO_TEST` | Lets install tests treat a Go test binary as the Claudio executable. |

## See Also

- [Installation](installation)
- [CLI Reference](cli-reference)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
