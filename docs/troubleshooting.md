---
layout: default
title: "Troubleshooting"
---

# Troubleshooting

Start with:

```bash
claudio status
```

That shows the config file in use, whether audio is enabled, the effective
volume, active soundpack, backend, logging, tracking, and Claudio version.

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

Then verify:

```bash
claudio --version
```

## Hooks Installed But No Sound

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

Run a manual payload:

```bash
echo '{"session_id":"debug","cwd":".","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"ok","stderr":"","interrupted":false}}' | claudio
```

If that works, the issue is likely hook registration or agent trust. If it
does not, inspect logging and audio backend configuration.

## Codex Hooks Do Nothing

After installing Codex hooks:

```bash
claudio install --agent codex --scope user
```

Run `/hooks` in Codex and trust the Claudio hook. Codex will not run an
untrusted hook.

Dry-run the target path:

```bash
claudio install --agent codex --scope user --dry-run
```

For project hooks, make sure you installed from the project root:

```bash
claudio install --agent codex --scope project --dry-run
```

## Claude Code Hooks Do Nothing

Inspect the target settings file:

```bash
claudio install --agent claude --scope user --dry-run
claudio install --agent claude --scope user --print
```

For project hooks, run from the repository root:

```bash
claudio install --agent claude --scope project --dry-run
```

Reinstalling is idempotent for Claudio hooks:

```bash
claudio install --agent claude --scope user
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

- Audio files are regular files, not symlinks.
- Extensions are `.wav`, `.mp3`, or `.aiff`.
- Paths match Claudio keys, such as `success/git-success.wav`.
- `default.wav` exists for final fallback.

## Audio Backend Errors

Show the configured backend:

```bash
claudio status
```

Try the system-command backend:

```bash
echo '{"session_id":"debug","cwd":".","hook_event_name":"Stop"}' | CLAUDIO_AUDIO_BACKEND=system_command claudio
```

`system_command` uses platform audio commands where available. On Linux, make
sure tools such as `paplay`, `ffplay`, `afplay`, or `aplay` are installed as
appropriate for your environment.

The `fake` backend is for tests. It accepts playback calls but produces no
audio.

## Debug Logs

Enable debug file logging:

```bash
export CLAUDIO_LOG_LEVEL=debug
```

Default log path:

```text
<XDG_CACHE_HOME>/claudio/logs/claudio.log
```

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
claudio uninstall --agent claude --scope user
claudio uninstall --agent codex --scope user
```

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
