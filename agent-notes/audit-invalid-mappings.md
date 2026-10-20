# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

# State

Audit/master is published as draft #111, head `94fbe02e65faa75207e37db11c743af497de9dbe`, branch `fix/audit-invalid-mappings`. Fix commit `b0efa05` adds the regression and three-line production change. The regression failed for all three cases before the fix and passed afterward (0.49 s). Current master `733b16feabdbffa8ce3bac77b89aa3a2fe233e5a` merged cleanly. CI is running.

The isolated checkout is now on `integrate/soundpack-synth-audit-fix`, starting from original #99 head `cfef149659ef3d1021d23c9c7fa174496359204b` and merging #111's exact head cleanly. Its feature diff against #111 remains the original 16-file synth/search change; the audit regression and fix are inherited. Original #98/#99 branches and prose are unchanged, as is the user checkout.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Publish the stacked synth/search draft against `fix/audit-invalid-mappings` and await exact-head CI on both drafts. #1's portable log suffix and config-discovery behavior already exist on current master; no unique fix found to preserve. Parent decides final merge/closure; no original PR metadata or branch has been changed.
