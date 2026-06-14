package cli

import (
	"fmt"
	"io"
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
	Type       string // "embedded", "json", "directory"
	SoundCount int
	Path       string
}

var embeddedPlatformSoundpackFiles = []string{"windows.json", "wsl.json", "darwin.json", "linux.json"}

func embeddedPlatformSoundpackIdentifier(name string) (string, bool) {
	if name == "" || strings.ContainsAny(name, `/\`) || filepath.Ext(name) != "" {
		return "", false
	}

	filename := name + ".json"
	for _, embedded := range embeddedPlatformSoundpackFiles {
		if filename == embedded {
			return "embedded:" + filename, true
		}
	}

	if _, err := config.GetEmbeddedPlatformSoundpackData(filename); err == nil {
		return "embedded:" + filename, true
	}

	return "", false
}

// discoverSoundpacks finds all available soundpacks from embedded, XDG, and config sources.
// Returns a deduplicated list of soundpack info structs.
func discoverSoundpacks() ([]soundpackInfo, error) {
	slog.Debug("discovering soundpacks")

	var packs []soundpackInfo
	seen := make(map[string]struct{}) // Deduplicate by name+path

	// 1. Embedded platform packs (always present)
	embeddedPacks, err := discoverEmbeddedSoundpacks()
	if err != nil {
		slog.Warn("failed to discover embedded soundpacks", "error", err)
	} else {
		for _, p := range embeddedPacks {
			key := p.Name + "|" + p.Path
			if _, exists := seen[key]; !exists {
				seen[key] = struct{}{}
				packs = append(packs, p)
			}
		}
	}

	// 2. XDG data directory packs
	xdgPacks := discoverXDGSoundpacks()
	for _, p := range xdgPacks {
		key := p.Name + "|" + p.Path
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			packs = append(packs, p)
		}
	}

	// 3. Managed git soundpacks
	gitPacks := discoverManagedGitSoundpacks()
	for _, p := range gitPacks {
		key := p.Name + "|" + p.Path
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			packs = append(packs, p)
		}
	}

	// 4. Config soundpack_paths entries
	configPacks := discoverConfigSoundpacks()
	for _, p := range configPacks {
		key := p.Name + "|" + p.Path
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			packs = append(packs, p)
		}
	}

	slog.Info("total soundpacks discovered", "count", len(packs))
	return packs, nil
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
			Type:       "embedded",
			SoundCount: soundCount,
			Path:       "(built-in)",
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
				name := strings.TrimSuffix(entry.Name(), ".json")
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
			name := strings.TrimSuffix(entry.Name(), ".json")
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

// discoverConfigSoundpacks checks paths from config soundpack_paths
func discoverConfigSoundpacks() []soundpackInfo {
	cm := config.NewConfigManager()
	cfg, err := cm.LoadConfig()
	if err != nil {
		slog.Debug("could not load config for soundpack path discovery", "error", err)
		return nil
	}

	var packs []soundpackInfo

	for _, path := range cfg.SoundpackPaths {
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
			name := strings.TrimSuffix(filepath.Base(path), ".json")
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
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".wav" || ext == ".mp3" || ext == ".aiff" {
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

// copyFile copies a single file from src to dst, creating parent directories as needed.
func copyFile(src, dst string) error {
	slog.Debug("copying file", "src", src, "dst", dst)

	// Create destination directory
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
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy data: %w", err)
	}

	return nil
}

// copyDirectory recursively copies a directory from src to dst.
func copyDirectory(src, dst string) error {
	slog.Debug("copying directory", "src", src, "dst", dst)

	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("failed to calculate relative path: %w", relErr)
		}

		dstPath := filepath.Join(dst, rel)

		if info.IsDir() {
			slog.Debug("creating directory", "path", dstPath)
			return os.MkdirAll(dstPath, 0755)
		}

		slog.Debug("copying file in directory", "src", path, "dst", dstPath)
		return copyFile(path, dstPath)
	})
}
