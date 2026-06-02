//go:build windows

package install

import "log/slog"

// fsyncDir is a documented no-op on Windows: NTFS does not expose a
// portable directory-handle flush. The temp+rename pattern is durable
// enough on NTFS for the settings.json use case.
func fsyncDir(dir string) error {
	slog.Debug("parent dir fsync skipped on windows", "dir", dir)
	return nil
}
