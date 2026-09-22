---
layout: default
title: "Soundpacks"
---

# Soundpacks

A soundpack maps Claudio sound keys (such as `success/git-success.wav`) to
audio files. A pack can be a directory, a JSON manifest, or a git repository
that Claudio manages for you.

The first half of this page covers using and installing packs. The second
covers building one. The last section explains how Claudio picks a sound,
which you only need when debugging or naming files precisely.

## Audio Formats

Claudio's native player decodes:

- **WAV**: mono or stereo; 16-, 24-, or 32-bit PCM, or 32-bit float. Files
  with more than two channels are rejected.
- **MP3**
- **AIFF**: 16-, 24-, or 32-bit. Files with more than two channels are
  downmixed to stereo.

Playback reads at most 100 MiB from any audio file. MP3 decoding also stops
once the decoded PCM passes 100 MiB, so an MP3 well under that size on disk
can still hit the limit after decoding.

## Categories

Every event maps to one category, and each category is a directory in the
pack:

| Category | Typical events | Directory |
| --- | --- | --- |
| Loading | `PreToolUse`, `SubagentStart` | `loading/` |
| Success | successful `PostToolUse` | `success/` |
| Error | failed `PostToolUse`, `PostToolUseFailure` | `error/` |
| Interactive | prompts, notifications, permission requests | `interactive/` |
| Completion | `Stop`, `SubagentStop` | `completion/` |
| System | session start, compaction | `system/` |

`default.wav` at the pack root is the last fallback for every event.

## Where Soundpacks Live

Claudio uses XDG base directories. On Linux and WSL the data directory is
`~/.local/share`; on macOS it is `~/Library/Application Support`; on Windows
it is `%LOCALAPPDATA%`. On macOS and Windows the config directory is the same
folder as the data directory.

| Item | Linux / WSL | macOS | Windows |
| --- | --- | --- | --- |
| Installed packs | `~/.local/share/claudio/soundpacks/<name>/` | `~/Library/Application Support/claudio/soundpacks/<name>/` | `%LOCALAPPDATA%\claudio\soundpacks\<name>\` |
| Managed git clones | `~/.local/share/claudio/soundpack-repos/<name>/` | `~/Library/Application Support/claudio/soundpack-repos/<name>/` | `%LOCALAPPDATA%\claudio\soundpack-repos\<name>\` |
| Managed git registry | `~/.config/claudio/soundpacks.json` | `~/Library/Application Support/claudio/soundpacks.json` | `%LOCALAPPDATA%\claudio\soundpacks.json` |
| `config.json` | `~/.config/claudio/config.json` | `~/Library/Application Support/claudio/config.json` | `%LOCALAPPDATA%\claudio\config.json` |

An installed JSON pack is a directory like any other, with its manifest at
`soundpacks/<name>/soundpack.json`.

`XDG_DATA_HOME` and `XDG_CONFIG_HOME` override these locations on every
platform, including Windows.

### Built-In Packs

Four platform packs are compiled into the binary and need no install step.
`claudio soundpack list` shows them with the type `embedded`.

| Pack | What it plays |
| --- | --- |
| `windows` | Sounds from `C:\Windows\Media` |
| `wsl` | The same Windows sounds, reached through `/mnt/c/Windows/Media` |
| `darwin` | macOS system sounds from `/System/Library/Sounds` |
| `linux` | Seven synthesized tones shipped inside the binary, one per category plus `default.wav` |

The `windows`, `wsl`, and `darwin` packs only map to files that already ship
with the operating system. WSL gets its own pack, even though it runs the
Linux binary, because the Windows sound files are available there.

## Using A Pack

### Switching The Active Soundpack

```bash
claudio soundpack list
claudio soundpack use <name>
```

`soundpack use` sets `default_soundpack` in `config.json`. The name has to
be one that `soundpack list` shows: an embedded pack, a pack in the XDG
`soundpacks/` directory, a pack listed in `soundpack_paths`, or a managed git
pack.

To override the pack for one run without changing the config:

```bash
claudio --soundpack <name>
CLAUDIO_SOUNDPACK=<name> claudio
```

If the configured pack cannot be found, Claudio logs an error and falls back
to the platform pack.

### Installing A Pack From Git

There is no central soundpack index. To share a pack, put it in a git
repository and hand out the URL. Any public or private repository that
contains a directory pack or a JSON pack works.

```bash
claudio soundpack add https://github.com/owner/repo --name my-pack --default
claudio soundpack add gh:owner/repo --subdir packs/minimal --name minimal
```

`add` clones the repository into `soundpack-repos/<name>/`, validates it,
records it in the local `soundpacks.json` registry (name, source URL, ref,
commit), and adds the playable path to `soundpack_paths`. `gh:owner/repo` is
shorthand for `https://github.com/owner/repo.git`.

