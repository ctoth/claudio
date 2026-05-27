# Codex hook support — design

Date: 2026-05-27
Status: approved (pending spec review)
Branch base: `feat/mcp-tool-normalization`

## Goal

Let Claudio play contextual sounds for OpenAI Codex CLI, the same way it already
does for Claude Code. A user running Codex should be able to install Claudio
hooks and hear loading, success, error, and lifecycle sounds.

## Background

Codex CLI shipped stable hooks in May 2026. The contract is close enough to
Claude Code that most of Claudio works unchanged:

- Every command hook receives one JSON object on stdin (same as Claude Code).
- Shared field names: `session_id`, `cwd`, `hook_event_name`, `tool_name`,
  `tool_input`, `tool_response`, `prompt`.
- Hook config lives in `~/.codex/hooks.json` (or inline in `config.toml`), with
  top-level shape `{"hooks": {EventName: [ {matcher, hooks:[{type,command}]} ]}}`
  — structurally the same as Claude's `settings.json` hooks block.
- A hook that exits 0 with no stdout is treated as success and execution
  continues. Claudio's detached worker already exits 0 silently, so it never
  blocks Codex.

Sources:
- https://developers.openai.com/codex/hooks
- https://developers.openai.com/codex/config-advanced

### Codex hook events

`SessionStart`, `SubagentStart`, `PreToolUse`, `PermissionRequest`,
`PostToolUse`, `PreCompact`, `PostCompact`, `UserPromptSubmit`, `SubagentStop`,
`Stop`.

Codex has no `Notification` or `SessionEnd` event. Claude has no `SubagentStart`
or `PostCompact`. Everything else overlaps.

### Differences that drive the work

1. `transcript_path` is nullable/omitted in Codex. Claudio's parser currently
   rejects an event when `transcript_path` is empty (`parser.go:124`).
2. Install target is `~/.codex/hooks.json`, not `~/.claude/settings.json`.
3. The Codex event set differs (adds `SubagentStart`, `PostCompact`; drops
   `Notification`, `SessionEnd`).
4. Codex tool names include `apply_patch` (Claudio has no special case; flows
   through the generic path) and `mcp__<server>__<tool>` (already normalized to
   `mcp` by the prior commit on this branch).
5. Non-managed Codex hooks require explicit trust via the `/hooks` command.
   Claudio cannot bypass this; it surfaces a reminder after install.

## Decisions

- **Scope:** full parity. Map the Codex-only events and tools, not just the
  shared ones.
