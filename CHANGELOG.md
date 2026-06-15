# Changelog

All notable user-facing changes to Claudio are summarized here from the local
release tags and current checkout history.

## Unreleased

### Added
- Added command artifact installation for additional agents, including Antigravity.
- Added native Linux embedded default sounds.
- Added a developer `Makefile` for common build, test, CI, smoke, release, and cleanup workflows.

### Fixed
- Fixed resolution of bare embedded soundpack names.
- Improved malgo device initialization under concurrent playback.
- Improved fallback behavior across system audio commands.
- Preserved existing hooks during Claudio reinstall.
- Recognized hook commands by portable executable basename.
- Aligned the native Linux soundpack with Claudio fallback-chain sound keys.
- Improved soundpack install and update behavior.
- Improved detached hook worker stdio handling.
- Ensured configured file logging is attached consistently.
- Improved multi-handler logging error handling.
- Made `--version` handling consistent across flag positions.
- Reused the already-loaded config when initializing tracking.
- Improved install verification for enabled hooks.
- Improved no-CGO and CGO-tag behavior for audio tests.
- Improved audio lifecycle behavior around stop, close, decoder registration, sound IDs, decoder context handling, frame counts, volume access, and malgo context initialization.
- Rejected invalid NaN and Inf volume values.
- Routed configured volume into `paplay`, `ffplay`, and `afplay`.
- Migrated tracking analytics from `fallback_level` to `chain_type`.
- Improved concurrent soundpack and tracking lookup behavior.
- Added size limits and path validation around untrusted soundpack and settings inputs.
- Prevented soundpack symlinks from escaping their soundpack root.
- Preserved non-Claudio hook entries during install and uninstall.
- Improved settings write durability with sync, backup, atomic-write, and locking behavior.
- Fixed Codex hook installation with `CODEX_HOME`.
- Preserved interrupted MCP sound hints.
- Normalized MCP tool names for sound mapping.
- Reduced routine operational log verbosity.

### Changed
- Overhauled the public documentation.
- Split soundpack CLI implementation by subcommand.
- Simplified internal filesystem, CLI, audio, and sound-chain code paths.
- Reworked audio backend construction around `NewBackend`.
- Consolidated audio source handling around `Reader` plus `FilePather`.
- Moved malgo-specific code into its own package.
- Moved WSL detection into `internal/platform`.
- Replaced streaming tracking hooks with explicit `RecordEvent` and lookup-buffer APIs.
- Threaded sound-resolution observation through the CLI.
- Simplified uninstall verification.
- Raised coverage gates for hooks and install packages.

### Documentation
- Updated Claude settings paths, soundpack search paths, soundpack command flags, supported formats, hook events, configuration search order, Windows behavior, fallback chains, and environment-variable references.
- Added Codex hook support design and implementation documentation.
- Updated analysis documentation from `--show-fallbacks` to `--show-chains`.

## v1.13.1 - 2026-04-23

### Fixed
- Fixed directory soundpack audio extension fallback.

## v1.13.0 - 2026-04-22

### Added
- Added git-backed soundpack management.
- Added support for relative paths inside JSON soundpacks.

### Fixed
- Fixed README file mode.

## v1.12.0 - 2026-03-16

### Added
- Added `soundpack use` for switching the active soundpack.
- Added detached hook processing so hook playback can continue without blocking the agent.
- Added shared detach logic for Unix and Windows process handling.
- Added GitHub Actions CI for tests, lint, and no-CGO builds.
- Added release workflow support for cross-platform binary builds.
- Added PermissionRequest and SessionEnd hook support.
- Added the soundpack command group with `init`, `list`, `validate`, and `install`.
- Added the Star Trek TNG bridge soundpack.

### Fixed
- Fixed MSYS path resolution during uninstall on Windows.
- Improved Windows compatibility in tests and hook detection.
- Allowed explicit `volume=0.0` to behave as mute.
- Normalized executable paths for Bash compatibility.
- Skipped audio playback tests when audio hardware is unavailable.
- Resolved lint findings across hook-logger, audio, CLI, and tracking code.

