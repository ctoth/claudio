---
layout: default
title: "CLI Reference"
---

# CLI Reference

This page documents the current command surface exposed by `claudio`.

```bash
claudio [flags]
claudio [command]
```

Global flags:

| Flag | Meaning |
| --- | --- |
| `--config string` | Load a specific config file for this invocation. |
| `--volume string` | Override volume for this invocation. Must be `0.0` through `1.0`. |
| `--soundpack string` | Override the active soundpack for this invocation. |
| `--silent` | Process the hook without audio playback. |
| `--version`, `-v` | Print version. |
| `--help`, `-h` | Print help. |

`--volume`, `--soundpack`, and `--silent` affect hook processing only. They are
never written to `config.json`.

## `claudio`

With no subcommand, Claudio reads a hook JSON payload from stdin and processes
it.

```bash
echo '{"session_id":"test","cwd":".","hook_event_name":"UserPromptSubmit","prompt":"hello"}' | claudio
```

Required payload fields are:

- `session_id`
- `cwd`
- `hook_event_name`

Tool events can also include `tool_name`, `tool_input`, and `tool_response`.
A payload that is not valid JSON or lacks a required field exits with code `1`.

## `claudio install`

Installs hooks for Claude Code, Codex CLI, Gemini CLI, Qwen Code, or GitHub
Copilot CLI.

```bash
claudio install [flags]
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--agent`, `-a` | `auto` | `auto`, `all`, `claude`, `codex`, `gemini`, `qwen`, or `copilot`. |
| `--scope`, `-s` | `global` | `global` or `project`. |
| `--dry-run`, `-d` | false | Show what would happen without writing. |
| `--print`, `-p` | false | Print the mode, scope, target agent, and settings path. |
| `--quiet`, `-q` | false | Suppress progress messages. |

`--agent auto` installs for every agent that shows evidence of being present:
its executable is on `PATH`, its settings file or settings directory exists, or
Claudio hooks are already installed there. It fails if no agent is found.
`--agent all` targets all five agents whether or not they are installed.