- **Install format:** `~/.codex/hooks.json` (pure JSON, reuses existing merge
  logic, no TOML dependency, never touches the user's `config.toml`).
- **Agent selection:** `--agent`/`-a` flag on `install` and `uninstall`, values
  `claude` (default) and `codex`.
- **Sound assets:** mapping logic only. New Codex-only hints rely on the
  5-level fallback until custom audio is added. No new `.wav` files.
- **Test fixtures:** inferred from the documented schema. Verify against real
  Codex payloads later.

## Architecture

The core pipeline — parse → `GetContext` → `SoundMapper` → audio backend — is
reused unchanged. Two areas change: the hook parser gains tolerance and new
event cases; the install/uninstall layer becomes agent-aware.

### Component 1 — Parser tolerance (`internal/hooks/parser.go`)

- Stop treating an empty `transcript_path` as a fatal validation error. Keep
  `session_id`, `cwd`, and `hook_event_name` required (Codex always sends all
  three).
- Add `GetContext` cases:
  - `SubagentStart` → category `Loading`, hint `subagent-start`, operation
    `subagent-start`.
  - `PostCompact` → category `System`, hint `post-compact`, operation
    `post-compact`.
- No change needed for `PermissionRequest`, `SessionStart`, `PreCompact`,
  `Stop`, `SubagentStop` — already mapped.
- `apply_patch` flows through the existing generic
  `<tool>-start` / `<tool>-success` / `<tool>-error` path. The 5-level fallback
  covers any missing audio.
- Success/error detection reuses `analyzeToolResponse`: `isError`, `stderr`,
  and `interrupted` checks, defaulting to success when none are present. This
  matches the documented Codex `tool_response` ("MCP result or output") as
  closely as the docs allow; revisit when real payloads are available.

This component is self-contained: input is a JSON byte slice, output is a
`HookEvent` and an `EventContext`. It has no knowledge of which agent produced
the event.

### Component 2 — Agent abstraction (`internal/install`)

Introduce an `Agent` value (`claude` | `codex`) that selects three things:

1. **Config path finder.**
   - Claude: `~/.claude/settings.json` (existing `FindBestSettingsPath`,
     unchanged).
   - Codex: `~/.codex/hooks.json`, with the same Windows `USERPROFILE` / MSYS
     handling used for Claude.
2. **Hook registry.**
   - Claude registry: unchanged.
   - Codex registry: `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Stop`,
     `SubagentStop`, `SubagentStart`, `PreCompact`, `PostCompact`,
     `SessionStart`, `PermissionRequest`.
3. **Default matcher.** Codex `"*"`, Claude `".*"`.

Reused without change because they operate on the generic `hooks` JSON key:
`ReadSettingsFile`, `WriteSettingsFile`, `MergeHooksIntoSettings`,
`IsClaudioHook`, and the atomic temp-file-plus-rename write.

The boundary stays clean: callers ask the agent for "where do I write" and
"which events do I register," then hand off to the existing generic merge/write
functions.

### Component 3 — CLI wiring (`internal/cli`)

- Add `--agent`/`-a` to `install` and `uninstall`. Default `claude`. Validate
  against the known set; reject anything else with a clear error.
- The install workflow takes the agent, resolves the path finder and registry,
  and runs the existing read → generate → merge → write → verify steps.
- After a successful Codex install, print:
  `Run /hooks in Codex to trust the claudio hook.`
- `--dry-run` and `--print` show the selected agent and the resolved config
  path; the Codex dry-run also shows the trust reminder.

## Data flow (Codex)

1. Codex fires an event and spawns `claudio` with the event JSON on stdin.
2. Claudio reads the payload, spawns a detached worker, and exits 0 with no
   stdout. Codex sees success and continues — never blocked.
3. The worker parses the payload, derives category and sound hint, and plays the
   resolved sound through the 5-level fallback.

## Error handling

- Parser: a null or missing `transcript_path` is no longer fatal. Unknown events
  fall through to the existing default (`Interactive`, `default` sound).
- Installer: a missing `hooks.json` is created (`ReadSettingsFile` returns empty
  settings). The merge preserves any non-Claudio Codex hooks already present.
  Writes stay atomic.
- Trust: Claudio cannot auto-trust a non-managed hook. The reminder is the
  mitigation.

## Testing

Strict TDD: red first, then minimal implementation, then refactor. Fixtures are
built from the documented Codex schema.

### Coverage targets

Current baseline (this machine):
- `internal/hooks` 80.2%
- `internal/install` 81.0%
- `internal/cli` — tests do not build locally (cgo linker `cannot find 'ld'`
  via mingw; the package pulls in the audio/malgo cgo dependency).

Targets:
- Ratchet `internal/hooks` and `internal/install` to **≥ 90%**.
- New code (Codex parser cases, agent abstraction, `--agent` flag handling)
  to **≥ 95%** with branch coverage on each new decision point.
- Coverage ratchets up, never down (RULES always-on rule).

### Test cases

Parser (`internal/hooks/parser_test.go`):
- Event with null `transcript_path` parses successfully.
- Event with omitted `transcript_path` parses successfully.
- Still rejects missing `session_id`, `cwd`, or `hook_event_name`.
- `SubagentStart` → `Loading` / `subagent-start`.
- `PostCompact` → `System` / `post-compact`.
- `apply_patch` `PreToolUse` → `apply_patch-start`; `PostToolUse` success →
  `apply_patch-success`; with `isError` → error category.
- `mcp__server__tool` event → normalized to `mcp` (regression guard).

Install (`internal/install`):
- Codex path finder resolves `~/.codex/hooks.json` (and Windows
  `USERPROFILE`).
- Codex registry contains exactly the ten Codex events and no
  `Notification` / `SessionEnd`.
- Generate + merge Codex hooks into an empty `hooks.json` (afero in-memory).
- Merge preserves a pre-existing non-Claudio Codex hook.
- Idempotent re-install does not duplicate Claudio entries.
- Default matcher is `"*"` for Codex.

Uninstall (`internal/uninstall`):
- `--agent codex` removes only Claudio entries, preserves others.

CLI (`internal/cli`):
- `install --agent codex --dry-run` shows the Codex path and trust reminder.
- `install --agent codex --print` shows agent + path.
- Invalid `--agent` value is rejected.
- Default `--agent` is `claude` (no behavior change for existing users).

Note: if the local cgo linker stays broken, CLI-package tests run in CI or on a
machine with a working `ld`. The hooks and install packages have no cgo
dependency and test locally.

## Out of scope (YAGNI)

- `config.toml` / inline TOML hook installation.
- New audio assets for Codex-only hints.
- Codex structured outputs (deny / rewrite / `updatedInput`). Claudio is an
  observer; it never alters Codex behavior.
- Auto-detecting installed agents.
