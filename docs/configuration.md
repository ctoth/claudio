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

Claudio searches the XDG config directories for `claudio/config.json`. The
first file found wins; files are not merged with each other. `--config` skips
the search and loads the named file.

| Platform | User config (searched first) | Then |
| --- | --- | --- |
| Linux, WSL | `~/.config/claudio/config.json` | `/etc/xdg/claudio/config.json` |
| macOS | `~/Library/Application Support/claudio/config.json` | `~/Library/Preferences`, `/Library/Application Support`, `/Library/Preferences`, `~/.config` |
| Windows | `%LOCALAPPDATA%\claudio\config.json` | `%ProgramData%\claudio\config.json`, `%APPDATA%\claudio\config.json` |

`XDG_CONFIG_HOME` and `XDG_CONFIG_DIRS` override these locations on every
platform.

Commands that persist settings, such as `claudio volume`, `claudio mute`,
`claudio unmute`, and `claudio soundpack use`, write to the user config path,
or to the `--config` file if given. When the user file does not exist yet, they
seed it from the effective configuration (a system-wide file if one exists,
otherwise the defaults).

### Missing, Empty, or Broken Files

A missing config file, or an empty one such as `--config /dev/null` (`NUL` on
Windows), means defaults. A file that cannot be parsed or fails validation is
reported once on stderr as a `Warning:` line (and in the log file), and hook
mode, `claudio analyze`, and `claudio status` carry on with the defaults, so a
typo never breaks your agent's hooks. `claudio status` marks such a file as
`invalid, ignored`. Commands that write the config (`volume`, `mute`, `unmute`,
`soundpack use`) refuse instead, so they never overwrite a file you may still
want to fix.

### Partial Files

Every field is optional. A config file is read on top of the defaults, so a
file that sets only what you want to change works:

```json
{ "volume": 0.3 }
```

Fields you leave out keep their defaults, including fields inside
`file_logging` and `sound_tracking`. Explicit values always win: `"enabled":
false` mutes Claudio, and `"file_logging": null` turns file logging off.

To start from a file with every field filled in, let Claudio write one:

```bash
claudio volume 0.5
```

## Full Example

```json
{
  "volume": 0.5,
  "default_soundpack": "linux",
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

This matches the Linux defaults. Embedded packs can be named either way:
`linux` or `embedded:linux.json`, and likewise `windows`, `wsl`, and `darwin`.
Files that Claudio writes use the `embedded:` form.

## Fields

| Field | Default | Meaning |
| --- | --- | --- |
| `volume` | `0.5` | Playback volume from `0.0` to `1.0`. NaN, infinities, and out-of-range values fail validation. |
| `default_soundpack` | platform-specific | Soundpack name, path, managed git name, or embedded platform id. Required. |
| `soundpack_paths` | `[]` | Extra JSON files or directories to search in addition to XDG soundpack paths. |
| `enabled` | `true` | When false, Claudio processes hooks but plays no audio. |
| `log_level` | `warn` | `debug`, `info`, `warn`, or `error`. Controls the log file only. |
| `audio_backend` | `auto` | `auto`, `oto`, or `system_command`. |
| `file_logging` | enabled | Rotated file logging. See [Logging](#logging). |
| `sound_tracking` | enabled | SQLite tracking for usage and missing-sound analysis. See [Tracking](#tracking). |
| `forward` | off | Send events to another address instead of playing them here. See [Forwarding](#forwarding). |

The platform default for `default_soundpack` is a platform JSON file
(`windows.json`, `wsl.json`, `darwin.json`, or `linux.json`) placed next to the
`claudio` executable if one exists, otherwise the matching embedded pack. WSL
is detected separately from Linux.

`auto` selects `oto`, Claudio's built-in native backend, on Windows, macOS, and
Linux. Under WSL it selects `system_command`, which runs an external player
such as `paplay`, when one is installed, and falls back to `oto` otherwise.

An older config that sets `audio_backend` to `malgo` still loads: the removed
backend is treated as `oto` and a deprecation warning is logged. Change it to
`oto` or `auto` to silence the warning.

## Environment Variables

| Variable | Effect |
| --- | --- |
| `CLAUDIO_VOLUME` | Overrides `volume`. An out-of-range value makes hooks fail validation. |
| `CLAUDIO_ENABLED` | Overrides `enabled`. Accepts Go boolean forms such as `true`, `false`, `1`, and `0`. |
| `CLAUDIO_SOUNDPACK` | Overrides `default_soundpack`. |
| `CLAUDIO_LOG_LEVEL` | Overrides `log_level`. An unknown level makes hooks fail validation. |
| `CLAUDIO_AUDIO_BACKEND` | Overrides `audio_backend`. Invalid values are logged and ignored; `malgo` maps to `oto`. |
| `CLAUDIO_FILE_LOGGING` | Overrides `file_logging.enabled`. Accepts Go boolean forms. |
| `CLAUDIO_SOUND_TRACKING` | Overrides `sound_tracking.enabled`. Accepts Go boolean forms. |
| `CLAUDIO_SOUND_TRACKING_DB` | Overrides `sound_tracking.database_path`. |
| `CLAUDIO_FORWARD_URL` | Overrides `forward.url`. A value that is not an `http://` or `https://` address is logged and ignored. |
| `CLAUDIO_FORWARD_TOKEN` | Overrides `forward.token`. Its value is never logged. |
| `XDG_CONFIG_HOME` | Changes the user config path and the managed soundpack registry location. |
| `XDG_CONFIG_DIRS` | Changes the system config paths searched after the user path. |
| `XDG_DATA_HOME` | Changes where installed and managed soundpacks are stored. |
| `XDG_DATA_DIRS` | Changes the system directories searched for directory soundpacks. |
| `XDG_CACHE_HOME` | Changes the log and extracted embedded-sound cache locations, and on Linux the tracking database location. |

