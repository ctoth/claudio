# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

Follow-up goal: audit every valid mapped file, including formats or names excluded from the directory scan. A generated valid Vorbis tone reproduced a zero-file successful strict audit; alongside a clean WAV it reproduced a one-file successful strict audit omitting the Ogg. A mapped WAV named `tone.bin` was also silently skipped. Native playback decodes WAV/MP3/AIFF and reports `ErrUnsupportedFormat` for Ogg; no new decoder is needed to give the truthful failure requested.

# State

Isolated checkout, branch `fix/audit-invalid-mappings`, based on #98 head `762b28c222aab0ae0b5144e7786290f2e77652c2`. User checkout is unchanged. Fix commit `b0efa05` adds the regression and three-line production change. The regression failed for all three cases before the fix and passed afterward (0.49 s). Current master `733b16feabdbffa8ce3bac77b89aa3a2fe233e5a` merged cleanly into this branch. Publishing this as a replacement draft leaves original #98 unchanged and gives its full feature set current-master CI.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

Follow-up smallest fix: seed the existing audit file list with every resolved mapping before sorting and compacting duplicates (about seven production lines). The existing decoder then measures supported content by magic and records precise undecodable errors for unsupported content. Preserve directory-pack discovery and the existing unreferenced-audio scope. CLI regressions exercise real Ogg-only and mixed Ogg/WAV packs in normal and strict modes, shared-file mappings, text/JSON errors, and a valid WAV under a nonstandard extension. The 100 ms Vorbis fixture was generated and independently decoded by ffmpeg, and stored as base64 so tests require no external encoder/decoder.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Publish the audit/master replacement draft, integrate it into original #99's head on an isolated branch, and publish a stacked synth/search draft. Await exact-head CI on both. #1's portable log suffix and config-discovery behavior already exist on current master; no unique fix found to preserve. Parent decides final merge/closure; no original PR metadata or branch has been changed.
