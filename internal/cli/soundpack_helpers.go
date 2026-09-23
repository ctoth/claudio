package cli

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"claudio.click/internal/config"
	"claudio.click/internal/platform"
	"claudio.click/internal/soundpack"
)

// soundpackInfo holds metadata about a discovered soundpack. Used by both
// the list subcommand and the soundpack_git.go discovery paths.
type soundpackInfo struct {
	Name       string
	Type       string // "embedded", "git", "json", "directory"
	SoundCount int
	Path       string
	Identifier string // "embedded:<file>" for embedded packs, empty otherwise
}

const soundpackTypeEmbedded = "embedded"

var embeddedPlatformSoundpackFiles = []string{"windows.json", "wsl.json", "darwin.json", "linux.json"}

// discoverSoundpacks lists every soundpack reachable by name, using the
// soundpack_paths of the effective XDG config. See
// discoverSoundpacksWithPaths for the order.
func discoverSoundpacks() ([]soundpackInfo, error) {
	return discoverSoundpacksWithPaths(configuredSoundpackPaths()), nil
}

// discoverSoundpacksWithPaths lists every soundpack reachable by name, in
// resolution precedence order: embedded platform packs, managed git packs,
// XDG data directory packs, then configPaths (config soundpack_paths).
// Entries with the same name and path are listed once. `soundpack list`
// prints this list, `soundpack use` accepts its names, and the runtime
// resolves a name to the first entry with that name (lookupSoundpack), so
// all three agree.
func discoverSoundpacksWithPaths(configPaths []string) []soundpackInfo {
	slog.Debug("discovering soundpacks")

	embeddedPacks, err := discoverEmbeddedSoundpacks()
	if err != nil {
		slog.Warn("failed to discover embedded soundpacks", "error", err)
	}

	var packs []soundpackInfo
	seen := make(map[string]struct{}) // Deduplicate by name+path
	for _, source := range [][]soundpackInfo{
		embeddedPacks,
		discoverManagedGitSoundpacks(),
		discoverXDGSoundpacks(),
		discoverConfigSoundpacks(configPaths),
	} {
		for _, p := range source {
			key := p.Name + "|" + p.Path
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				packs = append(packs, p)
			}
		}
	}

	slog.Info("total soundpacks discovered", "count", len(packs))
	return packs
}

// lookupSoundpack returns the soundpack a name resolves to: the first
// entry named name in discoverSoundpacksWithPaths(configPaths).
func lookupSoundpack(name string, configPaths []string) (soundpackInfo, bool) {
	if name == "" {
		return soundpackInfo{}, false
	}
	for _, p := range discoverSoundpacksWithPaths(configPaths) {
		if p.Name == name {
			return p, true
		}
	}
	return soundpackInfo{}, false
}

// configuredSoundpackPaths returns soundpack_paths from the effective XDG
// config, or nil when it cannot be loaded.
func configuredSoundpackPaths() []string {
	cfg, err := config.NewConfigManager().LoadConfig()
	if err != nil {
		slog.Debug("could not load config for soundpack path discovery", "error", err)
		return nil
	}
	return cfg.SoundpackPaths
}

// discoverEmbeddedSoundpacks returns info for embedded platform packs.
func discoverEmbeddedSoundpacks() ([]soundpackInfo, error) {
	var packs []soundpackInfo

	for _, file := range embeddedPlatformSoundpackFiles {
		data, err := config.GetEmbeddedPlatformSoundpackData(file)
		if err != nil {
			slog.Warn("failed to read embedded platform soundpack", "file", file, "error", err)
			continue
		}

		spFile, peekErr := soundpack.PeekJSONSoundpackFromBytes(data)
		if peekErr != nil {
			slog.Warn("failed to parse embedded platform soundpack", "file", file, "error", peekErr)
			continue
		}

		// Count non-empty mapping values
		soundCount := 0
		for _, val := range spFile.Mappings {
			if val != "" {
				soundCount++
			}
		}

		name := strings.TrimSuffix(file, ".json")
		slog.Debug("discovered embedded soundpack", "name", name, "sounds", soundCount)

		packs = append(packs, soundpackInfo{
			Name:       name,
			Type:       soundpackTypeEmbedded,
			SoundCount: soundCount,
			Path:       "(built-in)",
			Identifier: "embedded:" + file,
		})
	}

	if len(packs) == 0 {
		return nil, fmt.Errorf("no embedded platform soundpacks found")
	}

	return packs, nil
}

