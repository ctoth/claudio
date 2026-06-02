package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"claudio.click/internal/soundpack"
	"github.com/spf13/cobra"
)

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
	Name           string
	Version        string
	Mappings       map[string]string // all mappings from the soundpack
	AllKeys        []string          // all known keys
	MappedKeys     map[string]string // keys with non-empty values
	BrokenRefs     map[string]string // key -> path for files that don't exist
	FormatWarnings map[string]string // key -> path for files with non-audio extensions
	IsDirectory    bool
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

	// Peek applies the size cap + basics + mappings count cap. The
	// validate command then walks the mappings to produce its own
	// detailed broken-references report, which is richer than what the
	// strict untrusted loader returns.
	spFilePtr, err := soundpack.PeekJSONSoundpackFromFile(path)
	if err != nil {
		slog.Error("failed to peek file", "path", path, "error", err)
		return validateResult{}, fmt.Errorf("failed to load JSON soundpack: %w", err)
	}
	spFile := *spFilePtr
	soundpack.ResolveJSONSoundpackMappings(&spFile, filepath.Dir(path))

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
	walkErr := filepath.Walk(dirPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".wav" && ext != ".mp3" && ext != ".aiff" {
			return nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("directory soundpack contains symlinked audio file: %s", path)
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
	if walkErr != nil {
		return validateResult{}, fmt.Errorf("failed to scan directory soundpack: %w", walkErr)
	}

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
