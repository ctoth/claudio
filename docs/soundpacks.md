---
layout: default
title: "Soundpacks"
---

# Soundpacks

A soundpack maps Claudio sound keys to audio files. Soundpacks can be
directories, JSON files, or git repositories managed by Claudio.

Supported audio formats are:

- WAV
- MP3
- AIFF

This page has three parts: using a pack someone else made (or one that's
already built in), building your own, and — for anyone extending Claudio
itself — how sound selection actually works under the hood.

## Categories

Claudio currently maps events into these categories:

| Category | Typical events | Directory |
| --- | --- | --- |
| Loading | `PreToolUse`, `SubagentStart` | `loading/` |
| Success | successful `PostToolUse` | `success/` |
| Error | failed `PostToolUse` | `error/` |
| Interactive | prompts, notifications, permission requests | `interactive/` |
| Completion | `Stop`, `SubagentStop` | `completion/` |
| System | session start and compaction | `system/` |

`default.wav` is the final fallback for every chain.

## Where Soundpacks Live

Claudio resolves soundpack storage through XDG base directories. On Linux and
WSL that means `~/.local/share`; on macOS, `~/Library/Application Support`; on
Windows, `%LOCALAPPDATA%` (both data and config home default to the same
folder there).

| Item | Linux | macOS | Windows | WSL |
| --- | --- | --- | --- | --- |
| Directory packs | `~/.local/share/claudio/soundpacks/<name>/` | `~/Library/Application Support/claudio/soundpacks/<name>/` | `%LOCALAPPDATA%\claudio\soundpacks\<name>\` | `~/.local/share/claudio/soundpacks/<name>/` |
| JSON packs | `~/.local/share/claudio/<name>.json` | `~/Library/Application Support/claudio/<name>.json` | `%LOCALAPPDATA%\claudio\<name>.json` | `~/.local/share/claudio/<name>.json` |
| Managed git clones | `~/.local/share/claudio/soundpack-repos/<name>/` | `~/Library/Application Support/claudio/soundpack-repos/<name>/` | `%LOCALAPPDATA%\claudio\soundpack-repos\<name>\` | `~/.local/share/claudio/soundpack-repos/<name>/` |
| Managed git registry | `~/.config/claudio/soundpacks.json` | `~/Library/Application Support/claudio/soundpacks.json` | `%LOCALAPPDATA%\claudio\soundpacks.json` | `~/.config/claudio/soundpacks.json` |
| `config.json` | `~/.config/claudio/config.json` | `~/Library/Application Support/claudio/config.json` | `%LOCALAPPDATA%\claudio\config.json` | `~/.config/claudio/config.json` |

`XDG_DATA_HOME` and `XDG_CONFIG_HOME` override these on every platform,
including Windows.

Embedded platform packs (`windows`, `wsl`, `darwin`, `linux`) ship baked into
the `claudio` binary. They need no install step; `claudio soundpack list`
shows them with an `embedded` type. WSL gets its own `wsl.json` pack instead
of `linux.json` even though it runs the Linux binary, because WSL sessions
typically play audio through a Windows-side player.

## Using A Pack

Nothing here requires writing a soundpack yourself — this is for picking
between built-in packs or installing one someone else made.

### Switching The Active Soundpack

```bash
claudio soundpack list
claudio soundpack use <name>
```

`soundpack use` writes `default_soundpack` in `config.json`. `<name>` must
already appear in `claudio soundpack list` — embedded, directory, JSON, and
managed git packs are all valid targets.

`soundpack use` only checks that the name is *listed*, not that it will
actually resolve at runtime — see
[Discovery Vs. Runtime Resolution](#discovery-vs-runtime-resolution) under
Directory Soundpacks below. For packs installed with `soundpack install` or
`soundpack add` this is a non-issue since both commands also wire up
`soundpack_paths` or the git registry. It only bites hand-copied packs.

A one-off override without touching config:

```bash
claudio --soundpack <name>
CLAUDIO_SOUNDPACK=<name> claudio
```

### Installing Someone Else's Pack

Managed git soundpacks are cloned into Claudio's data directory and recorded in
the managed soundpack registry. Claudio adds the playable subpath to
`soundpack_paths`, so runtime resolution uses the same loader as local packs.

There is no central marketplace or public index of soundpacks. "Registry"
here means the local `soundpacks.json` bookkeeping file that tracks packs
*you* installed (name, source URL, ref, commit) so `update`/`remove`/`status`
have something to act on. Sharing a pack with someone else just means giving
them a git URL — any public or private repo containing a directory or JSON
soundpack works, and installing one doesn't require understanding the pack
formats below at all:

```bash
claudio soundpack add https://github.com/owner/repo --name my-pack --default
claudio soundpack add gh:owner/repo --subdir packs/minimal --name minimal
```

Update:

```bash
claudio soundpack update my-pack
claudio soundpack update --all
```

Updates are pull-on-demand only. Claudio never fetches in the background or
during normal hook processing — a managed pack stays pinned at whatever
commit it was cloned or last updated to until you run `soundpack update`
again yourself (e.g. from your own cron job, if you want that).

Inspect:

```bash
claudio soundpack status
claudio soundpack status my-pack
```

### Removing A Pack

How you remove a pack depends on how it was installed:

- **Managed git packs** — use the CLI. It deletes the clone and the registry
  entry together:

  ```bash
  claudio soundpack remove my-pack
  claudio soundpack remove my-pack --keep-files   # drop registry entry, keep the clone
  claudio soundpack remove my-pack --force        # drop registry/config entries even if clone deletion fails
  ```

- **Directory and JSON packs installed with `soundpack install`** — there is
  no dedicated remove command. Delete the installed path by hand and, for
  JSON packs, drop the matching entry from `soundpack_paths` in
  `config.json`:

  ```bash
  # directory pack
  rm -rf <XDG_DATA_HOME>/claudio/soundpacks/my-pack

  # JSON pack — also remove its soundpack_paths entry in config.json
  rm <XDG_DATA_HOME>/claudio/my-pack.json
  ```

If the removed pack was `default_soundpack`, set a new one with
`claudio soundpack use <name>` or edit `config.json` directly — otherwise
Claudio falls back to the platform default the next time a sound is resolved.

## Building A Pack

### Directory Soundpacks

Directory soundpacks use the sound key as a relative file path.

```text
my-pack/
  default.wav
  loading/
    git-commit-start.wav
    git-start.wav
    bash-start.wav
    loading.wav
  success/
    git-commit-success.wav
    git-success.wav
    bash-success.wav
    success.wav
  error/
    git-commit-error.wav
    git-error.wav
    bash-error.wav
    error.wav
  interactive/
    message-sent.wav
    notification.wav
    permission-request.wav
    interactive.wav
  completion/
    agent-complete.wav
    subagent-complete.wav
    completion.wav
  system/
    session-start.wav
    compacting.wav
    post-compact.wav
    system.wav
