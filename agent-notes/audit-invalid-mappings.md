# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

# State

Isolated checkout, branch `fix/audit-invalid-mappings`, based on #98 head `762b28c222aab0ae0b5144e7786290f2e77652c2`. User checkout is unchanged. The new CLI regression failed as expected for all three cases, with and without `--strict`; each returned nil and a successful audit summary. After the three-line production fix, all three cases pass (0.49 s), and `git diff --check` is clean.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Verify the new regression, commit the small fix, integrate current master without rewriting original PRs, and publish draft integration candidates for audit/master and its synth/search dependent. Use exact-head CI for the repository gate. #1's portable log suffix and config-discovery behavior already exist on current master; no unique fix found to preserve.