### Changed
- Used the Go builtin `min`.
- Configured CI jobs to run independently.

## v1.11.1 - 2025-11-11

### Added
- Added CGO conditional compilation for the audio subsystem.
- Added missing mappings to the Windows and WSL soundpacks.

### Fixed
- Preserved existing non-Claudio hooks during hook installation.
- Reduced user-visible logging noise.
- Improved platform integration test stability.

## v1.10.1 - 2025-08-14

### Fixed
- Released the `v1.10.x` analysis and tracking updates under the corrected version.

## v1.10.0 - 2025-08-14

### Added
- Added SQLite-backed sound tracking.
- Added tracking configuration and `CLAUDIO_SOUND_TRACKING_DB`.
- Added session ID propagation for per-session tracking.
- Added `analyze missing` for missing-sound reports.
- Added `analyze usage` for usage summaries.
- Added common analysis query infrastructure and filters.
- Added context extraction and tool-first grouping for missing-sound analysis.

### Fixed
- Resolved missing-sound checks through the configured soundpack resolver.

### Documentation
- Documented analysis commands, JSON soundpacks, automatic platform detection, embedded soundpacks, WSL audio utilities, and installation URLs.
- Refreshed documentation and page metadata.

## v1.9.3 - 2025-08-11

### Added
- Embedded bundled soundpacks into the binary.

### Fixed
- Updated tests for embedded soundpack behavior.

## v1.9.2 - 2025-08-11

### Fixed
- Improved platform soundpack detection using the current working directory.
- Improved platform soundpack integration coverage.
- Simplified executable path behavior in the install command.

### Changed
- Added standardized release process documentation.
- Simplified the file-locking dependency footprint.

## v1.9.1 - 2025-08-11

### Fixed
- Restored root-level `go install` support.

## v1.9.0 - 2025-08-11

### Added
- Added WSL platform detection.

### Fixed
- Added fallback behavior for invalid platform soundpack selection.
- Improved platform soundpack detection and hook installer behavior.

## v1.8.2 - 2025-08-10

### Fixed
- Moved the CLI entrypoint under `cmd/claudio` to produce the intended binary name.

## v1.8.1 - 2025-08-10

### Fixed
- Released the `v1.8.x` module-path migration under the corrected version.

## v1.8.0 - 2025-08-10

### Changed
- Completed the Go module path migration from `github.com/ctoth/claudio` to `claudio.click`.

## v1.7.2 - 2025-08-10

### Changed
- Updated the Go module path for `claudio.click` vanity URL support.

## v1.7.1 - 2025-08-10

### Added
- Added Go module vanity URL support for `claudio.click`.

## v1.7.0 - 2025-08-10

### Fixed
- Improved configuration write behavior.

### Changed
- Refactored config and install filesystem access through afero.
- Simplified settings-file write behavior.
- Added a cross-platform example configuration.

## v1.6.1 - 2025-08-10

### Fixed
- Added atomic settings file operations with recovery handling.

## v1.6.0 - 2025-08-03

### Added
- Added full cross-platform audio system support.
- Added end-to-end and CLI integration tests for the unified audio system.

### Fixed
- Fixed 24-bit AIFF playback and audio lifecycle management.
- Updated `NewFileSource` API usage.

### Changed
- Refactored malgo playback to use `AudioPlayer` plus `DecoderRegistry`.
- Consolidated audio playback implementations.
- Expanded unified audio tests across supported formats.
- Enhanced the Darwin soundpack.

## v1.5.0 - 2025-08-03

### Added
- Added AIFF decoder support through `go-audio/aiff`.
- Registered AIFF files in the decoder registry.

### Fixed
- Improved default decoder registry coverage.

### Documentation
- Documented AIFF support.

## v1.4.0 - 2025-08-03