// discoverXDGSoundpacks scans XDG data directories for installed soundpacks
func discoverXDGSoundpacks() []soundpackInfo {
	xdg := config.NewXDGDirs()
	// GetSoundpackPaths("") returns the base soundpack directories
	basePaths := xdg.GetSoundpackPaths("")

	var packs []soundpackInfo

	for _, basePath := range basePaths {
		slog.Debug("scanning XDG soundpack directory", "path", basePath)

		entries, err := os.ReadDir(basePath)
		if err != nil {
			slog.Debug("could not read XDG soundpack directory", "path", basePath, "error", err)
			continue
		}

		for _, entry := range entries {
			fullPath := filepath.Join(basePath, entry.Name())

			if entry.IsDir() {
				manifestPath := filepath.Join(fullPath, "soundpack.json")
				if spFile, peekErr := soundpack.PeekJSONSoundpackFromFile(manifestPath); peekErr == nil && spFile.Name != "" {
					packs = append(packs, soundpackInfo{
						Name:       spFile.Name,
						Type:       "json",
						SoundCount: countNonEmptyMappings(spFile.Mappings),
						Path:       manifestPath,
					})
					continue
				}
				// Directory soundpack - count audio files
				count := countAudioFiles(fullPath)
				slog.Debug("discovered directory soundpack", "name", entry.Name(), "path", fullPath, "sounds", count)
				packs = append(packs, soundpackInfo{
					Name:       entry.Name(),
					Type:       "directory",
					SoundCount: count,
					Path:       fullPath,
				})
			} else if strings.HasSuffix(entry.Name(), ".json") {
				// JSON soundpack file — peek (apply size cap and basic
				// validation) so malformed or oversized files are
				// skipped rather than blowing up discovery.
				spFile, peekErr := soundpack.PeekJSONSoundpackFromFile(fullPath)
				if peekErr != nil {
					slog.Debug("could not peek JSON soundpack file", "path", fullPath, "error", peekErr)
					continue
				}
				soundCount := countNonEmptyMappings(spFile.Mappings)
				name := spFile.Name
				if name == "" {
					name = strings.TrimSuffix(entry.Name(), ".json")
				}
				slog.Debug("discovered JSON soundpack", "name", name, "path", fullPath, "sounds", soundCount)
				packs = append(packs, soundpackInfo{
					Name:       name,
					Type:       "json",
					SoundCount: soundCount,
					Path:       fullPath,
				})
			}
		}
	}

	// Also check the parent claudio data directory for JSON files
	parentPaths := xdg.GetSoundpackPaths("")
	for _, basePath := range parentPaths {
		parentDir := filepath.Dir(basePath) // claudio/ directory
		slog.Debug("scanning parent claudio directory for JSON soundpacks", "path", parentDir)

		entries, err := os.ReadDir(parentDir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			fullPath := filepath.Join(parentDir, entry.Name())
			spFile, peekErr := soundpack.PeekJSONSoundpackFromFile(fullPath)
			if peekErr != nil {
				continue
			}
			soundCount := countNonEmptyMappings(spFile.Mappings)
			name := spFile.Name
			if name == "" {
				name = strings.TrimSuffix(entry.Name(), ".json")
			}
			slog.Debug("discovered JSON soundpack in parent dir", "name", name, "path", fullPath, "sounds", soundCount)
			packs = append(packs, soundpackInfo{
				Name:       name,
				Type:       "json",
				SoundCount: soundCount,
				Path:       fullPath,
			})
		}
	}

	return packs
}

// discoverConfigSoundpacks lists the packs at config soundpack_paths entries.
func discoverConfigSoundpacks(configPaths []string) []soundpackInfo {
	var packs []soundpackInfo

	for _, path := range configPaths {
		slog.Debug("checking config soundpack_path", "path", path)

		info, err := os.Stat(path)
		if err != nil {
			slog.Debug("config soundpack_path not accessible", "path", path, "error", err)
			continue
		}

		if info.IsDir() {
			count := countAudioFiles(path)
			name := filepath.Base(path)
			slog.Debug("discovered directory soundpack from config", "name", name, "path", path, "sounds", count)
			packs = append(packs, soundpackInfo{
				Name:       name,
				Type:       "directory",
				SoundCount: count,
				Path:       path,
			})
		} else if strings.HasSuffix(path, ".json") {
			spFile, peekErr := soundpack.PeekJSONSoundpackFromFile(path)
			if peekErr != nil {
				slog.Debug("could not peek config JSON soundpack", "path", path, "error", peekErr)
				continue
			}
			soundCount := countNonEmptyMappings(spFile.Mappings)
			name := spFile.Name
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), ".json")
			}
			slog.Debug("discovered JSON soundpack from config", "name", name, "path", path, "sounds", soundCount)
			packs = append(packs, soundpackInfo{
				Name:       name,
				Type:       "json",
				SoundCount: soundCount,
				Path:       path,
			})
		}
	}

	return packs
}