| Flag | Effect |
| --- | --- |
| `--name` | Name for the installed pack |
| `--subdir` | Directory or JSON file inside the repository to use as the pack |
| `--ref` | Branch, tag, or commit to check out |
| `--default` | Make it the active pack |
| `--replace` | Replace an existing managed pack with the same name |
| `--skip-validate` | Skip validation before adding |

Update one pack or all of them:

```bash
claudio soundpack update my-pack
claudio soundpack update --all
claudio soundpack update my-pack --force   # discard local changes in the clone first
```

Claudio never fetches in the background or while handling hooks. A managed
pack stays at the commit it was cloned or last updated to until you run
`soundpack update`. Schedule that yourself if you want it automatic.

Check what is installed:

```bash
claudio soundpack status
claudio soundpack status my-pack
```

### Removing A Pack

Managed git packs:

```bash
claudio soundpack remove my-pack                # delete the clone and registry entry
claudio soundpack remove my-pack --keep-files   # drop the registry entry, keep the clone
claudio soundpack remove my-pack --force        # drop registry and config entries even if deleting the clone fails
```

`remove` also takes the pack out of `soundpack_paths`, and if it was the
active pack, resets `default_soundpack` to the platform default.

Packs installed with `soundpack install` have no remove command. Delete the
directory and remove its entry from `soundpack_paths` in `config.json`:

```bash
rm -rf "$XDG_DATA_HOME/claudio/soundpacks/my-pack"   # adjust for your platform
```

If it was the active pack, pick another with `claudio soundpack use <name>`.
Until you do, Claudio logs an error on each event and uses the platform pack.

## Building A Pack

### Directory Soundpacks

In a directory pack, each sound key is a file path relative to the pack
root:

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

Sound keys always end in `.wav`, but the file does not have to. When the
exact `.wav` file is missing, Claudio looks for the same name with `.mp3`,
`.aiff`, `.aif`, or `.mpeg`, so `success/success.mp3` answers for
`success/success.wav`.

