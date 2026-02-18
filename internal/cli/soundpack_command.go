package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"claudio.click/internal/audio"
	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
	"github.com/spf13/cobra"
)

// newSoundpackCommand creates the soundpack command group
func newSoundpackCommand() *cobra.Command {
	soundpackCmd := &cobra.Command{
		Use:   "soundpack",
		Short: "Manage soundpacks",
		Long:  "Commands for creating, validating, and managing soundpacks",
	}
	soundpackCmd.AddCommand(newSoundpackInitCommand())
	soundpackCmd.AddCommand(newSoundpackListCommand())
	return soundpackCmd
}

// newSoundpackInitCommand creates the soundpack init subcommand
func newSoundpackInitCommand() *cobra.Command {
	var dir string
	var fromPlatform bool

	initCmd := &cobra.Command{
		Use:   "init <name>",
		Short: "Create a new soundpack template",
		Long: `Create a new soundpack JSON template with all known sound mapping keys.

All mapping values default to empty strings. Use --from-platform to pre-fill
values from the current platform's embedded soundpack.

Examples:
  claudio soundpack init my-pack
  claudio soundpack init my-pack --dir /path/to/output
  claudio soundpack init my-pack --from-platform`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackInit(cmd, args[0], dir, fromPlatform)
		},
	}

	initCmd.Flags().StringVar(&dir, "dir", ".", "Output directory for the soundpack file")
	initCmd.Flags().BoolVar(&fromPlatform, "from-platform", false, "Pre-fill values from current platform's sounds")

	return initCmd
}

// runSoundpackInit executes the soundpack init command
func runSoundpackInit(cmd *cobra.Command, name, dir string, fromPlatform bool) error {
	slog.Debug("running soundpack init", "name", name, "dir", dir, "from_platform", fromPlatform)

	// Determine output path
	outputPath := filepath.Join(dir, name+".json")

	// Overwrite protection: fail if file already exists
	if _, err := os.Stat(outputPath); err == nil {
		slog.Error("target file already exists", "path", outputPath)
		return fmt.Errorf("file already exists: %s", outputPath)
	}

	// Extract all known sound keys from embedded platform JSONs
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		slog.Error("failed to extract sound keys", "error", err)
		return fmt.Errorf("failed to extract sound keys: %w", err)
	}
	slog.Info("extracted sound keys", "count", len(keys))

	// Build mappings
	mappings := make(map[string]string, len(keys))
	for _, key := range keys {
		mappings[key] = ""
	}

	// If --from-platform, pre-fill from current platform's embedded soundpack
	if fromPlatform {
		platformFile := detectPlatformFile()
		slog.Debug("loading platform soundpack for pre-fill", "platform_file", platformFile)

		data, err := config.GetEmbeddedPlatformSoundpackData(platformFile)
		if err != nil {
			slog.Warn("failed to load platform soundpack for pre-fill", "file", platformFile, "error", err)
		} else {
			var platformSP soundpack.JSONSoundpackFile
			if err := json.Unmarshal(data, &platformSP); err != nil {
				slog.Warn("failed to parse platform soundpack for pre-fill", "file", platformFile, "error", err)
			} else {
				for key, val := range platformSP.Mappings {
					if _, exists := mappings[key]; exists {
						mappings[key] = val
					}
				}
				slog.Info("pre-filled mappings from platform soundpack", "platform_file", platformFile)
			}
		}
	}

	// Build the output struct
	spFile := soundpack.JSONSoundpackFile{
		Name:        name,
		Description: "Custom soundpack",
		Version:     "1.0.0",
		Mappings:    mappings,
	}

	// Marshal to JSON with sorted keys (Go's json.Marshal sorts map keys by default)
	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		slog.Error("failed to marshal JSON", "error", err)
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Error("failed to create output directory", "dir", dir, "error", err)
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		slog.Error("failed to write file", "path", outputPath, "error", err)
		return fmt.Errorf("failed to write file: %w", err)
	}

	slog.Info("soundpack template created", "path", outputPath, "keys", len(keys))
	cmd.Printf("Created soundpack template: %s (%d sound keys)\n", outputPath, len(keys))

	return nil
}

// soundpackInfo holds metadata about a discovered soundpack
type soundpackInfo struct {
	Name       string
	Type       string // "embedded", "json", "directory"
	SoundCount int
	Path       string
}

