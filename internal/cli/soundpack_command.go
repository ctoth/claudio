package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"claudio.click/internal/audio"
	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
	"github.com/adrg/xdg"
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
	soundpackCmd.AddCommand(newSoundpackValidateCommand())
	soundpackCmd.AddCommand(newSoundpackInstallCommand())
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

// newSoundpackValidateCommand creates the soundpack validate subcommand
func newSoundpackValidateCommand() *cobra.Command {
	validateCmd := &cobra.Command{
		Use:   "validate <path>",
		Short: "Validate a soundpack and show coverage report",
		Long: `Validate a JSON soundpack file or directory soundpack.

Checks:
  1. JSON structure: valid JSON that parses into a soundpack
  2. Referenced files exist: non-empty mappings point to real files
  3. Coverage gaps: compare mappings against all known sound keys
  4. Format check: referenced files should be .wav, .mp3, or .aiff

Exit code 0 if no broken references, non-zero if broken references found.
Empty mappings are informational, not errors.

Examples:
  claudio soundpack validate my-pack.json
  claudio soundpack validate /path/to/soundpack-dir`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackValidate(cmd, args[0])
		},
	}
	return validateCmd
}

// validateResult holds the results of soundpack validation
type validateResult struct {
	Name             string
	Version          string
	Mappings         map[string]string // all mappings from the soundpack
	AllKeys          []string          // all known keys
	MappedKeys       map[string]string // keys with non-empty values
	BrokenRefs       map[string]string // key -> path for files that don't exist
	FormatWarnings   map[string]string // key -> path for files with non-audio extensions
	IsDirectory      bool
}

// runSoundpackValidate executes the soundpack validate command
func runSoundpackValidate(cmd *cobra.Command, path string) error {
	slog.Debug("running soundpack validate", "path", path)

	// Determine if path is a directory or JSON file
	info, err := os.Stat(path)
	if err != nil {
		slog.Error("cannot access path", "path", path, "error", err)
		return fmt.Errorf("cannot access path: %w", err)
	}

	var result validateResult
	if info.IsDir() {
		result, err = validateDirectorySoundpack(path)
	} else {
		result, err = validateJSONSoundpackFile(path)
	}
	if err != nil {
		return err
	}

	// Print the validation report
	printValidateReport(cmd, result)

	// Exit with non-zero if there are broken references
	if len(result.BrokenRefs) > 0 {
		return fmt.Errorf("validation failed: %d broken reference(s)", len(result.BrokenRefs))
	}

	return nil
}

// validateJSONSoundpackFile validates a JSON soundpack file
func validateJSONSoundpackFile(path string) (validateResult, error) {
	slog.Debug("validating JSON soundpack", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("failed to read file", "path", path, "error", err)
		return validateResult{}, fmt.Errorf("failed to read file: %w", err)
	}

	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
		slog.Error("failed to parse JSON", "path", path, "error", err)
		return validateResult{}, fmt.Errorf("failed to parse JSON: %w", err)
	}

	slog.Info("parsed JSON soundpack", "name", spFile.Name, "mappings", len(spFile.Mappings))

	// Get all known keys
	allKeys, err := ExtractAllSoundKeys()
	if err != nil {
		slog.Error("failed to extract all sound keys", "error", err)
		return validateResult{}, fmt.Errorf("failed to extract sound keys: %w", err)
	}

	// Identify mapped (non-empty) keys, broken refs, and format warnings
	mappedKeys := make(map[string]string)
	brokenRefs := make(map[string]string)
	formatWarnings := make(map[string]string)

	for key, val := range spFile.Mappings {
		if val == "" {
			continue
		}
		mappedKeys[key] = val

		// Check if file exists
		if _, statErr := os.Stat(val); statErr != nil {
			slog.Warn("broken reference", "key", key, "path", val)
			brokenRefs[key] = val
		} else {
			// Check file format
			ext := strings.ToLower(filepath.Ext(val))
			if ext != ".wav" && ext != ".mp3" && ext != ".aiff" {
				slog.Warn("non-audio format", "key", key, "path", val, "ext", ext)
				formatWarnings[key] = val
			}
		}
	}

	return validateResult{
		Name:           spFile.Name,
		Version:        spFile.Version,
		Mappings:       spFile.Mappings,
		AllKeys:        allKeys,
		MappedKeys:     mappedKeys,
		BrokenRefs:     brokenRefs,
		FormatWarnings: formatWarnings,
		IsDirectory:    false,
	}, nil
}

