# Architectural Debt Log

Architectural items the post-review-fixes campaign found but deliberately
left alone. None of them crash or corrupt anything; they are shape problems
that deserve their own scoped change rather than a drive-by fix inside
unrelated work.

Each entry gives the location, the severity (cost of leaving it), the blast
radius (what a fix touches), and the report the finding came from.

---

## #51 — CLI struct DI container is not applied everywhere

**Location:** `internal/cli/cli.go` (the `CLI` struct and its
`configManager` field) versus the remaining direct
`config.NewConfigManager()` calls.

**Severity:** Low. The verbs now go through `loadConfigForVerb` and
`mutateConfigForCommand` (`internal/cli/verb_helpers.go`), which use the
injected `cli.configManager`. Four sites still build their own manager:

- `initializeAudioSystem` platform-pack fallback (`cli.go`)
- `setupLogging` log-path resolution (`cli.go`)
- `removeConfigSoundpackPath` default-soundpack reset (`soundpack_git.go`)
- `discoverConfigSoundpacks` (`soundpack_helpers.go`)

Tests cannot inject a fake manager into those paths, so they depend on
`testenv.IsolateXDG(t)` to keep the developer's real config out. That is
enforced by convention, not by the type system.

**Blast radius if fixed:** Small. Pass the `*CLI` (or its config manager)
into the four functions above.

**Original finding:** Chunk 19 scout (`reports/chunk-19a-scout-cli-hygiene-report.md`).

---

## #52 — `initializeAudioSystem` is mostly soundpack resolution

**Location:** `internal/cli/cli.go` `initializeAudioSystem` (search by
name; line numbers drift). It is about 140 lines.

**Severity:** Medium. The function is named for audio backend setup, but
most of its body resolves the active soundpack: XDG data dirs,
`soundpack_paths` entries, embedded platform JSONs, and managed git packs.
That is soundpack-package logic sitting in CLI plumbing.

**Blast radius if fixed:** Extract a function (or type) in
`internal/soundpack` that takes a name and returns a resolved mapper, then
have the CLI compose backend setup separately. The move is mechanical, but
the function is long and well tested; the risk is quietly changing
resolution precedence along the way.

**Original finding:** Chunk 19 scout (#52, deferred as architectural).

---

## #54 — Soundpack discovery lives in `internal/cli`

**Location:** `internal/cli/soundpack_helpers.go` — `discoverSoundpacks`,
`discoverEmbeddedSoundpacks`, `discoverXDGSoundpacks`,
`discoverConfigSoundpacks`, `countAudioFiles`, `countNonEmptyMappings`, and
the `soundpackInfo` type — plus `discoverManagedGitSoundpacks` in
`internal/cli/soundpack_git.go`.

**Severity:** Medium. Knowing which packs exist, where they are, and how
many sounds each has is `internal/soundpack` territory. It lives in the CLI
because the CLI was the first consumer. `soundpack list`, `soundpack use`,
and the managed-git code all reach into it now; moving it would let a
non-CLI consumer reuse it.

**Blast radius if fixed:** All callers are in `internal/cli`, so the move is
mostly `git mv` plus import updates. `soundpackInfo` moves with it.

**Original finding:** Chunk 19 scout (#54, deferred as architectural).

---

## Redundant file checks on the hook hot path

**Location:** `internal/soundpack/soundpack.go` and
`internal/audio/source.go`.

**Severity:** Low. Every hook run is a fresh process, so the active pack is
loaded each time:

- Loading a JSON soundpack runs `os.Stat` on every mapped file
  (`validateMappingFilesExist`), even though only one sound will play. The
  shipped platform packs have 90–107 mappings.
- Chain resolution then stats each candidate path until one exists
  (`UnifiedSoundpackResolver.ResolveSound`), and playback opens the winning
  file again (`FileSource.Reader` in `internal/audio/source.go`).

With a warm filesystem cache this is invisible. With a cold cache (first
sound after boot, or a pack on a slow or network drive) it adds up.

**Blast radius if fixed:** Check mapped files lazily during resolution
instead of at load time, and have the resolver hand back an opened source
rather than a path. The second change moves file-handle ownership up to the
resolver and changes the shape of every fake resolver in the tests.

**Original finding:** Chunk 14 analyst F7 (pre-existing, flagged for later).

---

## Install workflow tests cover only Claude and Codex

**Location:** `internal/cli/install_command_e2e_test.go` — has
`TestRunInstallWorkflow_EndToEnd_NoDryRun` (Claude) and
`TestRunInstallWorkflowCodexUsesCaptainHookSpecs` (Codex). Gemini, Qwen
Code, and GitHub Copilot CLI have no equivalent.

**Severity:** Low. Those agents' install paths are unit-tested through the
agent registry and settings-merge tests. What is missing is a full
`runInstallWorkflow` run against `.gemini/settings.json`,
`.qwen/settings.json`, and `.copilot/settings.json`. A schema drift would
probably still be caught by the unit tests, but not certainly.

**Blast radius if fixed:** One table-driven test alongside the existing two,
using the Gemini, Qwen, and Copilot agent targets. The plumbing exists.

**Original finding:** Chunk 18 analyst F6.
