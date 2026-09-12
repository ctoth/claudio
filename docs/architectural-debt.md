# Architectural Debt Log

This file records deferred architecture work. These items describe
maintainability or testability costs rather than known user-facing failures.

Each entry names the finding, the location, the severity (impact if
left unfixed), the blast radius (what changes when you do fix it), and
the original audit reference when that source is still available.

---

## #51 — CLI struct DI container is half-applied

**Location:** `internal/cli/cli.go` (the `CLI` struct + its
`configManager` field) versus every direct call to
`config.NewConfigManager()` scattered across the verbs.

**Severity:** Medium. The CLI struct exists to be a DI container for
config-, fs-, and backend-injection, but at least seven verb / helper
sites side-step it with their own `config.NewConfigManager()` call.
Tests cannot inject a fake config manager into those sites, so the
verbs end up reading the developer's real `~/.config/claudio/config.json`
unless `testenv.IsolateXDG(t)` has already been called — which is now
the norm but is enforced by convention, not by the type system.

**Blast radius if fixed:** Touches every soundpack subcommand verb,
the volume / mute / unmute / status verbs, and the analyze verb. Either
commit to a context-DI pattern (`ctx`-borne `*CLI`) or pass deps as
function arguments. The "half-applied DI container" state is the worst
of both worlds.

**Original finding:** Internal audit item #51; the source report is not retained.

---

## #52 — `initializeAudioSystem` is 128 lines of soundpack resolution

**Location:** `internal/cli/cli.go` `initializeAudioSystem` (search by
name; line number drifts).

**Severity:** Medium. The function is named for audio backend init but
most of its body is soundpack-path resolution: walking the active
soundpack name through XDG data dirs, soundpack_paths config entries,
embedded platform JSONs, and managed git packs. That's
soundpack-package business logic in CLI plumbing.

**Blast radius if fixed:** Extract a new function (or type) in
`internal/soundpack/resolve` that takes a name and returns a resolved
`soundpack.Source`. CLI then composes audio backend init separately
from soundpack resolution. The split is mechanical but the function is
long, well-tested, and currently green; risk is in subtly changing
resolution precedence during the move.

**Original finding:** Internal audit item #52; the source report is not retained.

---

## #54 — Soundpack discovery lives in `internal/cli`

**Location:** `internal/cli/soundpack_helpers.go` (after the Chunk 20
split) — `discoverSoundpacks`, `discoverEmbeddedSoundpacks`,
`discoverXDGSoundpacks`, `discoverConfigSoundpacks`, `countAudioFiles`,
`countNonEmptyMappings`.

**Severity:** Medium. Soundpack discovery (which packs exist, where,
how many sounds each has) is conceptually `internal/soundpack` territory.
It is in `internal/cli` because the CLI was the only consumer when the
code was first written. Today the `analyze`, `soundpack list`, and
`soundpack use` verbs all reach into it, plus `soundpack_git.go` for
the managed-git registry. Moving to `internal/soundpack` would let
non-CLI consumers (e.g. a future TUI) reuse the discovery.

**Blast radius if fixed:** Most call sites are in-package today, so the
move is `git mv` + import updates. The shared `soundpackInfo` type
would need to move with it.

**Original finding:** Internal audit item #54; the source report is not retained.

---

## Atomic-write duplication between install and config

**Location:** `internal/install/settings_io.go` and
`internal/config/atomic_io.go`. Both implement the same temp-file +
fsync + rename + parent-dir-fsync atomic write pattern, separately.

**Severity:** Medium-low. The two implementations are aligned today
but drift is inevitable — Chunk 4's `.bak` discipline only lives in
settings_io; Chunk 7's NaN/range guards only live in atomic_io. A bug
fix on one side that should land on both will be forgotten.

**Blast radius if fixed:** Lift the shared core into
`internal/safeio/atomicjson` (the existing `safeio` package already
owns the size-capped read primitive, so this is a natural fit).
Settings_io keeps its lock and backup discipline on top; atomic_io
becomes a thin wrapper. Both call sites continue to pass through their
own pre-write validation.

**Original finding:** Internal audit items from chunks 4 and 7; the source
reports are not retained.

---

## Two-Stat-per-resolved-sound pattern

**Location:** The resolver in `internal/soundpack` and the playback
layer in `internal/audio` both `os.Stat` the same file on the hot
path — once to confirm existence during chain resolution, then again
when the source is opened for playback.

**Severity:** Low. Two stats per sound is invisibly cheap on a modern
filesystem cache. It is on the hook hot path though, so if the cache
is cold (first sound after boot, or after the file changed) the cost
doubles. Worth a single-stat refactor eventually.

**Blast radius if fixed:** Resolver returns an opened `*os.File` (or
an `audio.AudioSource`) instead of a path. Tracking observer gets the
same file object. Eliminates the second stat at the cost of moving
file-handle ownership up to the resolver. Test surface is moderate:
every test that constructs a fake resolver would change shape.

**Original finding:** Internal audit item F7; the source report is not retained.

---

## Gemini install e2e variant not yet written

**Location:** `internal/cli/install_command_e2e_test.go` covers Claude and
Codex install end-to-end; Gemini has no equivalent test.

**Severity:** Low. The Gemini install path is unit-tested through the agent
registry and settings merge tests. The missing piece is a full install workflow
test against `.gemini/settings.json` under
`CLAUDIO_TEST_RECOGNIZE_GO_TEST`.

**Blast radius if fixed:** Add a Gemini case beside the existing Claude and
Codex end-to-end tests.

**Original finding:** Internal audit item F6; the source report is not retained.
