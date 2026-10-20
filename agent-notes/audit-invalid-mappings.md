# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

# State

Isolated checkout, branch `fix/audit-invalid-mappings`, based on #98 head `762b28c222aab0ae0b5144e7786290f2e77652c2`. User checkout is unchanged. Fix commit `b0efa05` adds the regression and three-line production change. The regression failed for all three cases before the fix and passed afterward (0.49 s). Current master `733b16feabdbffa8ce3bac77b89aa3a2fe233e5a` merged cleanly into this branch. Publishing this as a replacement draft leaves original #98 unchanged and gives its full feature set current-master CI.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Publish the audit/master replacement draft, integrate it into original #99's head on an isolated branch, and publish a stacked synth/search draft. Await exact-head CI on both. #1's portable log suffix and config-discovery behavior already exist on current master; no unique fix found to preserve. Parent decides final merge/closure; no original PR metadata or branch has been changed.
