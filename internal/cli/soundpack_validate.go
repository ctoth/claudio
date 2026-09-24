package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

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
  2. Referenced files exist: non-empty mappings point to real files, given as
     relative paths inside the soundpack (no absolute paths or "..")
  3. Coverage gaps: compare mappings against all known sound keys
  4. Format check: referenced files should be .wav, .mp3, .mpeg, .aiff, or .aif

Exit code 0 if no broken or unsafe references, non-zero otherwise.
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
	UnsafeRefs     map[string]string // key -> reason the mapping path was rejected
	FormatWarnings map[string]string // key -> path for files with non-audio extensions
	IsDirectory    bool
	problems       error // non-nil when BrokenRefs or UnsafeRefs is non-empty
}

// runSoundpackValidate executes the soundpack validate command
func runSoundpackValidate(cmd *cobra.Command, path string) error {
	result, err := validateSoundpackPath(path)
	if err != nil {
		return err
	}

	printValidateReport(cmd, result)

	// Exit with non-zero if there are broken or unsafe references
	if err := result.Err(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// validateJSONSoundpackFile validates a JSON soundpack file
func validateJSONSoundpackFile(path string) (validateResult, error) {
	// ValidateJSONSoundpack applies the size cap, basics and mappings
	// count cap, then checks every mapping through the runtime loader's
	// trust boundary, so validate, install and add accept exactly what the
	// runtime will load.
	v, err := soundpack.ValidateJSONSoundpack(path)
	if err != nil {
		return validateResult{}, fmt.Errorf("failed to load JSON soundpack: %w", err)
	}

	slog.Info("parsed JSON soundpack", "name", v.File.Name, "mappings", len(v.File.Mappings))

	allKeys, err := ExtractAllSoundKeys()
	if err != nil {
		return validateResult{}, fmt.Errorf("failed to extract sound keys: %w", err)
	}

	mappedKeys := make(map[string]string)
	for key, val := range v.File.Mappings {
		if val != "" {
			mappedKeys[key] = val
		}
	}
	formatWarnings := make(map[string]string)
	for key, resolved := range v.Resolved {
		if !soundpack.IsAudioExt(filepath.Ext(resolved)) {
			slog.Warn("non-audio format", "key", key, "path", resolved)
			formatWarnings[key] = resolved
		}
	}

	return validateResult{
		Name:           v.File.Name,
		Version:        v.File.Version,
		Mappings:       v.File.Mappings,
		AllKeys:        allKeys,
		MappedKeys:     mappedKeys,
		BrokenRefs:     v.Broken,
		UnsafeRefs:     v.Unsafe,
		FormatWarnings: formatWarnings,
		IsDirectory:    false,
		problems:       v.Err(),
	}, nil
}

// validateSoundpackPath validates a directory or JSON soundpack at path.
// The returned error covers only unreadable or malformed packs; mapping
// problems are in the result (see validateResult.Err).
func validateSoundpackPath(path string) (validateResult, error) {
	info, err := os.Stat(path)
	if err != nil {
		return validateResult{}, fmt.Errorf("cannot access path: %w", err)
	}
	if info.IsDir() {
		return validateDirectorySoundpack(path)
	}
	return validateJSONSoundpackFile(path)
}

// resolveDirectoryKey returns the first existing regular file mapper
// offers for key, or "" when none exists.
func resolveDirectoryKey(mapper soundpack.PathMapper, key string) string {
	candidates, err := mapper.MapPath(key)
	if err != nil {
		return ""
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}

// Err reports broken or unsafe mappings as an error.
func (r validateResult) Err() error {
	return r.problems
}

// validateDirectorySoundpack validates a directory-based soundpack
func validateDirectorySoundpack(dirPath string) (validateResult, error) {
	// Get all known keys
	allKeys, err := ExtractAllSoundKeys()
	if err != nil {
		return validateResult{}, fmt.Errorf("failed to extract sound keys: %w", err)
	}

	// Walk the directory first: it rejects symlinked audio files.
	audioFiles := make(map[string]string)
	if err := filepath.Walk(dirPath, newDirectorySoundpackWalkFunc(dirPath, audioFiles)); err != nil {
		return validateResult{}, fmt.Errorf("failed to scan directory soundpack: %w", err)
	}

	// Coverage counts a known key only when the runtime's directory mapper
	// resolves it (including alternate extensions such as .mp3 for a .wav
	// key). Stray files that match no key do not count, so coverage cannot
	// exceed 100%.
	name := filepath.Base(dirPath)
	mapper := soundpack.NewDirectoryMapper(name, []string{dirPath})
	mappedKeys := make(map[string]string)
	allMappings := make(map[string]string, len(allKeys))
	for _, key := range allKeys {
		allMappings[key] = ""
		if path := resolveDirectoryKey(mapper, key); path != "" {
			mappedKeys[key] = path
			allMappings[key] = path
		}
	}

	slog.Info("scanned directory soundpack", "name", name, "audio_files", len(audioFiles), "covered_keys", len(mappedKeys))

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

	// Coverage Summary. Only known keys count, so coverage never exceeds
	// 100% (a JSON pack may map extra, unknown keys).
	totalKeys := len(result.AllKeys)
	totalMapped := 0
	categoryKeys := make(map[string]int)   // category -> total keys
	categoryMapped := make(map[string]int) // category -> mapped keys
	for _, key := range result.AllKeys {
		cat := categoryFromKey(key)
		categoryKeys[cat]++
		if _, mapped := result.MappedKeys[key]; mapped {
			totalMapped++
			categoryMapped[cat]++
		}
	}
	pct := float64(0)
	if totalKeys > 0 {
		pct = float64(totalMapped) / float64(totalKeys) * 100
	}

	cmd.Println("Coverage Summary:")
	cmd.Printf("  Total:       %d/%d (%.1f%%)\n", totalMapped, totalKeys, pct)

	// Per-category breakdown, sorted for consistent output
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

	// Unsafe References
	if len(result.UnsafeRefs) > 0 {
		cmd.Println()
		cmd.Println("Unsafe References (must be relative paths inside the soundpack):")
		unsafeKeys := make([]string, 0, len(result.UnsafeRefs))
		for key := range result.UnsafeRefs {
			unsafeKeys = append(unsafeKeys, key)
		}
		sort.Strings(unsafeKeys)
		for _, key := range unsafeKeys {
			cmd.Printf("  %s -> %s (%s)\n", key, result.Mappings[key], result.UnsafeRefs[key])
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

// newDirectorySoundpackWalkFunc returns a filepath.WalkFunc that records every
// audio file under dirPath in found, keyed by its slash-separated relative path.
// VCS metadata directories are skipped, and entries that vanish mid-walk (e.g.
// git's transient maintenance.lock in a managed clone) are ignored.
func newDirectorySoundpackWalkFunc(dirPath string, found map[string]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if path != dirPath && errors.Is(walkErr, fs.ErrNotExist) {
				slog.Debug("skipping entry that vanished during soundpack scan", "path", path, "error", walkErr)
				return nil
			}
			return walkErr
		}
		if info.IsDir() {
			if path != dirPath && info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if !soundpack.IsAudioExt(filepath.Ext(path)) {
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

		found[key] = path

		return nil
	}
}