// validateDirectorySoundpack validates a directory-based soundpack
func validateDirectorySoundpack(dirPath string) (validateResult, error) {
	slog.Debug("validating directory soundpack", "path", dirPath)

	// Get all known keys
	allKeys, err := ExtractAllSoundKeys()
	if err != nil {
		slog.Error("failed to extract all sound keys", "error", err)
		return validateResult{}, fmt.Errorf("failed to extract sound keys: %w", err)
	}

	// Scan directory for audio files and map them to known keys
	// Key pattern: <category>/<filename> (e.g., loading/bash-start.wav)
	mappedKeys := make(map[string]string)
	allMappings := make(map[string]string)

	// Initialize all known keys as empty
	for _, key := range allKeys {
		allMappings[key] = ""
	}

	// Walk directory to find audio files
	filepath.Walk(dirPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".wav" && ext != ".mp3" && ext != ".aiff" {
			return nil
		}

		// Build the key from relative path components
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return nil
		}
		// Normalize to forward slashes for key matching
		key := filepath.ToSlash(rel)

		slog.Debug("found audio file in directory", "key", key, "path", path)

		allMappings[key] = path
		mappedKeys[key] = path

		return nil
	})

	name := filepath.Base(dirPath)
	slog.Info("scanned directory soundpack", "name", name, "found_files", len(mappedKeys))

	return validateResult{
		Name:           name,
		Version:        "",
		Mappings:       allMappings,
		AllKeys:        allKeys,
		MappedKeys:     mappedKeys,
		BrokenRefs:     make(map[string]string), // Directory files were found via walk, so no broken refs
		FormatWarnings: make(map[string]string),
		IsDirectory:    true,
	}, nil
}

