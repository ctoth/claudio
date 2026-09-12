# Repository audit and repairs, September 12, 2026

The actionable audit findings have been repaired. Fresh Windows checks pass, including the full test suite with and without CGO, race detection, vet, lint, and module verification. The first PR CI run passed Linux and Windows tests, all coverage gates, lint, the non-CGO build, and the Jekyll build. It also exposed two macOS cache-isolation test defects, repaired in a follow-up commit. See [PR #34](https://github.com/ctoth/claudio/pull/34) for checks on the latest revision. The numbered findings below describe the audited baseline.

The checkout was fast-forwarded from `1b57c92` to `fdd35ca4c5d13e4980202cae30d6ac3bfd3cb3e9` before the audit. The baseline had 329 tracked files, including 183 Go files, 100 Go test files, 15 Markdown documents, 107 MP3s, and five WAVs. The Go tests define 697 top-level tests. The complete tracked-file list is in [inventory.txt](inventory.txt).

Three independent AI-assisted reviews covered runtime code, CLI and integrations, and documentation and CI. Findings were checked through source review and isolated reproductions. Implementation, automated checks, and the NVDA interaction were performed by coding agents.

## Why Codex says "running hook(s)"

The user confirmed that the message is brief and work continues. This is normal Codex activity reporting, not evidence of a stuck hook.

The installed Codex CLI is `0.154.0`. The installed Claudio executable was built from module version `v1.14.0`, using Go `1.26.4`. Both that release and the newer checkout report Claudio version `1.14.0`, so the version banner alone cannot distinguish their implementations.

The local `~/.codex/hooks.json` registers Claudio for wildcard `PreToolUse` and `PostToolUse` events, plus session, prompt, subagent, permission, and compaction events. The handlers have no custom status message, no explicit timeout, and no asynchronous flag. Thus commands invoke Claudio before and after execution.

Codex 0.154.0's [hook display implementation](https://github.com/openai/codex/blob/rust-v0.154.0/codex-rs/tui/src/history_cell/hook_cell.rs) waits 300 milliseconds before exposing activity. Its `running_status_summary` returns "Running hook" or "Running hooks" when no shared custom status message exists. This text comes from Codex, not Claudio's stdout or stderr. Empty hook output does not suppress the activity indicator.

Five local measurements with ordinary, closed stdin gave median startup times of 158 ms for direct installed Claudio, 1.15 seconds through Windows PowerShell, and 2.54 seconds through PowerShell 7. These were diagnostic samples during other audit activity, not controlled performance benchmarks or proof of the exact shell selected by the running Codex host. They demonstrate how shell startup can exceed the display threshold even when Claudio succeeds without output.

Updating the executable does fix a separate, reproduced hang. In the old release, hook reading waited for EOF. Sending complete JSON but keeping the pipe open left the old process blocked until the probe killed it after three seconds. The newly built baseline exited in 113 ms. The new `ReadJSONBounded` reader returns once valid JSON arrives. Normal closed-pipe medians were 111 ms and 123 ms respectively. See [probe_hooks.py](probe_hooks.py) and [hook-probe.json](hook-probe.json). This is not the brief-message symptom the user described.

Other supported agents' hook registrations were inspected. Their user interfaces were not compared, so identical notices across hosts were not established.

Options for reducing notices have different effects:

- A custom `statusMessage` can replace the generic wording when Codex displays one hook or a group with the same message.
- Asynchronous Claudio hooks can reduce waiting, but do not guarantee that Codex hides their activity. They cannot make policy or approval decisions, so this should apply only to advisory audio hooks.
- Disabling Claudio's before/after-tool hooks through `/hooks` reduces notices at the cost of those audio cues. Keep unrelated hooks intact.

The [official hook documentation](https://learn.chatgpt.com/docs/hooks) describes synchronous defaults, asynchronous execution, hook trust, and `/hooks`. Changes to a hook definition require reviewing its new hash. No installed hook settings were changed during this audit.

## Confirmed defects in the baseline

1. **High: JSON soundpack names escape the installation directory.** In `internal/cli/soundpack_install.go`, baseline lines 101–115, the manifest's `name` becomes part of the destination path without validation. A valid pack named `../audit-escape` overwrote a marker outside the Claudio directory while normal validation was enabled. The demonstration stayed inside an isolated temporary directory. Restrict names to safe components and check destination containment before writing.

2. **High: JSON installation loses companion audio.** The same installer, baseline lines 118–128, copies only the manifest. A manifest with `default.wav` mapped to `tone.wav` validates at its source, installs successfully, then fails validation at its destination because `tone.wav` was not copied. Install the manifest and its referenced files together and validate the installed result before changing configuration.

3. **High: soundpack commands discard damaged configuration.** `soundpack_use.go`, baseline lines 69–72, `soundpack_install.go`, lines 149–153, and `soundpack_git.go`, lines 588–610, replace any load error with defaults before saving. A malformed config containing recoverable user data was overwritten by `soundpack use windows`, which returned success. Only a missing file should permit defaults; parse and permission errors must preserve existing data.

4. **Medium: soundpack mutations ignore `--config`.** These commands create their own config managers and write the default XDG path. A probe selected a custom config, but `soundpack use linux` left that file unchanged and modified the default file instead. Resolve one writable path from the command and use it throughout the operation.

5. **Medium: config mutators bypass locking and atomic writes.** `ConfigManager.SaveToFile`, baseline lines 161–190, uses a direct truncating write. Soundpack mutations use it outside `LockConfigDir`, unlike volume and mute. The inconsistent paths are source-confirmed; a concurrent lost-update or crash was not forced. All mutators need one locked read/mutate/atomic-write operation.

6. **Medium: skipped fallback sounds are reported as missing.** `internal/soundpack/soundpack.go`, baseline lines 177–185, emits `exists=false` for candidates after the first winner without inspecting them. A probe with both candidates present reported the second one missing. Tracking stores those values and `analyze missing` counts them. Record actual lookups, or represent "not checked" explicitly.

7. **Medium: analytics can contradict their filter.** `GetSoundUsage`, baseline `internal/tracking/analyzer.go:303–355`, selects context through a subquery that ignores the outer filters. Two tools sharing a sound reproduced a report filtered to `Write` but labeled `Read`. Missing-sound grouping also chooses one context for a path that can span several tools and categories. Group by the dimensions displayed or return all dimensions accurately.

8. **Medium: invalid analytics filters silently mean something else.** `query_builder.go`, baseline lines 39–46 and 205–223, treats an invalid date preset as no lower time bound and an unknown category as loading. The probe's `todai` and `succes` inputs produced a valid all-history/loading query. Reject these inputs at the command boundary.

9. **Medium: pooled in-memory tracking connections have different schemas.** `NewDatabase(":memory:")` does not constrain the connection pool. Holding its initialized connection and making another query produced `no such table: hook_events`. Use one connection for a private in-memory database.

10. **Medium: directories are accepted as audio files.** The resolver checks only whether `os.Stat` succeeds. A directory named `directory.wav` was returned as a resolved sound. Require a regular file in validation and resolution. POSIX special-file behavior was not executed on this Windows host.

11. **High: integration tests contaminate the user's tracking database.** `tracking_end_to_end_test.go`, baseline lines 501–505, enables tracking without an isolated default cache path. A read-only query found 52 rows with the exact test session ID `env-config-test-tracking_enabled_default_path`, including a row from this audit's test run. The test's stderr-only assertion does not prove correct storage. Isolate the actual platform cache directory and assert against the temporary database. Existing user data was not deleted.

12. **Medium: command artifact installation and removal lose custom content.** `install_commands.go`, baseline lines 251–290, overwrites existing command or skill files and removes them without checking ownership. Regression tests reproduced both overwrite and deletion of customized content. Refuse destructive replacement or removal when content differs from the generated artifact.

13. **Medium: Codex installation overwrites malformed hook structures.** The Captain Hook adapter silently replaces a wrong-shaped `hooks` object or event entry. End-to-end tests reproduced a successful installation that discarded the existing structure. Validate those structures before passing them to the adapter.

14. **Medium: generated PowerShell commands expand legal path characters.** `GenerateCodexHookSpecs`, baseline `internal/install/hooks.go:47`, double-quotes the executable path. An execution test copied a system utility into `Cash$rate O'Brien`; the generated command expanded `$rate` and failed to find it. Use a PowerShell literal string with escaped apostrophes.

15. **High accessibility impact: the mobile documentation menu cannot be opened by keyboard.** `docs/_layouts/default.html` hides its checkbox with `display:none` and uses an unnamed, nonfocusable label as the only control. A headless Edge fixture extracted from the template exposed seven links at 800 pixels, but no links or menu tab stop at 375 pixels. A pointer click revealed the links. The repair preserves a native checkbox, gives it a name and keyboard focus, and exposes its checked state. NVDA verification is linked below.

16. **Medium: local lint tooling uses the wrong major version.** `.pre-commit-config.yaml` pins golangci-lint 1.64.8 while the repository configuration uses schema version 2 and CI uses 2.12. Align the versions and test their configuration contract.

The independent CLI reproductions are retained in [probe_cli.py](probe_cli.py) and [cli-probe.json](cli-probe.json). Runtime reproductions are in [probe_runtime.go](probe_runtime.go) and [runtime-probe.json](runtime-probe.json). Navigation evidence is in [probe_docs.py](probe_docs.py) and [docs-probe.json](docs-probe.json). These JSON files preserve the failing baseline behavior. Compare them with the [CLI](cli-probe-fixed.json), [runtime](runtime-probe-fixed.json), [hook](hook-probe-fixed.json), and [navigation](docs-probe-fixed.json) repair results. [NVDA verification](nvda-verification.md) records native keyboard and screen-reader evidence.

## Documentation and tooling findings from the baseline

The documentation and tooling issues below were corrected, except for the unresolved asset-provenance question. This list describes what the audit found before repair.

- The soundpack guide permitted absolute on-disk mapping paths while the runtime explicitly rejected them. The shipped Star Trek manifest contained 107 machine-specific absolute mappings. The guide, validator, installer, and shipped manifest now use safe relative paths.
- The remote-audio debugging snippet assigns `CLAUDIO_LOG_LEVEL` without exporting it. Several CLAUDE.md examples use an empty device as a JSON config, triggering errors and fallback instead of clean setup. Another assigns the variable to `echo`, not Claudio.
- The changelog lacks a distinct v1.14.0 release boundary. Several architecture and investigation documents refer to missing reports or tests that have since been added. LLM-facing guidance also contains stale agent, asset-location, and platform-path descriptions.
- The documentation layout has no skip link and emits an empty email link when no email is configured.
- `test_tracking.sh` uses a fixed database path, an arbitrary existing executable, and does not reliably propagate failure.
- Release automation tests and builds but does not check tag/banner consistency or enforce the full quality gate. The 97.9% coverage requirement covers only hooks and installation.
- The audio download report records source attribution and describes personal/hobby use, but the repository does not establish redistribution permission. This is a provenance question, not a legal determination. No assets were removed and no license was invented.
- Human-facing documentation was reviewed using unslop. Specific problems include repeated caveats, promotional agent-specific descriptions that no longer match support, and dense explanations. LLM-facing guidance was checked for correctness rather than subjected to prose compression.

## Follow-up findings and repairs

These concerns began as source review findings. Follow-up work established the following dispositions:

- Audio shutdown was reproduced with a play blocked immediately before device startup. `Close` returned while that play still needed the context. A lifecycle read lease now retains the context until each admitted play cleans up, and shutdown wakes active plays before `Close` waits. Device startup and `StopAll` share a lock. The regression passed after repair.
- Two concurrent plays of the same sound shared one registry key. Devices now have independent entry identities. A behavioral test starts two real plays of a 30-second sound, stops both, and checks prompt completion. It passed without a hardware skip. This test was added after the implementation; an earlier compile-only test is not claimed as behavioral failure evidence.
- MP3 decoding now has a 100 MiB decoded PCM limit, reusing the existing encoded-audio budget. Boundary tests use a small limit to check overrun rejection, final bytes returned with EOF, and cancellation without allocating a large fixture. This is preventive bounding, not a reproduced excessive-allocation exploit. Valid PCM WAV/AIFF output remains bounded by their encoded-file limit. WAV now converts each sample chunk immediately instead of retaining a second whole-file sample array. A 5,001-frame stereo regression verifies identical output across chunk boundaries. These limits do not cap total process memory or impose a duration limit.
- Tiny WAV fixtures reproduced an out-of-range panic for three channels and division by zero for zero block alignment. The decoder now validates channel count, block alignment, encoding, and float bit depth before invoking the library's sample reader. Five malformed-layout regressions passed after failing against the old implementation.
- A deterministic migration failure dropped an old column before failing on the next DDL statement. Migrations now run schema creation, inspection, changes, and version stamping on one connection under `BEGIN IMMEDIATE`, with rollback on error. Stress testing also reproduced `SQLITE_BUSY` during WAL connection initialization; bounded retries handle that specific result code. Concurrent migration and rollback regressions pass.
- The config discovery regression reproduced defaults being returned despite a config in the injected memory filesystem. Discovery now uses the manager's filesystem.
- Concurrent system-command playback reproduced an incorrect idle state while another command was active. The backend now tracks individual plays.
- The developer hook logger now uses the user's cache directory, a private directory, exclusive unique files, sanitized event names, and bounded JSON reads. Invalid input is neither saved nor echoed. Valid payloads remain intentionally available for debugging, with an explicit sensitive-data warning. Windows tests verify isolation, naming, uniqueness, and pipe behavior; Unix permission assertions require Unix CI. The initial testable entry-point test failed to compile before implementation, so no prior behavioral failure is claimed for that test.

## Repair verification

The first PR CI run caught two macOS-specific test defects introduced during repair. The integration assertion assumed an XDG cache path, although macOS uses `HOME/Library/Caches`; its database was already isolated. The developer logger helper set XDG and Windows cache variables but omitted `HOME`, so its macOS writes could reach the runner's real cache. Both helpers now check the platform cache path within their temporary root, and the logger isolates `HOME` and `USERPROFILE` before running. This also makes the invalid-payload test inspect the correct directory on macOS.

Fresh checks on Windows amd64 with Go 1.26.6 passed:

- [Full tests and coverage](verified-tests.txt), using `go test ./... -count=1` with a coverage profile.
- [Full race tests](verified-race.txt), using `go test ./... -race -count=1`.
- [Full tests without CGO](verified-nocgo-tests.txt) and a build of all packages with `CGO_ENABLED=0`.
- `go vet ./...`, golangci-lint 2.12.2, a CGO-enabled CLI build, `go mod tidy -diff`, and `go mod verify`.
- The [required coverage gate](verified-coverage-floor.txt), with hooks at 99.8% and installation at 98.0%, above the 97.9% floor.
- The [dependency vulnerability scan](patched-dependency-vuln.txt), with no vulnerabilities reported after the toolchain and dependency updates.
- All 112 bundled assets passed metadata inspection and [actual decoding](decoded-assets-final.json). Their PCM hashes were identical before and after the WAV chunking change. The largest decoded asset was 43,716,096 bytes.
- The isolated CLI, runtime, hook, and navigation probes linked above passed their repair checks. The runtime probe intentionally retains invalid direct query-builder calls for baseline comparison; validation now happens at the CLI boundary.
- Git Bash syntax and execution of `test_tracking.sh` passed. The script created and removed its own temporary data. A read-only check of the previously contaminated real database found the same 52 test-session rows after the repaired test runs, with no additions to that session.
- NVDA 2026.1.1 with Firefox verified mobile menu naming, checked state, opening, closing, and keyboard traversal. Headless Edge verified desktop and mobile layout behavior.
- The patch whitespace check passed with `core.whitespace=cr-at-eol`, preserving the baseline's committed CRLF files.

The original user database, installed Claudio binary, and installed hook settings were not changed. Registry and config remain separate files; filesystem failures between their writes are not a cross-file transaction. Matching add/remove retries now repair the corresponding config state. Customized command artifacts are preserved conservatively, which can require manual review when an older generated template differs from the current template.

## Baseline check results

Completed on Windows amd64 with Go 1.26.4 before repairs:

- Fresh full test suite passed.
- Fresh full race-detector suite passed.
- `go vet ./...` passed.
- golangci-lint 2.12.2 reported zero issues.
- CGO build and `CGO_ENABLED=0` build passed.
- `go mod verify` reported all modules verified.
- Coverage was 99.8% for hooks, 98.0% for installation, 97.1% for safe I/O, 92.1% for direct audio, 87.3% for uninstall, 86.9% for mapping, 83.4% for soundpacks, 78.7% for tracking, 74.6% for config, 73.3% for malgo, and 72.1% for CLI. These measurements do not establish that all behaviors are tested.
- `gofmt -l` reported every Go file because of checkout line endings. Comparing LF-normalized content found 29 files with additional formatting differences. This was not treated as 183 separate defects.

govulncheck 1.8.0 found a reachable standard-library advisory, [GO-2026-6088](https://pkg.go.dev/vuln/GO-2026-6088), through `mimetype.Detect` and XML parsing. The advisory is fixed in Go 1.26.6 and 1.25.13. Claudio passes only a 512-byte header to this detector, which limits this particular input path. Symbol reachability is not proof of an exploitable stack-exhaustion attack in Claudio. The repository now requires Go 1.25.13 and selects Go 1.26.6. Follow-up scanning also identified two unreachable dependency advisories, [GO-2026-5970](https://pkg.go.dev/vuln/GO-2026-5970) and [GO-2026-5942](https://pkg.go.dev/vuln/GO-2026-5942). Those dependencies were updated to fixed versions. The final dependency scan reports no vulnerabilities. See [scan output](patched-dependency-vuln.txt).

Local scope limits: WSL reported no installed Linux distributions for this Windows user. Ruby/Jekyll was absent locally. Linux/macOS tests and the new nonpublishing Jekyll build run through PR CI. WSL audio, remote SSH playback, other agents' UIs, JAWS, and VoiceOver were not exercised. Windows testing included live NVDA/Firefox navigation on an extracted template fixture and headless Edge at desktop and mobile widths. Git Bash ran the isolated tracking smoke script. The tracked-file inventory, reviewed packages, and executed checks define the audit scope; this is not a formal proof of every source or test line. Audio redistribution permission still requires provenance from the asset owner.