// countAudioFiles recursively counts audio files in a directory
func countAudioFiles(dir string) int {
	count := 0
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}
		if info.IsDir() {
			return nil
		}
		if soundpack.IsAudioExt(filepath.Ext(path)) {
			count++
		}
		return nil
	})
	return count
}

// countNonEmptyMappings counts how many mapping values are non-empty
func countNonEmptyMappings(mappings map[string]string) int {
	count := 0
	for _, val := range mappings {
		if val != "" {
			count++
		}
	}
	return count
}

// ExtractAllSoundKeys reads all embedded platform JSONs and returns the sorted
// union of all mapping keys. It uses PeekJSONSoundpackFromBytes (which
// applies the size and mappings-count caps but skips path/existence
// checks) since we only need the keys.
func ExtractAllSoundKeys() ([]string, error) {
	keySet := make(map[string]struct{})

	for _, file := range embeddedPlatformSoundpackFiles {
		data, err := config.GetEmbeddedPlatformSoundpackData(file)
		if err != nil {
			slog.Warn("failed to read embedded platform soundpack", "file", file, "error", err)
			continue
		}

		spFile, peekErr := soundpack.PeekJSONSoundpackFromBytes(data)
		if peekErr != nil {
			slog.Warn("failed to parse embedded platform soundpack", "file", file, "error", peekErr)
			continue
		}

		for key := range spFile.Mappings {
			keySet[key] = struct{}{}
		}

		slog.Debug("extracted keys from platform file", "file", file, "keys", len(spFile.Mappings))
	}

	if len(keySet) == 0 {
		return nil, fmt.Errorf("no sound keys found in any embedded platform soundpack")
	}

	// Convert to sorted slice
	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	slog.Info("extracted all sound keys", "total_unique", len(keys))
	return keys, nil
}

// detectPlatformFile returns the platform-specific embedded JSON filename
func detectPlatformFile() string {
	if platform.IsWSL() {
		return "wsl.json"
	}
	platformFile := runtime.GOOS + ".json"
	// Verify it exists as an embedded file; fall back to windows.json
	if _, err := config.GetEmbeddedPlatformSoundpackData(platformFile); err != nil {
		slog.Debug("platform file not found, falling back to windows.json", "tried", platformFile)
		return "windows.json"
	}
	return platformFile
}

// categoryFromKey extracts the category from a sound key.
// Keys like "loading/bash-start.wav" -> "loading"
// Keys like "default.wav" -> "default"
func categoryFromKey(key string) string {
	if idx := strings.Index(key, "/"); idx >= 0 {
		return key[:idx]
	}
	// Root-level keys like "default.wav"
	return strings.TrimSuffix(key, filepath.Ext(key))
}

// copyFile copies the regular file src to dst, creating parent directories
// as needed. src must not be a symlink: soundpack sources are untrusted and
// a link could pull in a file from outside the pack.
func copyFile(src, dst string) (err error) {
	slog.Debug("copying file", "src", src, "dst", dst)

	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("failed to inspect source: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to copy symlink: %s", src)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("refusing to copy non-regular file: %s", src)
	}

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dstDir, err)
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close destination: %w", closeErr)
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// copyDirectory recursively copies the directory src to dst. A symlinked
// src root is resolved first; any symlink inside the tree is an error, and
// .git directories are skipped.
func copyDirectory(src, dst string) error {
	slog.Debug("copying directory", "src", src, "dst", dst)

	root, err := filepath.EvalSymlinks(src)
	if err != nil {
		return fmt.Errorf("failed to resolve source directory: %w", err)
	}

	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fmt.Errorf("failed to calculate relative path: %w", relErr)
		}
		dstPath := filepath.Join(dst, rel)

		switch {
		case d.Type()&fs.ModeSymlink != 0:
			return fmt.Errorf("soundpack contains a symlink, which is not allowed: %s", path)
		case d.IsDir() && path != root && d.Name() == ".git":
			slog.Debug("skipping VCS metadata directory", "path", path)
			return filepath.SkipDir
		case d.IsDir():
			slog.Debug("creating directory", "path", dstPath)
			return os.MkdirAll(dstPath, 0755)
		default:
			slog.Debug("copying file in directory", "src", path, "dst", dstPath)
			return copyFile(path, dstPath)
		}
	})
}
