---
layout: default
title: "Troubleshooting"
---

# Troubleshooting

Start with:

```bash
claudio status
```

That shows the config file in use, whether audio is enabled (`MUTED` when it
is not), the effective volume, active soundpack, log level, which backend
`auto` resolved to and whether it is available, the log file path, tracking,
and Claudio version. It includes `CLAUDIO_*` environment overrides.

## `claudio: command not found`

Check where Go installed the binary:

```bash
go env GOPATH
go env GOBIN
```

Add the relevant bin directory to `PATH`, usually:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

If `GOBIN` is set, add that directory instead.

Then verify:

```bash
claudio --version
```

## Hooks Installed But No Sound

Check the `audio backend` line in `claudio status`. `available` means the
backend is compiled in, or for `system_command` that a player is on `PATH`. It
does not open the audio device, so it does not prove you will hear anything.
If the backend is unavailable, an enabled hook prints the error and exits
nonzero instead of starting its background worker, so the agent's hook output
shows the reason. Muted hooks stay quiet either way.

On Linux, check that a PulseAudio-compatible server is reachable (set
`PULSE_SERVER` when needed), or that the ALSA runtime library
`libasound.so.2` is installed. See [Installation](installation#audio-runtime).

If the output device stops taking audio without reporting an error (a
suspended PulseAudio sink, a disconnected Bluetooth or USB output), Claudio
abandons the sound about two seconds after it should have finished, and gives
up opening the device after five seconds. The log file records
`Oto playback stalled`. Reconnect or wake the device; the next hook opens it
fresh.

A config from an older release that sets `audio_backend` to `malgo` loads as
`oto` and logs a warning. Change it to `oto` or `auto` to silence the warning.

Check that Claudio is not muted:

```bash
claudio status
claudio unmute
claudio volume 0.5
```

Check environment overrides:

```bash
env | grep CLAUDIO
```

`CLAUDIO_ENABLED=false` or `CLAUDIO_VOLUME=0` can override the config file.
An invalid `CLAUDIO_AUDIO_BACKEND` value is ignored with only a warning in the
log file, so check its spelling against `auto`, `oto`, and `system_command`.

Run a manual payload:

```bash
echo '{"session_id":"debug","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"ok","stderr":"","interrupted":false}}' | claudio
```

If that works, the issue is likely hook registration or agent trust. If it
does not, inspect the [debug log](#debug-logs) and the audio backend.

If you moved or replaced the `claudio` binary after installing hooks, the hooks
still point at the old path. Run `claudio install` again with the new binary.

## No Supported Agents Detected

`claudio install` uses `--agent auto` by default. It installs hooks only for
agents Claudio can detect.

Check expected settings paths:

```bash
ls -la ~/.claude/settings.json
ls -la ~/.codex/hooks.json
ls -la ~/.gemini/settings.json
ls -la ~/.qwen/settings.json
ls -la ~/.copilot/settings.json
```

`CLAUDE_CONFIG_DIR`, `CODEX_HOME`, and `COPILOT_HOME` move these files when
set. Detection also succeeds when the agent's command is on `PATH`.

Run the target agent once if its settings directory does not exist yet, or
install explicitly:

```bash
claudio install --agent claude --scope global
claudio install --agent codex --scope global
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
```

## Codex Hooks Do Nothing

After installing Codex hooks:

```bash
claudio install --agent codex --scope global
```

Run `/hooks` in Codex and trust the Claudio hook. Codex will not run an
untrusted hook.

Dry-run the target path:

```bash
claudio install --agent codex --scope global --dry-run
```

For project hooks, make sure you installed from the project root:

```bash
claudio install --agent codex --scope project --dry-run
```

## Claude Code Hooks Do Nothing

If you use `CLAUDE_CONFIG_DIR`, make sure it has the same value when you run
`claudio install` as when you run Claude Code. Claudio writes to
`$CLAUDE_CONFIG_DIR/settings.json` when it is set and `~/.claude/settings.json`
when it is not.

Inspect the target settings file:

```bash
claudio install --agent claude --scope global --dry-run
claudio install --agent claude --scope global --print
```

For project hooks, run from the repository root:

```bash
claudio install --agent claude --scope project --dry-run
```

Reinstalling is idempotent for Claudio hooks:

```bash
claudio install --agent claude --scope global
```

## Gemini Hooks Do Nothing

Inspect the target settings file:

```bash
claudio install --agent gemini --scope global --dry-run
claudio install --agent gemini --scope global --print
```

For project hooks, run from the repository root:

```bash
claudio install --agent gemini --scope project --dry-run
```

Reinstalling is idempotent for Claudio hooks:

```bash
claudio install --agent gemini --scope global
```

## Qwen Code Hooks Do Nothing

Inspect the target settings file:

```bash
claudio install --agent qwen --scope global --dry-run
claudio install --agent qwen --scope global --print
```

For project hooks, run from the repository root:

```bash
claudio install --agent qwen --scope project --dry-run
```

Reinstalling is idempotent for Claudio hooks:

```bash
claudio install --agent qwen --scope global
```

## GitHub Copilot CLI Hooks Do Nothing

Inspect the target settings file:

```bash
claudio install --agent copilot --scope global --dry-run
claudio install --agent copilot --scope global --print
```

Project scope writes `./.github/copilot/settings.local.json`, or an existing
`./.github/copilot/settings.json`. Run it from the repository root:

```bash
claudio install --agent copilot --scope project --dry-run
```

## Wrong Sound Plays

Use tracking first:

```bash
claudio analyze usage --show-chains --show-summary
claudio analyze missing --preset all-time --limit 50
```

If a specific sound is missing, add that key to your soundpack. For example,
when `git commit` falls back to `success/git-success.wav`, add:

```text
success/git-commit-success.wav
```

Validate after changes:

```bash
claudio soundpack validate ./my-pack
```

## Custom Soundpack Not Found

List discovered soundpacks:

```bash
claudio soundpack list
```

If your pack is not listed, install it:

```bash
claudio soundpack install ./my-pack --default
```

or use a JSON path directly in config:

```json
{
  "default_soundpack": "my-pack",
  "soundpack_paths": ["/absolute/path/to/my-pack.json"]
}
```

Directory packs in the standard data location must live under:

```text
<XDG_DATA_HOME>/claudio/soundpacks/<name>/
```

## JSON Soundpack Fails Validation

Run:

```bash
claudio soundpack validate ./my-pack.json
```

Common causes:

- Referenced files do not exist.
- Relative paths are relative to the JSON file, not the shell's current directory.
- File extensions are not WAV, MP3, or AIFF.
- The JSON file is too large or malformed.

Empty mappings are allowed. Broken references fail validation.

## Directory Soundpack Fails Validation

Run:

```bash
claudio soundpack validate ./my-pack
```

Check:

- Audio files are regular files. A symlinked audio file fails validation.
- Extensions are `.wav`, `.mp3`, or `.aiff`. Other files are ignored.
- Paths match Claudio keys, such as `success/git-success.wav`.

Missing keys only lower the coverage numbers. Adding `default.wav` at the pack
root is still a good idea, since it is the last step of every fallback chain.

## Audio Backend Errors

Show the configured backend:

```bash
claudio status
```

Try the system-command backend:

```bash
echo '{"session_id":"debug","cwd":".","hook_event_name":"Stop"}' | CLAUDIO_AUDIO_BACKEND=system_command claudio
```

`system_command` runs the first player it finds on `PATH`, in this order:
`paplay`, `ffplay`, `aplay`, `afplay`. `afplay` ships with macOS; on Linux
install `pulseaudio-utils` (`paplay`) or FFmpeg (`ffplay`). On Windows only
`ffplay` applies. `aplay` plays WAV only and ignores the volume setting.

The `fake` backend is for tests. It accepts playback calls but produces no
audio.

If the agent runs on a remote machine over SSH, that box usually has no audio
device at all. Forward a PulseAudio socket instead; see
[Remote Audio Over SSH](remote-audio-ssh).

## Debug Logs

Enable debug file logging:

```bash
export CLAUDIO_LOG_LEVEL=debug
```

The level applies to the log file only. Claudio writes nothing below ERROR to
stderr, so read the file. `claudio status` prints its exact path. Defaults:

| Platform | Log file |
| --- | --- |
| Linux, WSL | `$XDG_CACHE_HOME/claudio/logs/claudio.log`, normally under `~/.cache` |
| macOS | `~/Library/Caches/claudio/logs/claudio.log` |
| Windows | `%LOCALAPPDATA%\cache\claudio\logs\claudio.log` |

Setting `XDG_CACHE_HOME` overrides the default on every platform.

If you do not want a log file for a one-off run:

```bash
CLAUDIO_FILE_LOGGING=false claudio status
```

## Tracking Has No Data

Check status:

```bash
claudio status
```

If tracking is disabled, enable it:

```bash
export CLAUDIO_SOUND_TRACKING=true
```

or in config:

```json
{
  "sound_tracking": {
    "enabled": true
  }
}
```

Then use Claudio normally and rerun:

```bash
claudio analyze usage
claudio analyze missing
```

## Remove Claudio

Remove hooks:

```bash
claudio uninstall --agent all --scope global
```

To remove one agent, pass `--agent claude`, `codex`, `gemini`, `qwen`, or
`copilot`. Repeat with `--scope project` in each repository where you installed
project hooks.

Remove optional command artifacts:

```bash
claudio uninstall-commands --agent claude
claudio uninstall-commands --agent codex
claudio uninstall-commands --agent antigravity
```

## Report An Issue

Include:

- Operating system
- `claudio --version`
- `claudio status`
- Agent and scope used for installation
- Relevant debug log excerpt
- A small hook payload that reproduces the issue, if possible

## See Also

- [Installation](installation)
- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Soundpacks](soundpacks)
- [Remote Audio Over SSH](remote-audio-ssh)