// printValidateReport prints the validation report to stdout
func printValidateReport(cmd *cobra.Command, result validateResult) {
	// Header
	if result.IsDirectory {
		cmd.Printf("Validating: %s (directory)\n", result.Name)
	} else {
		cmd.Printf("Validating: %s\n", result.Name)
	}
	cmd.Printf("Name: %s\n", result.Name)
	if result.Version != "" {
		cmd.Printf("Version: %s\n", result.Version)
	}
	cmd.Println()

	// Coverage Summary
	totalKeys := len(result.AllKeys)
	totalMapped := len(result.MappedKeys)
	pct := float64(0)
	if totalKeys > 0 {
		pct = float64(totalMapped) / float64(totalKeys) * 100
	}

	cmd.Println("Coverage Summary:")
	cmd.Printf("  Total:       %d/%d (%.1f%%)\n", totalMapped, totalKeys, pct)

	// Per-category breakdown
	categoryKeys := make(map[string]int)   // category -> total keys
	categoryMapped := make(map[string]int) // category -> mapped keys

	for _, key := range result.AllKeys {
		cat := categoryFromKey(key)
		categoryKeys[cat]++
	}

	for key := range result.MappedKeys {
		cat := categoryFromKey(key)
		categoryMapped[cat]++
	}

	// Sort categories for consistent output
	categories := make([]string, 0, len(categoryKeys))
	for cat := range categoryKeys {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	for _, cat := range categories {
		total := categoryKeys[cat]
		mapped := categoryMapped[cat]
		catPct := float64(0)
		if total > 0 {
			catPct = float64(mapped) / float64(total) * 100
		}
		cmd.Printf("  %-14s %d/%d (%.1f%%)\n", cat+":", mapped, total, catPct)
	}

	// Broken References
	if len(result.BrokenRefs) > 0 {
		cmd.Println()
		cmd.Println("Broken References (files not found):")
		brokenKeys := make([]string, 0, len(result.BrokenRefs))
		for key := range result.BrokenRefs {
			brokenKeys = append(brokenKeys, key)
		}
		sort.Strings(brokenKeys)
		for _, key := range brokenKeys {
			cmd.Printf("  %s -> %s\n", key, result.BrokenRefs[key])
		}
	}

	// Format Warnings
	if len(result.FormatWarnings) > 0 {
		cmd.Println()
		cmd.Println("Format Warnings (non-audio extensions):")
		warnKeys := make([]string, 0, len(result.FormatWarnings))
		for key := range result.FormatWarnings {
			warnKeys = append(warnKeys, key)
		}
		sort.Strings(warnKeys)
		for _, key := range warnKeys {
			cmd.Printf("  %s -> %s\n", key, result.FormatWarnings[key])
		}
	}

	// Empty Mappings
	emptyKeys := make([]string, 0)
	for _, key := range result.AllKeys {
		if val, exists := result.Mappings[key]; exists && val == "" {
			emptyKeys = append(emptyKeys, key)
		} else if !exists {
			// Key not in mappings at all (e.g. for JSON packs with partial keys)
			if _, mapped := result.MappedKeys[key]; !mapped {
				emptyKeys = append(emptyKeys, key)
			}
		}
	}
	sort.Strings(emptyKeys)

	if len(emptyKeys) > 0 {
		cmd.Println()
		cmd.Println("Empty Mappings (no file assigned):")
		for _, key := range emptyKeys {
			cmd.Printf("  %s\n", key)
		}
	}
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

// newSoundpackInstallCommand creates the soundpack install subcommand
func newSoundpackInstallCommand() *cobra.Command {
	var setDefault bool
	var skipValidate bool

	installCmd := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a soundpack from a JSON file or directory",
		Long: `Install a soundpack by copying it to the XDG data directory and updating config.

JSON soundpacks are copied to <XDG_DATA_HOME>/claudio/<name>.json
Directory soundpacks are copied to <XDG_DATA_HOME>/claudio/soundpacks/<name>/

The installed path is added to config soundpack_paths (idempotent).
Use --default to also set the soundpack as the default.

Examples:
  claudio soundpack install my-pack.json
  claudio soundpack install /path/to/soundpack-dir
  claudio soundpack install my-pack.json --default
  claudio soundpack install my-pack.json --skip-validate`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackInstall(cmd, args[0], setDefault, skipValidate)
		},
	}

	installCmd.Flags().BoolVar(&setDefault, "default", false, "Set as the default soundpack")
	installCmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip validation before installing")

	return installCmd
}

