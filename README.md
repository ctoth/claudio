# Claudio

Claudio is a hook-driven audio layer for coding agents. It listens to hook
events from Claude Code, OpenAI Codex CLI, Gemini CLI, Qwen Code, and GitHub
Copilot CLI, maps the event to a contextual sound, and plays that sound without
making the agent wait for playback.

It can play different sounds for tool starts, tool successes, tool failures,
prompts, notifications, completions, compaction, session starts, and subagent
events. Bash commands are parsed, so `git commit`, `npm test`, and `go build`
can each have their own sounds instead of sharing one generic shell sound.

Full documentation starts at [docs/index.md](docs/index.md).

## Install

```bash
go install claudio.click/cmd/claudio@latest
```

Install hooks for detected agents:

```bash
claudio install
```

`claudio install` uses `--agent auto --scope global` by default. It detects
Claude Code, Codex CLI, Gemini CLI, Qwen Code, and GitHub Copilot CLI, then
installs hooks for the agents it finds. To force a single agent:

```bash
claudio install --agent claude --scope global
claudio install --agent codex --scope global
claudio install --agent gemini --scope global
claudio install --agent qwen --scope global
claudio install --agent copilot --scope global
```

After Codex hook installation, run `/hooks` in Codex and trust the Claudio
hook. Use `--scope project` instead of `--scope global` when you want hooks only
for the current repository.

## Daily Commands

```bash
claudio status
claudio volume 0.4
claudio mute
claudio unmute
claudio uninstall --agent all --scope global
```

Optional agent command artifacts:

```bash
claudio install-commands --agent claude       # /claudio in Claude Code
claudio install-commands --agent codex        # $claudio skill in Codex
claudio install-commands --agent antigravity  # Antigravity skill and CLI command
```

## Soundpacks

Claudio ships platform defaults and supports three custom soundpack forms:

- Directory soundpacks under `loading/`, `success/`, `error/`, `interactive/`,
  `completion/`, and `system/`
- JSON soundpacks that map Claudio sound keys to files anywhere on disk
- Managed git soundpacks installed with `claudio soundpack add`

Useful commands:

```bash
claudio soundpack list
claudio soundpack init my-pack --from-platform
claudio soundpack validate ./my-pack.json
claudio soundpack install ./my-pack.json --default
claudio soundpack add gh:owner/repo --name my-pack --default
claudio soundpack update --all
```

Supported audio formats are WAV, MP3, and AIFF. See
[docs/soundpacks.md](docs/soundpacks.md) for layout, fallback chains, JSON
mappings, validation, and git-backed soundpacks.

## Tracking

Sound tracking is enabled by default. Claudio records resolved sounds and
missing fallback candidates in a SQLite database under the XDG cache directory,
then exposes that data through:

```bash
claudio analyze usage --show-summary --show-chains
claudio analyze missing --preset last-week
```

Use these reports to decide which sounds your custom pack should add next.

## Build And Test

```bash
go build ./cmd/claudio
go test ./...
```