### Added
- Added configurable audio backend support with `auto`, `system_command`, and `malgo`.
- Added `AudioSource` and `AudioBackend` abstractions.
- Added platform detection for backend selection.
- Added platform-specific soundpack detection.
- Added Windows and macOS soundpack coverage.
- Added file logging configuration, XDG log path resolution, and CLI lifecycle wiring.

### Fixed
- Improved Windows hook detection compatibility.
- Fixed project-scope installation directory creation.
- Improved install and uninstall test portability.
- Preserved verbose test logger levels.
- Improved full-path hook removal behavior.
- Updated troubleshooting guidance.

### Changed
- Enabled file logging by default for hook-based usage.

## v1.3.1 - 2025-07-30

### Changed
- Centralized the version constant and updated version references to use it.

## v1.3.0 - 2025-07-30

### Added
- Added SessionStart hook support.
- Added a central hook registry.
- Added website documentation structure, homepage, installation docs, examples, and troubleshooting docs.
- Added virtual JSON soundpack documentation.

### Fixed
- Fixed Claude Code hook format compliance.
- Added executable-path detection in install.
- Added path-based Claudio detection and removal in uninstall.
- Improved CLI logging levels, default configuration consistency, routine log verbosity, and logger setup.
- Made `--version` exit without initializing audio or other systems.
- Improved tool-name logging.
- Fixed Jekyll configuration and README/docs index handling for `claudio.click`.
- Corrected project naming typos.
- Aligned sound-mapping documentation with implementation.

### Changed
- Refactored installation to use a data-driven hook registry.
- Unified README and website documentation.

## v1.2.0 - 2025-07-29

### Added
- Added `claudio uninstall`.
- Added detection and removal of simple and complex Claudio hook configurations.
- Added uninstall workflow integration tests.

### Verified
- Live-tested complete uninstall functionality.

## v1.1.0 - 2025-07-29

### Added
- Added Cobra-based CLI command structure.
- Added `claudio install`.
- Added `install --dry-run`, `--force`, `--quiet`, and `--print`.
- Added Claude settings path detection.
- Added robust settings file reading.
- Added file locking for settings operations.
- Added hook configuration generation.
- Added idempotent hook merging.
- Added complete install workflow orchestration.

## v0.1.2 - 2025-07-28

### Fixed
- Added missing audio decoder definitions.

## v0.1.1 - 2025-07-28

### Changed
- Published the same initial module state as `v0.1.0`.

## v0.1.0 - 2025-07-28

### Added
- Initialized the Go module with malgo dependencies and structured logging.
- Added malgo context management.
- Added WAV and MP3 decoders.
- Added decoder registry magic-byte detection.
- Added memory-based audio playback.
- Added a hook JSON logger for capturing real Claude Code hook payloads.
- Added hook-to-sound mapping with fallback behavior.
- Added XDG Base Directory support.
- Added JSON configuration loading.
- Added real audio playback integration through sound loading and audio player wiring.
- Added soundpack path resolution, JSON soundpacks, soundpack factory detection, and sound file discovery.
- Added Bash command extraction and command-aware sound hints.
- Added original-tool fallback behavior.
- Added category support for completion and system events.
- Added notification type detection.
- Added context extraction for Stop, SubagentStop, and PreCompact.
- Added event-specific fallback chains:
  - PreToolUse enhanced chain.
  - PostToolUse success/error chain.
  - Simple event chain.
- Added initial README and developer notes.

### Fixed
- Fixed config auto-discovery by using the proper XDG path.
- Fixed main CLI integration and audio playback timing.
- Fixed WAV stereo handling.
- Fixed soundpack resolution in the CLI.
- Consolidated duplicate file resolution into sound loading.
- Fixed command parsing so paths are not treated as subcommands.
- Fixed CLI integration tests and module path/imports for `go install`.
- Changed PreToolUse suffix from `-thinking` to `-start`.
