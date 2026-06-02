package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// WriteConfigFile atomically writes cfg to filePath. Pattern mirrors
// internal/install/settings_io.go (WriteSettingsFile) byte-for-byte:
//
//  1. MkdirAll the parent dir.
//  2. BackupConfigFile (non-fatal, refuses to overwrite a valid .bak
//     with a corrupt source).
//  3. Probe existing mode (default 0644 for new files).
//  4. Marshal config with indent.
//  5. Create unique temp file in the same dir, Write, Sync, Close,
//     Chmod, atomic Rename.
//  6. Parent-dir fsync on OsFs (no-op on Windows; skipped on non-OsFs
//     like MemMapFs).
//
// Callers MUST hold LockConfigDir(filePath) for the entire
// read-mutate-write window.
//
// NOTE: This duplicates internal/install/settings_io.go's WriteSettingsFile
// to avoid a layering inversion (config currently does not depend on
// install). A future refactor can lift both into a shared
// internal/safeio/atomicjson package; doing it here would expand this
// chunk's scope.
func WriteConfigFile(filesystem afero.Fs, filePath string, cfg *Config) error {
	// 1. MkdirAll
	dir := filepath.Dir(filePath)
	if err := filesystem.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	// 2. Back up existing config before overwrite. Non-fatal: a missing
	// .bak is better than a blocked write.
	BackupConfigFile(filesystem, filePath)

	// 3. Detect existing file permissions to preserve them.
	fileMode := os.FileMode(0644)
	if existingInfo, err := filesystem.Stat(filePath); err == nil {
		fileMode = existingInfo.Mode() & os.ModePerm
	}

	// 4. Marshal config to JSON with indentation for human readability.
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config to JSON: %w", err)
	}

	// 5. Write atomically using a unique temp file + rename.
	tempFile, err := afero.TempFile(filesystem, dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	tempName := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to write temp config file: %w", err)
	}

	// Sync temp file to disk before close so a crash between rename and
	// full durability does not leave the new content unflushed.
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to sync temp config file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to close temp config file: %w", err)
	}

	if err := filesystem.Chmod(tempName, fileMode); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to set temp config file permissions: %w", err)
	}

	if err := filesystem.Rename(tempName, filePath); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to rename temp config file: %w", err)
	}

	// 6. Parent-dir fsync (OsFs only; skipped on Windows / MemMapFs).
	if _, isOs := filesystem.(*afero.OsFs); isOs {
		if err := fsyncDir(dir); err != nil {
			slog.Warn("config parent dir fsync failed (non-fatal)", "dir", dir, "err", err)
		}
	}

	slog.Debug("config written atomically", "path", filePath)
	return nil
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
