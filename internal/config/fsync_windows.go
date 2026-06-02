//go:build windows

package config

import "log/slog"

// fsyncDir is a documented no-op on Windows: NTFS does not expose a
// portable directory-handle flush. The temp+rename pattern is durable
// enough on NTFS for the config.json use case.
//
// Mirrors internal/install/fsync_windows.go.
func fsyncDir(dir string) error {
	slog.Debug("config parent dir fsync skipped on windows", "dir", dir)
	return nil
}
