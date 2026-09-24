// Package testenv provides test-isolation helpers that sandbox HOME,
// USERPROFILE, and XDG_* env vars to t.TempDir() so cli tests do not
// pollute the developer's real ~/.claude or ~/.cache/claudio.
//
// This package is intended to be imported only from _test.go files in the
// cli and related packages. It is a real Go package (not a _test.go
// helper) so it can be reused across packages.
package testenv

import (
	"path/filepath"
	"testing"

	"claudio.click/internal/audio/audiotest"
	"github.com/adrg/xdg"
)

// IsolateXDG sets HOME, USERPROFILE, LOCALAPPDATA, XDG_CACHE_HOME,
// XDG_DATA_HOME, XDG_CONFIG_HOME, XDG_DATA_DIRS, and XDG_CONFIG_DIRS to
// subdirectories under t.TempDir() and calls
// xdg.Reload() so the adrg/xdg library picks up the new values.
//
// It also sets CLAUDIO_FILE_LOGGING=false so the lumberjack file
// handle does not block t.TempDir() cleanup on Windows (where an
// open file cannot be removed). The override applies regardless of
// which config path the test follows because
// ApplyEnvironmentOverrides runs after every load path.
//
// Environment restoration is handled by t.Setenv (auto-restores via
// t.Cleanup). A final xdg.Reload() is registered via t.Cleanup before any
// t.Setenv, so it runs after the environment is restored and resets the
// xdg package's cached paths back to the host environment when the test
// ends.
//
// Returns the root sandbox directory for callers that need to construct
// specific paths within it (e.g. config files under .config/claudio).
//
// The helper does NOT call t.Parallel() — callers may add it themselves
// but should be aware that xdg.Reload() mutates package-global state and
// is not safe across concurrent IsolateXDG callers in the same process.
func IsolateXDG(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Cleanups run last-in, first-out: registering the reload before any
	// t.Setenv makes it run after they restore the environment, so adrg/xdg
	// re-reads the real paths rather than the deleted sandbox (#92).
	t.Cleanup(func() { xdg.Reload() })

	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	t.Setenv("LOCALAPPDATA", filepath.Join(root, ".cache"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, ".cache"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, ".local", "share"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, ".config"))
	// Without these, xdg falls back to the host's system dirs
	// (/usr/local/share, /etc/xdg, %ProgramData%) and installed soundpacks
	// or configs there leak into tests.
	t.Setenv("XDG_DATA_DIRS", filepath.Join(root, "data-dirs"))
	t.Setenv("XDG_CONFIG_DIRS", filepath.Join(root, "config-dirs"))

	// Disable file logging via env var so the lumberjack file handle
	// does not block t.TempDir() cleanup on Windows. This applies
	// regardless of whether the test passes an explicit --config or
	// relies on XDG autodiscovery, because the override runs in
	// ApplyEnvironmentOverrides after every load path.
	t.Setenv("CLAUDIO_FILE_LOGGING", "false")

	// Route every in-process cli.Run call through the fake audio backend so
	// the test binary never blocks waiting for a real audio device. The fake
	// stands in for "oto"; tests that assert on Play invocations read
	// audiotest.LastFakeBackend().Plays().
	audiotest.Install(t)
	t.Setenv("CLAUDIO_AUDIO_BACKEND", "oto")

	xdg.Reload()

	return root
}
