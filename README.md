# Claudio

Claudio is a hook-based audio plugin for Claude Code. It plays contextual
sounds for tool starts, successes, failures, prompts, completions, and other
events.

Full documentation lives in [docs/index.md](docs/index.md).

## Soundpacks

Claudio supports directory soundpacks, JSON soundpacks, and managed git
soundpacks.

```bash
# Create a JSON soundpack template
claudio soundpack init my-pack

# Install a local directory or JSON soundpack
claudio soundpack install ./my-pack --default

# Add a git-backed soundpack
claudio soundpack add https://github.com/ctoth/whatever --name whatever

# GitHub shorthand
claudio soundpack add gh:ctoth/whatever

# Update, inspect, and remove managed git soundpacks
claudio soundpack update whatever
claudio soundpack status whatever
claudio soundpack remove whatever
```

Git soundpacks are cloned into Claudio's data directory, recorded in
`soundpacks.json`, and added to `soundpack_paths` so runtime loading still uses
the same directory/JSON resolver.

See [docs/soundpacks.md](docs/soundpacks.md) for the soundpack layout,
fallback rules, JSON mappings, and git-backed workflow.