Agent settings locations used by `claudio install` also honor
`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, and `COPILOT_HOME`. See the
[CLI reference](cli-reference#claudio-install).

## CLI Flags

These global flags apply to direct hook execution:

```bash
claudio --config /path/to/config.json
claudio --volume 0.8
claudio --soundpack my-pack
claudio --silent
```

They are transient. They do not rewrite `config.json`, and `claudio status`
does not apply them (except `--config`).

## Persistent Controls

```bash
claudio volume          # print effective volume
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

`<XDG data home>` is `~/.local/share` on Linux, `~/Library/Application Support`
on macOS, and `%LOCALAPPDATA%` on Windows.

JSON soundpacks and arbitrary soundpack directories can also be added directly
to `soundpack_paths`. The soundpack install commands update this list for you.

Git soundpacks added with `claudio soundpack add` are cloned under
`<XDG data home>/claudio/soundpack-repos/<name>/` and recorded in
`<XDG config home>/claudio/soundpacks.json`.

## Logging

Only `ERROR` records reach stderr, whatever `log_level` says. `log_level`
controls what goes to the rotated log file, so debug output is only visible
there.

Default log path:

| Platform | Path |
| --- | --- |
| Linux, WSL | `~/.cache/claudio/logs/claudio.log` |
| macOS | `~/Library/Caches/claudio/logs/claudio.log` |
| Windows | `%LOCALAPPDATA%\cache\claudio\logs\claudio.log` |

`claudio status` prints the path in use.

`file_logging` fields:

| Field | Default | Meaning |
| --- | --- | --- |
| `enabled` | `true` | Write the log file. |
| `filename` | `""` | Custom log file path. Empty means the default path above. |
| `max_size_mb` | `10` | Rotate when the file reaches this size. |
| `max_backups` | `5` | Rotated files to keep. |
| `max_age_days` | `30` | Delete rotated files older than this. |
| `compress` | `true` | Gzip rotated files. |

To debug hook playback, raise the level for the agent's environment (or set
`log_level` in the config), trigger the hook, and read the log file:

```bash
export CLAUDIO_LOG_LEVEL=debug
```

## Tracking

Tracking records sound lookup chains in SQLite. It is on by default.

Default database path:

| Platform | Path |
| --- | --- |
| Linux, WSL | `~/.cache/claudio/sounds.db` (honors `XDG_CACHE_HOME`) |
| macOS | `~/Library/Caches/claudio/sounds.db` |
| Windows | `%LOCALAPPDATA%\cache\claudio\sounds.db` |

The database sits next to the `logs` directory. Earlier releases kept it
at `%LOCALAPPDATA%\claudio\sounds.db` on Windows; Claudio moves an existing
database from there the first time it runs.

Set `sound_tracking.database_path` or `CLAUDIO_SOUND_TRACKING_DB` to use a
different file.

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

## Forwarding

By default Claudio plays a sound on the machine where the hook runs. With
`forward.url` set, the hook sends the event somewhere else instead, and a
[`claudio listen`](cli-reference#claudio-listen) on the machine with the
speakers picks and plays the sound.

```json
{
  "forward": {
    "url": "http://127.0.0.1:19190/events",
    "token": ""
  }
}
```

or `CLAUDIO_FORWARD_URL` and `CLAUDIO_FORWARD_TOKEN`.

The hook makes one `POST` to the URL exactly as written, with the event as a
JSON body, and counts any `2xx` answer as delivered. Claudio does not care
what is at that address:

- A `claudio listen` web server. Its address ends in `/events`, as above.
- Anything else that takes a `POST` and hands the body on, such as a relay
  that the listening machine reads with `claudio listen --stdin`. Use the
  relay's own address, path and query.

`token`, when set, is sent as `Authorization: Bearer <token>`. For a
`claudio listen` web server it is the token that was started with; leave it
out if there is none.

A forwarding machine plays nothing itself. It needs no audio device and no
soundpack, and it records nothing in its own tracking database: the listening
machine does that. Muting it (`claudio mute`, `--silent`, `CLAUDIO_ENABLED`)
stops it sending.

What is sent is the set of names a sound is chosen from: the category, the
sound hint, the command and subcommand (for a shell command, its first words,
such as `git` and `commit`), the phase, the tool name and the operation, plus
the agent's name, the session id and the time. The prompt, the rest of the
command line, tool input and output, the working directory and the transcript
path are not sent.

If the address cannot be reached within five seconds, or answers outside
`2xx`, the hook logs a warning and still succeeds. No sound plays anywhere. A
`404` from a `claudio listen` web server usually means the URL is missing
`/events`.

Over SSH, forward the listener's port back to the laptop and point the remote
box at its own end of the tunnel:

```bash
# on the laptop
claudio listen

# connect with the port forwarded
ssh -R 19190:127.0.0.1:19190 mybox

# on mybox, in the shell rc file
export CLAUDIO_FORWARD_URL=http://127.0.0.1:19190/events
```

Across a network without a tunnel, the token and the events travel in the
clear over `http://`. Use a tunnel, or put the listener behind an `https://`
address.

When the two machines cannot reach each other at all, put something both can
reach in between. The sending side posts to it, and the listening side pipes
whatever it hands back, one event per line, into `claudio listen --stdin`:

```bash
# on the machine with the speakers
<command that prints each event the relay receives> | claudio listen --stdin
```

That side makes only an outgoing connection, so it needs no open port.

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
