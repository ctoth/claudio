# Goal

Correct false-success audits in replacement drafts #111/#112 while preserving original #98/#99 features and reviewed history. Parent owns merging, closure and readiness. Keep the user checkout and installed hooks/settings unchanged.

# State

#111 is published at `199f3a581d23a56fecd6df1093fd073a02bdecd5`, retaining reviewed head `1f740347`, original invalid-mapping fix `b0efa05`, mapped-content fix `48b8831`, and normal merge `b2d2138` of latest master `7e96e43c94fc7de323928dd5adb6ed3450ec0533`.

#112 is merging #111 normally onto its reviewed head `4de070cf`, preserving original #99's synthesis/search/sample/limiter implementation and Nyquist fix `7df9116`. The only conflict was this historical task note; production and feature docs merged automatically. No original PR branch or prose has changed.

Actual CLI reproduction with a valid 100 ms Vorbis tone: before the fix, Ogg-only returned success with zero audited files, and a mixed clean WAV/Ogg pack returned success with just the WAV, even under strict. After the fix, Ogg-only reports one file/one undecodable error and mixed reports two files/one undecodable error, in normal and strict modes and JSON/text output. Two keys sharing the Ogg produce one file/error. A WAV named `.bin` is now decoded by magic and measured at 48 kHz/400 ms. Original invalid-mapping and master-to-strict-audit regressions also passed (5.648 s CLI package).

# Decisions and why

Seed the existing file list with every resolved mapping before sorting/compacting (about seven production lines), then use the existing playback decoder and undecodable finding. Native playback supports WAV/MP3/AIFF and rejects Ogg; do not invent measurements or add a separate audit decoder. Retain existing unreferenced-audio scope and directory discovery. The valid Vorbis fixture was generated and independently decoded by ffmpeg, stored as base64; tests require no external encoder/decoder.

The original missing/unsafe mapping fix remains: check `JSONValidation.Err()` before iterating resolved mappings. The Nyquist endpoint guard and WAV-through-audit regressions at 22050/24000 Hz remain unchanged.

Confirmed documentation drift on #112: `master.Process` uses `ToleranceLU/2` to stop peak limiting, so `--tolerance` can alter output. Correct its CLI reference wording while preserving behavior.

# Surprises

The audit's directory extension filter silently discarded valid mapped content before decoding, including unsupported Ogg and supported WAV bytes under an unusual extension. Explicit mapped inputs must be included independently of directory discovery.

# Next step

Finish the normal #112 merge and tolerance prose correction, run focused consumer/feature regressions and full exact-head CI on both, then return exact heads for parent re-review. Keep both PRs draft; do not merge or close them.
