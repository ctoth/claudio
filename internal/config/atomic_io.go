package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/afero"

	"claudio.click/internal/safeio"
)

// WriteConfigFile backs up the existing config (see BackupConfigFile) and
// then replaces filePath with cfg via safeio.WriteJSONFile, which owns the
// temp-file + fsync + rename + parent-dir-fsync atomic write.
//
// Callers MUST hold LockConfigDir(filePath) for the entire
// read-mutate-write window.
func WriteConfigFile(filesystem afero.Fs, filePath string, cfg *Config) error {
	// Non-fatal: a missing .bak is better than a blocked write.
	BackupConfigFile(filesystem, filePath)
	return safeio.WriteJSONFile(filesystem, filePath, cfg, ".config-*.tmp")
}

// BackupConfigFile copies filePath to filePath+".bak" iff it exists and
// parses as JSON. Failure is logged at WARN but not returned — a
// missing backup is better than a blocked write, and refusing to
// overwrite a valid .bak with a corrupt source preserves the
// last-known-good copy. Uses temp+rename so a crash mid-write cannot
// corrupt the recovery file.
//
// Mirrors internal/install/settings_io.go's BackupSettingsFile.
func BackupConfigFile(filesystem afero.Fs, filePath string) {
	info, err := filesystem.Stat(filePath)
	if err != nil {
		return // no file, no backup needed
	}
	data, err := afero.ReadFile(filesystem, filePath)
	if err != nil {
		slog.Warn("config backup skipped: read failed", "path", filePath, "err", err)
		return
	}
	var probe Config
	if err := json.Unmarshal(data, &probe); err != nil {
		slog.Warn("config backup skipped: existing file is not valid JSON, refusing to overwrite .bak",
			"path", filePath, "err", err)
		return
	}

	bakPath := filePath + ".bak"
	bakDir := filepath.Dir(bakPath)
	mode := info.Mode() & os.ModePerm

	tempFile, err := afero.TempFile(filesystem, bakDir, ".config-bak-*.tmp")
	if err != nil {
		slog.Warn("config backup skipped: temp file create failed", "path", bakPath, "err", err)
		return
	}
	tempName := tempFile.Name()
	cleanup := func() { _ = filesystem.Remove(tempName) }

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		cleanup()
		slog.Warn("config backup skipped: temp write failed", "path", bakPath, "err", err)
		return
	}
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		cleanup()
		slog.Warn("config backup skipped: temp sync failed", "path", bakPath, "err", err)
		return
	}
	if err := tempFile.Close(); err != nil {
		cleanup()
		slog.Warn("config backup skipped: temp close failed", "path", bakPath, "err", err)
		return
	}
	if err := filesystem.Chmod(tempName, mode); err != nil {
		cleanup()
		slog.Warn("config backup skipped: chmod failed", "path", bakPath, "err", err)
		return
	}
	if err := filesystem.Rename(tempName, bakPath); err != nil {
		cleanup()
		slog.Warn("config backup rename failed", "path", bakPath, "err", err)
		return
	}
	slog.Debug("config backed up", "from", filePath, "to", bakPath)
}