Settings file locations for each agent and scope are listed on the
[home page](./#what-claudio-installs). Claude Code honors `CLAUDE_CONFIG_DIR`,
Codex honors `CODEX_HOME`, and GitHub Copilot CLI honors `COPILOT_HOME`.

Examples:

```bash
claudio install
claudio install --agent all --scope global
claudio install --agent claude --scope global
claudio install --agent codex --scope project
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
claudio install --agent codex --scope global --dry-run
```

Codex users must trust the hook with `/hooks` after installation.

## `claudio uninstall`

Removes Claudio hooks for Claude Code, Codex CLI, Gemini CLI, Qwen Code, or
GitHub Copilot CLI.

```bash
claudio uninstall [flags]
```

Flags match `install`:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--agent`, `-a` | `auto` | `auto`, `all`, `claude`, `codex`, `gemini`, `qwen`, or `copilot`. |
| `--scope`, `-s` | `global` | `global` or `project`. |
| `--dry-run`, `-d` | false | Show what would be removed. |
| `--print`, `-p` | false | Print the mode, scope, target agent, and settings path. |
| `--quiet`, `-q` | false | Suppress progress messages. |

Only Claudio's own hook entries are removed. Other hooks in the same settings
file are preserved.

## `claudio install-commands`

Installs optional command artifacts for asking an agent to control Claudio.

```bash
claudio install-commands --agent claude
claudio install-commands --agent codex
claudio install-commands --agent antigravity
```

| Flag | Default | Meaning |
| --- | --- | --- |
| `--agent`, `-a` | `claude` | `claude`, `codex`, or `antigravity`. |

| Agent | Artifact |
| --- | --- |
| `claude` | `~/.claude/commands/claudio.md` |
| `codex` | `$HOME/.agents/skills/claudio/SKILL.md` |
| `antigravity` | `~/.gemini/config/skills/claudio/SKILL.md` and `~/.gemini/antigravity-cli/skills/claudio.md` |

When `CLAUDE_CONFIG_DIR` is set, the Claude Code command goes to
`$CLAUDE_CONFIG_DIR/commands/claudio.md` instead.

If an artifact already exists and its content is not something Claudio wrote,
the command refuses to touch it and exits with an error. Move or delete your
customized file first.

## `claudio uninstall-commands`

Removes artifacts created by `install-commands`. Takes the same `--agent` flag,
also defaulting to `claude`.

```bash
claudio uninstall-commands --agent claude
claudio uninstall-commands --agent codex
claudio uninstall-commands --agent antigravity
```

## `claudio status`

Prints the effective configuration after file and environment overrides.

```bash
claudio status
```

Example output:

```text
claudio status

  config file:    /home/me/.config/claudio/config.json
  enabled:        true
  volume:         0.50 (from config.json)
  soundpack:      embedded:linux.json
  log level:      warn
  audio backend:  auto -> oto (available; playback not tested)
  file logging:   enabled (/home/me/.cache/claudio/logs/claudio.log)
  tracking:       enabled (/home/me/.cache/claudio/sounds.db)
  version:        1.14.0
```

The `audio backend` line shows what `auto` resolves to and whether that backend
is available. It does not play a sound.

When audio is disabled, the `enabled` line includes the literal word `MUTED`.

`--config` selects the file to report on. The transient `--volume`,
`--soundpack`, and `--silent` flags are not applied.

## `claudio volume`

Gets or sets the persisted volume in `config.json`.

```bash
claudio volume
claudio volume 0.25
```

With no argument it prints the volume hooks will use, including a
`CLAUDIO_VOLUME` override if one is set. With an argument it writes the value
to the config file.

`claudio volume`, `mute`, `unmute`, and `soundpack use` write to the file named
by `--config`, or else to the user config path (see
[Configuration](configuration#config-file)). If that file does not exist yet,
they create it from the effective configuration, so the new file is complete.

Environment variable `CLAUDIO_VOLUME` and global flag `--volume` still override
the persisted value at runtime.

## `claudio mute` And `claudio unmute`

Persistently toggles `enabled` in `config.json`.

```bash
claudio mute
claudio unmute
```

Environment variable `CLAUDIO_ENABLED` still overrides the persisted value at
runtime.

## `claudio soundpack`

Manages soundpacks.

```bash
claudio soundpack [command]
```

### `soundpack init`

Creates a JSON soundpack template with all known mapping keys.

```bash
claudio soundpack init <name> [flags]
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--dir string` | `.` | Output directory. |
| `--from-platform` | false | Pre-fill mappings from the current embedded platform soundpack. |

Examples:

```bash
claudio soundpack init my-pack
claudio soundpack init my-pack --dir ./soundpacks
claudio soundpack init my-pack --from-platform
```

### `soundpack list`

Lists embedded, XDG, and config-discovered soundpacks with their type, sound
count, and path.

```bash
claudio soundpack list
```

The embedded packs are `windows`, `wsl`, `darwin`, and `linux`.

### `soundpack validate`

Validates a JSON file or directory soundpack and prints coverage.

```bash
claudio soundpack validate <path>
```

Validation checks JSON shape, missing referenced files, known-key coverage, and
supported extensions. WAV, MP3, and AIFF are supported. Broken references and
unsafe paths (absolute, or containing `..`) cause a non-zero exit. Empty
mappings are informational.

### `soundpack install`

Copies a local JSON file or directory into the XDG data directory and updates
`soundpack_paths`.

```bash
claudio soundpack install <path> [flags]
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--default` | false | Set the installed soundpack as `default_soundpack`. |
| `--skip-validate` | false | Skip validation before copying. |

JSON files and their referenced audio files install under
`<XDG_DATA_HOME>/claudio/soundpacks/<name>/`, with the installed manifest at
`soundpack.json`.
Directories install to `<XDG_DATA_HOME>/claudio/soundpacks/<name>/`.

### `soundpack use`

Switches the active soundpack by name.

```bash
claudio soundpack use <name>
claudio soundpack use windows
```

This sets `default_soundpack`. The name must appear in
`claudio soundpack list`.

### `soundpack add`

Clones a git-backed soundpack into
`<XDG data home>/claudio/soundpack-repos/<name>/` and adds it to
`soundpack_paths`. Requires `git` on `PATH`.

```bash
claudio soundpack add <git-url> [flags]
```

Flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--name string` | derived from URL | Install name. |
| `--ref string` | default branch | Branch, tag, or commit to check out. |
| `--subdir string` | repository root | Directory or JSON file within the repository. |
| `--default` | false | Set as active soundpack. |
| `--skip-validate` | false | Skip validation. |
| `--replace` | false | Replace an existing managed git soundpack with the same name. |

GitHub shorthand is accepted:

```bash
claudio soundpack add gh:owner/repo --name my-pack --default
```

### `soundpack update`

Updates managed git soundpacks.

```bash
claudio soundpack update <name>
claudio soundpack update --all
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--all` | Update every managed git soundpack. |
| `--force` | Discard local clone changes before updating. |

### `soundpack remove`

Removes a managed git soundpack.

```bash
claudio soundpack remove <name>
```

Flags:

| Flag | Meaning |
| --- | --- |
| `--keep-files` | Remove registry/config entries but leave the clone on disk. |
| `--force` | Remove registry/config entries even if clone deletion fails. |

### `soundpack status`

Shows managed git soundpack status.

```bash
claudio soundpack status
claudio soundpack status <name>
```

## `claudio analyze`

Reads the tracking database. Both subcommands fail if tracking is disabled.

```bash
claudio analyze usage [flags]
claudio analyze missing [flags]
```

`missing` lists fallback-chain candidates that were requested but not found,
most requested first. `usage` lists the sounds that actually played.

Shared flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--days int` | `7` | Number of days to analyze. `0` means all time. |
| `--preset string` | empty | `today`, `yesterday`, `this-week`, `last-week`, `this-month`, `last-month`, or `all-time`. Overrides `--days`. |
| `--tool string` | empty | Filter by tool name. |
| `--category string` | empty | `loading`, `success`, `error`, `interactive`, `completion`, or `system`. Other values are rejected. |
| `--limit int` | `20` | Maximum rows. |

`usage` also supports:

| Flag | Meaning |
| --- | --- |
| `--show-summary` | Print summary statistics. |
| `--show-chains` | Print chain-type stats and average fallback depth. |

Examples:

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze usage --tool Bash --preset today
claudio analyze missing --preset all-time --limit 50
claudio analyze missing --category error
```

## `claudio completion`

Generates a shell completion script for Bash, Fish, PowerShell, or Zsh.

```bash
claudio completion bash
claudio completion fish
claudio completion powershell
claudio completion zsh
```

## Exit Codes

Subcommand failures return exit code `1`.

In hook mode, Claudio exits `1` before playback for an unparseable payload, an
out-of-range `CLAUDIO_VOLUME` or `--volume`, and an audio backend that cannot
be resolved. A config file is never fatal: a missing or empty one means
defaults, and one that cannot be parsed or fails validation prints a single
`Warning:` line and Claudio continues with defaults. Once the payload is accepted, missing sounds and
playback errors are only logged.

## See Also

- [Installation](installation)
- [Configuration](configuration)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
