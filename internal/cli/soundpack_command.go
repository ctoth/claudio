package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sort"

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
	// More subcommands will be added later by other agents
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
