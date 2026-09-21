package safeio

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// WriteJSONFile marshals v as indented JSON and replaces filePath with it
// atomically:
//
//  1. MkdirAll the parent dir.
//  2. Probe the existing file's mode (0644 for a new file).
//  3. Marshal v with indentation.
//  4. Create a unique temp file in the same dir (tempPattern is an
//     afero.TempFile pattern such as ".settings-*.tmp"), Write, Sync,
//     Close, Chmod, atomic Rename.
//  5. Parent-dir fsync on OsFs so the rename entry survives a crash
//     (no-op on Windows; skipped on non-OsFs such as MemMapFs).
//
// Any failure leaves filePath untouched and removes the temp file.
// Callers own the discipline layered on top: advisory locking, .bak
// backups and pre-write validation all belong above this function.
func WriteJSONFile(filesystem afero.Fs, filePath string, v any, tempPattern string) error {
	dir := filepath.Dir(filePath)
	if err := filesystem.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Detect existing file permissions to preserve them.
	fileMode := os.FileMode(0644) // Default for new files
	if existingInfo, err := filesystem.Stat(filePath); err == nil {
		fileMode = existingInfo.Mode() & os.ModePerm
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for %s: %w", filePath, err)
	}

	tempFile, err := afero.TempFile(filesystem, dir, tempPattern)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempName := tempFile.Name()

	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync the temp file before close so a crash between rename and full
	// durability doesn't leave the new content unflushed. afero.File
	// declares Sync() on the interface — MemMapFs no-ops, OsFs delegates
	// to os.File.Sync (the real fsync(2) syscall).
	if err := tempFile.Sync(); err != nil {
		tempFile.Close()
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tempFile.Close(); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := filesystem.Chmod(tempName, fileMode); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	if err := filesystem.Rename(tempName, filePath); err != nil {
		_ = filesystem.Remove(tempName)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// fsync the parent directory so the rename entry survives a crash on
	// POSIX filesystems. Skipped on Windows (no portable directory flush)
	// and on non-OS filesystems (MemMapFs has no real directory to sync).
	// Failure here is non-fatal — the rename already succeeded.
	if _, isOs := filesystem.(*afero.OsFs); isOs {
		if err := fsyncDir(dir); err != nil {
			slog.Warn("parent dir fsync failed (non-fatal)", "dir", dir, "err", err)
		}
	}

	slog.Debug("file written atomically", "path", filePath)
	return nil
}
