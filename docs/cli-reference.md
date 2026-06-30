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
| `--print`, `-p` | false | Print target configuration details. |
| `--quiet`, `-q` | false | Reduce output. |

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
| `--print`, `-p` | false | Print removal details. |
| `--quiet`, `-q` | false | Reduce output. |

## `claudio install-commands`

Installs optional command artifacts for asking an agent to control Claudio.

```bash
claudio install-commands --agent claude
claudio install-commands --agent codex
claudio install-commands --agent antigravity
```

| Agent | Artifact |
| --- | --- |
| `claude` | `~/.claude/commands/claudio.md` |
| `codex` | `$HOME/.agents/skills/claudio/SKILL.md` |
| `antigravity` | `~/.gemini/config/skills/claudio/SKILL.md` and `~/.gemini/antigravity-cli/skills/claudio.md` |

## `claudio uninstall-commands`

Removes artifacts created by `install-commands`.

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

When audio is disabled, the `enabled` line includes the literal word `MUTED`.

## `claudio volume`

Gets or sets the persisted volume in `config.json`.

```bash
claudio volume
claudio volume 0.25
```

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

Lists embedded, XDG, and config-discovered soundpacks.

```bash
claudio soundpack list
```

### `soundpack validate`

Validates a JSON file or directory soundpack and prints coverage.

```bash
claudio soundpack validate <path>
```

Validation checks JSON shape, missing referenced files, known-key coverage, and
supported extensions. WAV, MP3, and AIFF are supported. Broken references cause
a non-zero exit. Empty mappings are informational.

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

JSON files install to `<XDG_DATA_HOME>/claudio/<name>.json`.
Directories install to `<XDG_DATA_HOME>/claudio/soundpacks/<name>/`.

### `soundpack use`

Switches the active soundpack by name.

```bash
claudio soundpack use <name>
```

The name must appear in `claudio soundpack list`.

### `soundpack add`

Clones a git-backed soundpack into Claudio-managed storage.

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

Reads the tracking database.

```bash
claudio analyze usage [flags]
claudio analyze missing [flags]
```

Shared flags:

| Flag | Default | Meaning |
| --- | --- | --- |
| `--days int` | `7` | Number of days to analyze. `0` means all time. |
| `--preset string` | empty | `today`, `yesterday`, `last-week`, `this-month`, or `all-time`. |
| `--tool string` | empty | Filter by tool name. |
| `--category string` | empty | Filter by stored category. Common values are `success`, `error`, `loading`, and `interactive`. |
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

## Exit Codes

Most command failures return exit code `1`. Validation and configuration
errors are printed to stderr with the failing command context.

## See Also

- [Installation](installation)
- [Configuration](configuration)
- [Soundpacks](soundpacks)
- [Troubleshooting](troubleshooting)