// runSoundpackInstall executes the soundpack install command
func runSoundpackInstall(cmd *cobra.Command, srcPath string, setDefault, skipValidate bool) error {
	slog.Debug("running soundpack install", "path", srcPath, "set_default", setDefault, "skip_validate", skipValidate)

	// Check that the source path exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		slog.Error("cannot access source path", "path", srcPath, "error", err)
		return fmt.Errorf("cannot access source path: %w", err)
	}

	isDir := srcInfo.IsDir()

	// Determine soundpack name
	var name string
	if isDir {
		name = filepath.Base(srcPath)
	} else {
		name = strings.TrimSuffix(filepath.Base(srcPath), ".json")
	}
	slog.Info("determined soundpack name", "name", name, "is_directory", isDir)

	// Validate unless --skip-validate
	if !skipValidate {
		slog.Debug("validating soundpack before install")
		if isDir {
			_, valErr := validateDirectorySoundpack(srcPath)
			if valErr != nil {
				slog.Error("validation failed", "error", valErr)
				return fmt.Errorf("validation failed: %w", valErr)
			}
		} else {
			_, valErr := validateJSONSoundpackFile(srcPath)
			if valErr != nil {
				slog.Error("validation failed", "error", valErr)
				return fmt.Errorf("validation failed: %w", valErr)
			}
		}
		slog.Info("soundpack validation passed")
	}

	// For JSON files, read and extract name from the JSON content
	if !isDir {
		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			slog.Error("failed to read JSON file", "path", srcPath, "error", readErr)
			return fmt.Errorf("failed to read JSON file: %w", readErr)
		}
		var spFile soundpack.JSONSoundpackFile
		if jsonErr := json.Unmarshal(data, &spFile); jsonErr != nil {
			slog.Error("failed to parse JSON", "path", srcPath, "error", jsonErr)
			return fmt.Errorf("failed to parse JSON: %w", jsonErr)
		}
		if spFile.Name != "" {
			name = spFile.Name
			slog.Debug("using name from JSON file", "name", name)
		}
	}

	// Determine install target
	var installPath string
	if isDir {
		// Directories -> <xdg.DataHome>/claudio/soundpacks/<name>/
		installPath = filepath.Join(xdg.DataHome, "claudio", "soundpacks", name)
	} else {
		// JSON files -> <xdg.DataHome>/claudio/<name>.json
		installPath = filepath.Join(xdg.DataHome, "claudio", name+".json")
	}
	slog.Info("install target determined", "install_path", installPath)

	// Copy file/directory
	if isDir {
		if err := copyDirectory(srcPath, installPath); err != nil {
			slog.Error("failed to copy directory", "src", srcPath, "dst", installPath, "error", err)
			return fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := copyFile(srcPath, installPath); err != nil {
			slog.Error("failed to copy file", "src", srcPath, "dst", installPath, "error", err)
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}
	slog.Info("soundpack copied successfully", "install_path", installPath)

	// Update config
	if err := updateConfigForInstall(installPath, name, setDefault); err != nil {
		slog.Error("failed to update config", "error", err)
		return fmt.Errorf("failed to update config: %w", err)
	}

	cmd.Printf("Installed soundpack '%s' to %s\n", name, installPath)
	return nil
}

// updateConfigForInstall loads the config, adds the install path, optionally sets default, and saves.
func updateConfigForInstall(installPath, name string, setDefault bool) error {
	slog.Debug("updating config for install", "install_path", installPath, "name", name, "set_default", setDefault)

	cm := config.NewConfigManager()

	// Load existing config
	cfg, err := cm.LoadConfig()
	if err != nil {
		slog.Warn("could not load existing config, using defaults", "error", err)
		cfg = cm.GetDefaultConfig()
	}

	// Add install path to soundpack_paths if not already present (idempotent)
	pathExists := false
	for _, p := range cfg.SoundpackPaths {
		if p == installPath {
			pathExists = true
			break
		}
	}
	if !pathExists {
		cfg.SoundpackPaths = append(cfg.SoundpackPaths, installPath)
		slog.Info("added install path to soundpack_paths", "path", installPath)
	} else {
		slog.Debug("install path already in soundpack_paths", "path", installPath)
	}

	// Set default if requested
	if setDefault {
		cfg.DefaultSoundpack = name
		slog.Info("set default soundpack", "name", name)
	}

	// Determine config file path
	xdgDirs := config.NewXDGDirs()
	configPaths := xdgDirs.GetConfigPaths("config.json")
	var configFilePath string
	if len(configPaths) > 0 {
		configFilePath = configPaths[0] // First path is user config (highest priority)
	} else {
		configFilePath = filepath.Join(xdg.ConfigHome, "claudio", "config.json")
	}
	slog.Debug("config file path determined", "path", configFilePath)

	// Save config
	if err := cm.SaveToFile(cfg, configFilePath); err != nil {
		slog.Error("failed to save config", "path", configFilePath, "error", err)
		return fmt.Errorf("failed to save config: %w", err)
	}

	slog.Info("config updated successfully", "path", configFilePath)
	return nil
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
