# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

# State

Audit/master is published as draft #111, head `94fbe02e65faa75207e37db11c743af497de9dbe`, branch `fix/audit-invalid-mappings`. Fix commit `b0efa05` adds the regression and three-line production change. The regression failed for all three cases before the fix and passed afterward (0.49 s). Current master `733b16feabdbffa8ce3bac77b89aa3a2fe233e5a` merged cleanly. CI is running.

Draft #112 is stacked on #111, branch `integrate/soundpack-synth-audit-fix`, starting from original #99 head `cfef149659ef3d1021d23c9c7fa174496359204b`. Its initial integration head `244487aedc6c0967d84636368eef5a31a74f58ba` passed all nine checks.

A further focused probe reproduced an audit panic on a valid 22050 Hz PCM WAV carrying its Nyquist frequency: `index out of range [1025] with length 1025` at `fundamental`, through Pack -> DecodeFrames -> Analyze -> Describe. The #99 review ledger was updated and passed the original gate. #112 now has a small fix that checks the final spectral bin and reports its frequency without interpolation, plus WAV-through-audit regressions at 22050 and 24000 Hz. Both regressions pass (0.03 s), and the working diff passes `git diff --check`.

Actual master `68d9090b3e9ab8e09361089110ef9f106e496a4d` includes parent's merged #108 and #110. It is merged into #111 as `1f7403473db1ad91f502d32d09622694897eb73d`, and into this synth branch through #111. Original #98/#99 branches and prose are unchanged, as is the user checkout.

Parent additionally authorized integrating #109. Its branch belongs to `serrebidev/claudio` with maintainer edits enabled. Isolated worktree `../claudio-warn-integration`, branch `maintenance/warn-pack-integration`, contains normal merge `9dc5f253cad3e7ab12276124ebe7ab66341f2dd3` (parents #109's `85c8495` and master `68d9090`). Only changelog conflict required resolution; both entries retained. Soundpack factory/resolution-warning and startup-warning-to-file focused tests pass. Push is in progress from this owned parent repository after the elevated shell correctly refused the sandbox-owned linked worktree; no trust or permission settings changed.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Publish the stacked synth/search draft against `fix/audit-invalid-mappings` and await exact-head CI on both drafts. #1's portable log suffix and config-discovery behavior already exist on current master; no unique fix found to preserve. Parent decides final merge/closure; no original PR metadata or branch has been changed.