One file per category plus `default.wav` is enough to cover every event.
Add more specific names as you go; [Fallback Chains](#fallback-chains)
lists which names Claudio tries for each event.

Validate and install:

```bash
claudio soundpack validate ./my-pack
claudio soundpack install ./my-pack --default
```

`install` validates the pack, copies it to `<XDG_DATA_HOME>/claudio/soundpacks/<name>/`,
and adds that path to `soundpack_paths`. `--skip-validate` skips the coverage
check; the copied pack is still checked for safety.

Claudio finds packs in the XDG `soundpacks/` directory by name. If a
directory there contains `soundpack.json`, Claudio loads it as a JSON pack;
otherwise it reads the category layout above. A pack anywhere else needs an
entry in `soundpack_paths` or a full path. Scans skip `.git` directories.

### JSON Soundpacks

A JSON pack maps sound keys to files stored next to the manifest:

```json
{
  "name": "system-sounds",
  "description": "Small pack using existing local sounds",
  "version": "1.0.0",
  "mappings": {
    "success/success.wav": "./sounds/success.wav",
    "error/error.wav": "./sounds/error.mp3",
    "loading/loading.wav": "./sounds/loading.wav",
    "interactive/message-sent.wav": "./sounds/message-sent.aiff",
    "default.wav": "./sounds/default.wav"
  }
}
```

Rules:

- `name` and at least one entry in `mappings` are required. `description`
  and `version` are optional.
- Each value must be a relative path inside the manifest's directory.
  Absolute paths and `..` are rejected, and a symlink that points outside
  the directory is rejected too.
- An empty value means the key is not mapped yet; it is skipped.
- `validate` and `install` reject a value whose file does not exist. If a
  file goes missing after install, Claudio logs a warning, skips that entry,
  and keeps using the rest of the pack.
- A pack can have at most 10,000 mappings.

Create a template:

```bash
claudio soundpack init my-pack                  # writes ./my-pack.json
claudio soundpack init my-pack --dir ./packs    # writes ./packs/my-pack.json
claudio soundpack init my-pack --from-platform  # pre-fills the current platform pack's mappings
```

A plain `init` template lists every known key with an empty value. Fill in
the keys you want; the empty ones are skipped, so you can leave them in or
delete them. `validate` lists them as unmapped.

Validate and install:

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack install ./my-pack.json --default
```

`install` copies the manifest and every file it references into
`<XDG_DATA_HOME>/claudio/soundpacks/<name>/`, keeping relative
subdirectories, writes the manifest there as `soundpack.json`, and adds it
to `soundpack_paths`. You can then refer to the pack by its directory name or
its `name` field.

### Validation

```bash
claudio soundpack validate ./my-pack.json
claudio soundpack validate ./my-pack
```

The report shows:

- Coverage of known sound keys, overall and per category
- Broken references (mapped files that do not exist)
- Unsupported file extensions
- Empty mappings

Broken references fail validation. Empty mappings are reported as unmapped
but do not fail it.

### Using Tracking To Improve A Pack

Sound tracking is on by default. Use Claudio for a while, then list the keys
it looked for and did not find:

```bash
claudio analyze missing --preset all-time --limit 50
```

The most frequent missing keys are usually the best sounds to add next.

## How Sound Selection Works

### Fallback Chains

For each event Claudio builds a list of candidate keys, from most specific to
least specific, and plays the first one the pack has. Duplicate candidates
are dropped.

#### PreToolUse

`git commit` run through the Bash tool:

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

A successful `git commit`:

```text
success/git-commit-success.wav
success/git-success.wav
success/bash-success.wav
success/tool-complete.wav
success/success.wav
default.wav
```

A failed one uses the `error/` category:

```text
error/git-commit-error.wav
error/git-error.wav
error/bash-error.wav
error/tool-complete.wav
error/error.wav
default.wav
```

The post-tool chain skips the bare command key (`success/git.wav`), so a
generic command sound cannot stand in for a specific result.

#### Other Events

Events without a tool use a short chain: a specific key, an event key, the
category sound, and `default.wav`.

| Event | Keys tried before the category sound |
| --- | --- |
| `UserPromptSubmit` | `interactive/message-sent.wav`, `interactive/prompt-submit.wav` |
| `Notification` | `interactive/notification.wav` |
| `PermissionRequest` | `interactive/permission-request.wav` |
| `Stop` | `completion/agent-complete.wav`, `completion/stop.wav` |
| `SubagentStop` | `completion/subagent-complete.wav`, `completion/subagent-stop.wav` |
| `SubagentStart` | `loading/subagent-start.wav` |
| `SessionStart` | `system/session-start.wav` |
| `PreCompact` | `system/compacting.wav`, `system/pre-compact.wav` |
| `PostCompact` | `system/post-compact.wav` |

For example, `Stop` tries `completion/agent-complete.wav`,
`completion/stop.wav`, `completion/completion.wav`, then `default.wav`.

### Command Parsing

For Bash tool events, Claudio parses the command string and recognizes the
subcommands of:

- `git`
- `npm`
- `docker`
- `cargo`
- `go`
- `pip`
- `yarn`
- `kubectl`

For other commands, Claudio still tries command-level keys such as
`loading/systemctl-start.wav`, and treats the second word as a subcommand
(`loading/systemctl-restart-start.wav`) when it looks like one rather than a
file path, flag, or URL.

### MCP Tools

MCP tools (names beginning with `mcp__`) first try the shared `mcp` keys,
then the full normalized tool name. For `mcp__github__create_issue` at
`PreToolUse`:

```text
loading/mcp-start.wav
loading/mcp.wav
loading/mcp-github-create-issue-start.wav
loading/mcp-github-create-issue.wav
loading/tool-start.wav
loading/loading.wav
default.wav
```

## See Also

- [CLI Reference](cli-reference)
- [Configuration](configuration)
- [Examples](examples)
- [Troubleshooting](troubleshooting)