// newSoundpackListCommand creates the soundpack list subcommand
func newSoundpackListCommand() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all discoverable soundpacks",
		Long: `List all soundpacks from embedded platform packs, XDG data directories,
and config soundpack_paths.

Shows name, type (embedded/json/directory), sound count, and path for each pack.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackList(cmd)
		},
	}
	return listCmd
}

// runSoundpackList executes the soundpack list command
func runSoundpackList(cmd *cobra.Command) error {
	slog.Debug("running soundpack list")

	packs, err := discoverSoundpacks()
	if err != nil {
		slog.Error("failed to discover soundpacks", "error", err)
		return fmt.Errorf("failed to discover soundpacks: %w", err)
	}

	slog.Info("discovered soundpacks", "count", len(packs))

	// Calculate column widths for tabular formatting
	nameWidth := len("NAME")
	typeWidth := len("TYPE")
	soundsWidth := len("SOUNDS")
	for _, p := range packs {
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
		if len(p.Type) > typeWidth {
			typeWidth = len(p.Type)
		}
		countStr := fmt.Sprintf("%d", p.SoundCount)
		if len(countStr) > soundsWidth {
			soundsWidth = len(countStr)
		}
	}

	// Add padding
	nameWidth += 2
	typeWidth += 2
	soundsWidth += 2

	// Print header
	format := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%s\n", nameWidth, typeWidth, soundsWidth)
	cmd.Printf(format, "NAME", "TYPE", "SOUNDS", "PATH")

	// Print rows
	for _, p := range packs {
		cmd.Printf(format, p.Name, p.Type, fmt.Sprintf("%d", p.SoundCount), p.Path)
	}

	return nil
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

	// 3. Config soundpack_paths entries
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

// discoverEmbeddedSoundpacks returns info for the 3 embedded platform packs
func discoverEmbeddedSoundpacks() ([]soundpackInfo, error) {
	platformFiles := []string{"windows.json", "wsl.json", "darwin.json"}
	var packs []soundpackInfo

	for _, file := range platformFiles {
		data, err := config.GetEmbeddedPlatformSoundpackData(file)
		if err != nil {
			slog.Warn("failed to read embedded platform soundpack", "file", file, "error", err)
			continue
		}

		var spFile soundpack.JSONSoundpackFile
		if err := json.Unmarshal(data, &spFile); err != nil {
			slog.Warn("failed to parse embedded platform soundpack", "file", file, "error", err)
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
				// JSON soundpack file
				data, err := os.ReadFile(fullPath)
				if err != nil {
					slog.Debug("could not read JSON soundpack file", "path", fullPath, "error", err)
					continue
				}
				var spFile soundpack.JSONSoundpackFile
				if err := json.Unmarshal(data, &spFile); err != nil {
					slog.Debug("could not parse JSON soundpack file", "path", fullPath, "error", err)
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
			data, err := os.ReadFile(fullPath)
			if err != nil {
				continue
			}
			var spFile soundpack.JSONSoundpackFile
			if err := json.Unmarshal(data, &spFile); err != nil {
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
			data, err := os.ReadFile(path)
			if err != nil {
				slog.Debug("could not read config JSON soundpack", "path", path, "error", err)
				continue
			}
			var spFile soundpack.JSONSoundpackFile
			if err := json.Unmarshal(data, &spFile); err != nil {
				slog.Debug("could not parse config JSON soundpack", "path", path, "error", err)
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
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
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

// ExtractAllSoundKeys reads all 3 embedded platform JSONs and returns the sorted
// union of all mapping keys. It uses raw json.Unmarshal (not LoadJSONSoundpack)
// to avoid file-existence validation.
func ExtractAllSoundKeys() ([]string, error) {
	platformFiles := []string{"windows.json", "wsl.json", "darwin.json"}
	keySet := make(map[string]struct{})

	for _, file := range platformFiles {
		data, err := config.GetEmbeddedPlatformSoundpackData(file)
		if err != nil {
			slog.Warn("failed to read embedded platform soundpack", "file", file, "error", err)
			continue
		}

		var spFile soundpack.JSONSoundpackFile
		if err := json.Unmarshal(data, &spFile); err != nil {
			slog.Warn("failed to parse embedded platform soundpack", "file", file, "error", err)
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
	if audio.IsWSL() {
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