```

Not sure exactly what to name a file? See
[Fallback Chains](#fallback-chains) below — the categorized folders plus
descriptive names above are enough to get started, but the chains explain
precisely which name wins when several could apply.

Install a directory pack:

```bash
claudio soundpack validate ./my-pack
claudio soundpack install ./my-pack --default
```

`soundpack validate` (details in [Validation](#validation) below) checks
JSON shape, missing files, and coverage before you install.

Directory packs are copied to:

```text
<XDG_DATA_HOME>/claudio/soundpacks/<name>/
```

`claudio soundpack install` validates, copies, and records the path in
`soundpack_paths` for you — that `soundpack_paths` entry is what makes the
pack actually playable, not just listed.

#### Discovery Vs. Runtime Resolution

`claudio soundpack list` and `claudio soundpack use <name>` scan the
locations under [Where Soundpacks Live](#where-soundpacks-live) directly, so
a directory or JSON pack you copy into place by hand shows up and can be
selected by name. Actually *using* a pack at hook-processing time is a
separate, narrower lookup: Claudio resolves `default_soundpack` by checking,
in order, whether it's a literal path that exists, a name in the managed git
registry, or a name matching an entry in `soundpack_paths`. The canonical
`soundpacks/<name>/` and `<name>.json` locations are **not** consulted at
that step.

In practice this means a hand-copied pack that isn't in `soundpack_paths`
will list and even `use` successfully, then silently fall back to the
platform default the moment a hook actually fires — no error, just the wrong
sound. `claudio soundpack install` avoids this because it adds the
`soundpack_paths` entry for you. If you place files by hand, either add the
path to `soundpack_paths` in `config.json` yourself, or set
`default_soundpack` directly to the pack's full path instead of its bare
name.

### JSON Soundpacks

JSON soundpacks map sound keys to files anywhere on disk. Paths can be absolute
or relative to the JSON file.

```json
{
  "name": "system-sounds",
  "description": "Small pack using existing local sounds",
  "version": "1.0.0",
  "mappings": {
    "success/success.wav": "./sounds/success.wav",
    "error/error.wav": "/home/me/sounds/error.mp3",
    "loading/loading.wav": "./sounds/loading.wav",
    "interactive/message-sent.wav": "./sounds/message-sent.aiff",
    "default.wav": "./sounds/default.wav"
  }
}
```

Create a template:

```bash
claudio soundpack init my-pack
```

Pre-fill the template with the current platform defaults:

```bash
claudio soundpack init my-pack --from-platform
```

Install a JSON pack:

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack install ./my-pack.json --default
```

