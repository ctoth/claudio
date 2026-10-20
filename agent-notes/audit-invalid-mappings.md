# Goal

Correct false-success audits in replacement drafts #111/#112 while preserving original #98/#99 features and reviewed history. Parent owns merging, closure and readiness. Keep the user checkout and installed hooks/settings unchanged.

# State

#111 is published at `199f3a581d23a56fecd6df1093fd073a02bdecd5`, retaining reviewed head `1f740347`, original invalid-mapping fix `b0efa05`, mapped-content fix `48b8831`, and normal merge `b2d2138` of latest master `7e96e43c94fc7de323928dd5adb6ed3450ec0533`.

#112 merged #111 normally in `68127a1` onto its reviewed head `4de070cf`, preserving original #99's synthesis/search/sample/limiter implementation and Nyquist fix `7df9116`. The only conflict was this historical task note; production and feature docs merged automatically. No original PR branch or prose has changed.

Focused post-integration checks passed on both branches. #111: CLI regressions and mastering round trip (4.000 s), directory/category/unreferenced audit behavior (0.662 s), JSON mastering round trip (0.611 s). #112: CLI regressions, synthesis-to-master-to-strict-audit and topic search (4.115 s); directory/category/unreferenced behavior and both Nyquist WAV rates (1.083 s); peak limiting (0.759 s); sampled recordings and rendered-pack mastering/audit (0.906 s). Synth, sample, search, limiter, descriptor and Nyquist feature files are byte-for-byte unchanged against reviewed #112 head.

Actual CLI reproduction with a valid 100 ms Vorbis tone: before the fix, Ogg-only returned success with zero audited files, and a mixed clean WAV/Ogg pack returned success with just the WAV, even under strict. After the fix, Ogg-only reports one file/one undecodable error and mixed reports two files/one undecodable error, in normal and strict modes and JSON/text output. Two keys sharing the Ogg produce one file/error. A WAV named `.bin` is now decoded by magic and measured at 48 kHz/400 ms. Original invalid-mapping and master-to-strict-audit regressions also passed (5.648 s CLI package).

# Decisions and why

Seed the existing file list with every resolved mapping before sorting/compacting (about seven production lines), then use the existing playback decoder and undecodable finding. Native playback supports WAV/MP3/AIFF and rejects Ogg; do not invent measurements or add a separate audit decoder. Retain existing unreferenced-audio scope and directory discovery. The valid Vorbis fixture was generated and independently decoded by ffmpeg, stored as base64; tests require no external encoder/decoder.

The original missing/unsafe mapping fix remains: check `JSONValidation.Err()` before iterating resolved mappings. The Nyquist endpoint guard and WAV-through-audit regressions at 22050/24000 Hz remain unchanged.

Corrected documentation drift on #112: `master.Process` uses `ToleranceLU/2` to stop peak limiting, so `--tolerance` can alter output. The CLI reference now describes this instead of claiming it only affects reporting. Behavior is unchanged.

# Surprises

The audit's directory extension filter silently discarded valid mapped content before decoding, including unsupported Ogg and supported WAV bytes under an unusual extension. Explicit mapped inputs must be included independently of directory discovery.

# Next step

Publish #112's integrated history and tolerance prose correction, await full exact-head CI on both drafts, then return exact heads for parent re-review. Keep both PRs draft; do not merge or close them. No remaining source conflict or focused-test failure.
