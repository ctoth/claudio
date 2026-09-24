package config

import (
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
// parses as a Config. See safeio.BackupJSONFile for the failure policy:
// errors are logged, never returned, and a corrupt source never
// overwrites the last-known-good .bak.
func BackupConfigFile(filesystem afero.Fs, filePath string) {
	safeio.BackupJSONFile(filesystem, filePath, &Config{}, ".config-bak-*.tmp")
}