JSON packs are copied to:

```text
<XDG_DATA_HOME>/claudio/<name>.json
```

`claudio soundpack install` records this path in `soundpack_paths`, which is
what makes the pack playable rather than merely listed — see
[Discovery Vs. Runtime Resolution](#discovery-vs-runtime-resolution) above.

### Validation

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack validate ./my-pack
```

Validation reports:

- Total known-key coverage
- Coverage by category
- Broken JSON references
- Unsupported file extensions
- Empty mappings

Broken references fail validation. Empty mappings do not.

### Use Tracking To Improve A Pack

Enable tracking, use Claudio normally, then inspect missing sounds:

```bash
claudio analyze missing --preset all-time --limit 50
```

The most frequent missing keys are usually the best next sounds to add.

## How Sound Selection Works

This part is for understanding or debugging Claudio's internals — why a
specific sound played, or how to extend the matching logic. Not required
reading for using or building a pack.

### Fallback Chains

Fallback chains are ordered from most specific to least specific. The first
existing sound wins. This is the mechanism behind the sound key names used in
the examples above (`git-commit-start.wav`, `bash-success.wav`, and so on).

#### PreToolUse

For `git commit` started through the Bash tool:

```text
loading/git-commit-start.wav
loading/git-commit.wav
loading/git-start.wav
loading/git.wav
loading/bash-start.wav
loading/bash.wav
loading/tool-start.wav
loading/loading.wav
default.wav
```

#### PostToolUse

For a successful `git commit`:

```text
success/git-commit-success.wav
success/git-success.wav
success/bash-success.wav
success/tool-complete.wav
success/success.wav
default.wav
```

For a failed `git commit`, the category changes:

```text
error/git-commit-error.wav
error/git-error.wav
error/bash-error.wav
error/tool-complete.wav
error/error.wav
default.wav
```

#### Simple Events

For `UserPromptSubmit`:

```text
interactive/message-sent.wav
interactive/prompt-submit.wav
interactive/interactive.wav
default.wav
```

For `Stop`:

```text
completion/agent-complete.wav
completion/stop.wav
completion/completion.wav
default.wav
```

For `PreCompact`:

```text
system/compacting.wav
system/pre-compact.wav
system/system.wav
default.wav
```

### Command Parsing

For Bash tool events, Claudio parses the command string and recognizes common
subcommands for tools such as:

- `git`
- `npm`
- `docker`
- `cargo`
- `go`
- `pip`
- `yarn`
- `kubectl`

Unknown commands are handled conservatively. Claudio still tries command-level
sounds such as `loading/systemctl-start.wav` when the words look like a command
and subcommand rather than file paths or URLs.

MCP tool names beginning with `mcp__` are normalized to `mcp` for sound lookup.

## See Also

- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Examples](examples)
- [Troubleshooting](troubleshooting)
