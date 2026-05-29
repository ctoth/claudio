//go:build !windows

package config

import (
	"log/slog"
	"os"
)

// fsyncDir opens dir read-only and fsyncs it, making any preceding
// rename in that directory durable across crash on POSIX filesystems.
//
// Mirrors internal/install/fsync_unix.go.
func fsyncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Sync(); err != nil {
		return err
	}
	slog.Debug("config parent dir fsynced", "dir", dir)
	return nil
}
