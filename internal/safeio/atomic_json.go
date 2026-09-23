package safeio

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
)

// defaultNewFileMode is the permission given to files WriteJSONFile
// creates when no previous file exists to copy the mode from.
const defaultNewFileMode os.FileMode = 0644

// WriteFileAtomic replaces filePath with data atomically:
//
//  1. Create a unique temp file in the same dir as filePath
//     (tempPattern is an afero.TempFile pattern such as ".settings-*.tmp").
//  2. Write, Sync, Close.
//  3. Chmod the temp file to mode.
//  4. Atomic Rename onto filePath.
//  5. Parent-dir fsync on OsFs so the rename entry survives a crash
//     (no-op on Windows; skipped on non-OsFs such as MemMapFs).
//
// The parent directory must already exist. Any failure leaves filePath
// untouched and removes the temp file.
func WriteFileAtomic(filesystem afero.Fs, filePath string, data []byte, mode os.FileMode, tempPattern string) error {
	dir := filepath.Dir(filePath)

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

	if err := filesystem.Chmod(tempName, mode); err != nil {
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
			slog.Warn("parent dir fsync failed (non-fatal)", "dir", dir, "error", err)
		}
	}

	slog.Debug("file written atomically", "path", filePath)
	return nil
}

// WriteJSONFile marshals v as indented JSON and replaces filePath with it
// via WriteFileAtomic. It creates the parent directory if needed and
// preserves the existing file's permission bits (0644 for a new file).
//
// Callers own the discipline layered on top: advisory locking, .bak
// backups and pre-write validation all belong above this function.
func WriteJSONFile(filesystem afero.Fs, filePath string, v any, tempPattern string) error {
	dir := filepath.Dir(filePath)
	if err := filesystem.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	fileMode := defaultNewFileMode
	if existingInfo, err := filesystem.Stat(filePath); err == nil {
		fileMode = existingInfo.Mode() & os.ModePerm
	}

	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON for %s: %w", filePath, err)
	}

	return WriteFileAtomic(filesystem, filePath, data, fileMode, tempPattern)
}

// BackupJSONFile copies filePath to filePath+".bak" iff filePath exists
// and unmarshals into probe (a pointer to the caller's expected shape).
// The copy keeps the source's permission bits and goes through
// WriteFileAtomic so a crash mid-write cannot corrupt the recovery file.
//
// Failure is logged at WARN but not returned: a missing backup is better
// than a blocked write, and refusing to overwrite a valid .bak with a
// corrupt source preserves the last-known-good copy.
func BackupJSONFile(filesystem afero.Fs, filePath string, probe any, tempPattern string) {
	info, err := filesystem.Stat(filePath)
	if err != nil {
		return // no file, no backup needed
	}
	data, err := afero.ReadFile(filesystem, filePath)
	if err != nil {
		slog.Warn("backup skipped: read failed", "path", filePath, "error", err)
		return
	}
	if err := json.Unmarshal(data, probe); err != nil {
		slog.Warn("backup skipped: existing file is not valid JSON, refusing to overwrite .bak",
			"path", filePath, "error", err)
		return
	}

	bakPath := filePath + ".bak"
	if err := WriteFileAtomic(filesystem, bakPath, data, info.Mode()&os.ModePerm, tempPattern); err != nil {
		slog.Warn("backup skipped: atomic write failed", "path", bakPath, "error", err)
		return
	}
	slog.Debug("file backed up", "from", filePath, "to", bakPath)
}
