# Goal

Fix PR #98's reproduced successful audit of a manifest with missing assets; preserve #99's stacked functionality. Parent coordinates merge and closure. No installed hook, authentication, or security changes.

Follow-up goal: audit every valid mapped file, including formats or names excluded from the directory scan. A generated valid Vorbis tone reproduced a zero-file successful strict audit; alongside a clean WAV it reproduced a one-file successful strict audit omitting the Ogg. A mapped WAV named `tone.bin` was also silently skipped. Native playback decodes WAV/MP3/AIFF and reports `ErrUnsupportedFormat` for Ogg; no new decoder is needed to give the truthful failure requested.

# State

Replacement draft #111 uses isolated branch `fix/audit-invalid-mappings`, preserving original #98 head `762b28c222aab0ae0b5144e7786290f2e77652c2`. Original invalid-mapping fix `b0efa05` remains in history. Reviewed published head `1f740347` is preserved as an ancestor; mapped-file correction is commit `48b8831`. Latest master `7e96e43c94fc7de323928dd5adb6ed3450ec0533` merged cleanly in `b2d2138`.

Before the mapped-file correction, CLI regressions observed nil error with zero files for Ogg-only and one file for mixed Ogg/WAV, both with and without strict. Afterward Ogg-only reports one file/one error, mixed reports two files/one error, and a WAV named `.bin` reports 48 kHz/400 ms. All focused audit CLI tests, original invalid-mapping regressions, and master-to-strict-audit round trip passed (5.648 s package). Directory discovery and JSON category/unreferenced behavior remain under existing tests. Full exact-head PR CI is the next check after publication. The user's checkout and installed hooks/settings are unchanged.

# Decisions and why

Smallest fix: use `JSONValidation.Err()` immediately after `ValidateJSONSoundpack`, about three production lines, before a zero-file or partial audit can report success. The regression covers missing-only, mixed valid/missing, and unsafe mappings, with and without `--strict`. No new validation mechanism.

Follow-up smallest fix: seed the existing audit file list with every resolved mapping before sorting and compacting duplicates (about seven production lines). The existing decoder then measures supported content by magic and records precise undecodable errors for unsupported content. Preserve directory-pack discovery and the existing unreferenced-audio scope. CLI regressions exercise real Ogg-only and mixed Ogg/WAV packs in normal and strict modes, shared-file mappings, text/JSON errors, and a valid WAV under a nonstandard extension. The 100 ms Vorbis fixture was generated and independently decoded by ffmpeg, and stored as base64 so tests require no external encoder/decoder.

# Surprises

The original audit discarded `Broken` and `Unsafe` and iterated only `Resolved`; a copied real pack manifest with 132 absent references yielded no audit errors. `master` already checks `Err()`.

# Next step

Publish the updated draft #111, merge its branch normally into draft #112 to preserve the reviewed synth/search/sample/limiter and Nyquist-fix history, and correct the confirmed tolerance documentation drift on #112. Run focused consumer/feature regressions and full exact-head CI on both. Parent requested exact heads for re-review and owns merge, closure and readiness. Do not merge, close, or mark either draft ready.
