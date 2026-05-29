package soundpack

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"claudio.click/internal/safeio"
)

// PathMapper defines how to map relative sound paths to candidate absolute paths
type PathMapper interface {
	// MapPath converts a relative sound path to candidate absolute paths
	MapPath(relativePath string) ([]string, error)
	GetName() string
	GetType() string
}

// SoundpackResolver resolves sound paths using a configurable mapping strategy
type SoundpackResolver interface {
	ResolveSound(relativePath string) (string, error)
	ResolveSoundWithFallback(paths []string) (string, error)
	GetName() string
	GetType() string
}

// UnifiedSoundpackResolver implements SoundpackResolver using any PathMapper
type UnifiedSoundpackResolver struct {
	mapper PathMapper
}

// NewSoundpackResolver creates a new unified soundpack resolver
func NewSoundpackResolver(mapper PathMapper) SoundpackResolver {
	slog.Debug("creating unified soundpack resolver",
		"mapper_name", mapper.GetName(),
		"mapper_type", mapper.GetType())

	return &UnifiedSoundpackResolver{
		mapper: mapper,
	}
}

// ResolveSound resolves a single sound path using the configured mapper
func (u *UnifiedSoundpackResolver) ResolveSound(relativePath string) (string, error) {
	if relativePath == "" {
		err := fmt.Errorf("sound path cannot be empty")
		slog.Error("resolve sound failed", "error", err)
		return "", err
	}

	slog.Debug("resolving sound path",
		"relative_path", relativePath,
		"mapper_type", u.mapper.GetType(),
		"mapper_name", u.mapper.GetName())

	// Get candidate paths from mapper
	candidates, err := u.mapper.MapPath(relativePath)
	if err != nil {
		slog.Error("path mapping failed", "relative_path", relativePath, "error", err)
		return "", fmt.Errorf("path mapping failed: %w", err)
	}

	slog.Debug("path mapping completed",
		"relative_path", relativePath,
		"candidates_count", len(candidates),
		"candidates", candidates)

	// Try each candidate path until we find an existing file
	for i, candidate := range candidates {
		slog.Debug("checking candidate", "index", i, "candidate", candidate)

		if _, err := os.Stat(candidate); err == nil {
			slog.Debug("sound path resolved successfully",
				"relative_path", relativePath,
				"resolved_path", candidate,
				"mapper_type", u.mapper.GetType(),
				"candidate_index", i)

			return candidate, nil
		} else {
			slog.Debug("candidate not found", "candidate", candidate, "error", err)
		}
	}

	// No candidates found
	err = &FileNotFoundError{
		SoundPath: relativePath,
		Paths:     candidates,
	}

	slog.Warn("sound path not resolved",
		"relative_path", relativePath,
		"candidates_checked", len(candidates),
		"mapper_type", u.mapper.GetType())

	return "", err
}

// ResolveSoundWithFallback tries multiple sound paths in order until one is found
func (u *UnifiedSoundpackResolver) ResolveSoundWithFallback(paths []string) (string, error) {
	if len(paths) == 0 {
		err := fmt.Errorf("no fallback paths provided")
		slog.Error("fallback resolution failed", "error", err)
		return "", err
	}

	slog.Debug("resolving sound with fallback",
		"paths", paths,
		"mapper_type", u.mapper.GetType())

	var lastErr error
	for i, path := range paths {
		slog.Debug("trying fallback path", "index", i, "path", path)

		resolved, err := u.ResolveSound(path)
		if err == nil {
			slog.Debug("fallback resolution successful",
				"resolved_path", resolved,
				"fallback_index", i,
				"fallback_path", path,
				"mapper_type", u.mapper.GetType())

			return resolved, nil
		}

		lastErr = err
		slog.Debug("fallback path failed", "index", i, "path", path, "error", err)
	}

	slog.Warn("all fallback paths failed",
		"paths_tried", len(paths),
		"mapper_type", u.mapper.GetType())

	return "", lastErr
}

// GetName returns the name of the underlying mapper
func (u *UnifiedSoundpackResolver) GetName() string {
	return u.mapper.GetName()
}

// GetType returns the type of the underlying mapper
func (u *UnifiedSoundpackResolver) GetType() string {
	return u.mapper.GetType()
}

// FileNotFoundError represents a sound file not found error
type FileNotFoundError struct {
	SoundPath string
	Paths     []string
}

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("sound file not found: %s (searched in: %s)", e.SoundPath, strings.Join(e.Paths, ", "))
}

// IsFileNotFoundError checks if an error is a FileNotFoundError
func IsFileNotFoundError(err error) bool {
	_, ok := err.(*FileNotFoundError)
	return ok
}

// JSONSoundpackFile represents the structure of a JSON soundpack file
type JSONSoundpackFile struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Mappings    map[string]string `json:"mappings"`
}

// MaxSoundpackMappings caps the number of entries in a soundpack JSON.
// Defense against pathological JSONs that pass the byte-size cap but
// would still pin os.Stat calls during existence-check validation.
//
// Shipped platform JSONs have ~90 mappings; a maximalist legitimate
// soundpack covering every fallback key is on the order of low hundreds.
// 10,000 is two orders of magnitude over realistic legitimate use.
const MaxSoundpackMappings = 10_000

// LoadJSONSoundpack loads an UNTRUSTED JSON soundpack file from disk and
// returns a JSONMapper. The file's directory is used as the soundpack
// root; every mapping value must be a relative path resolving under that
// root (no absolute paths, no `..` traversal). For trusted go:embed
// bytes that reference absolute system paths, use
// LoadEmbeddedPlatformSoundpack instead.
func LoadJSONSoundpack(filePath string) (PathMapper, error) {
	slog.Debug("loading JSON soundpack", "file_path", filePath)

	// Open and read the JSON file
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("failed to open JSON soundpack file", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to open JSON soundpack file: %w", err)
	}
	defer file.Close()

	// Read file contents
	fileData, err := safeio.ReadAllCapped(file, safeio.MaxSoundpackJSONBytes, "soundpack JSON")
	if err != nil {
		slog.Error("failed to read JSON soundpack file", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read JSON soundpack file: %w", err)
	}

	mapper, err := loadJSONSoundpackUntrusted(fileData, filepath.Dir(filePath))
	if err != nil {
		slog.Error("JSON soundpack load failed", "file_path", filePath, "error", err)
		return nil, err
	}

	slog.Debug("JSON soundpack loaded successfully",
		"file_path", filePath,
		"name", mapper.GetName())

	return mapper, nil
}

// LoadJSONSoundpackFromBytes loads an UNTRUSTED JSON soundpack from byte
// data and resolves mapping values relative to baseDir. Use this when
// the bytes came from an attacker-influenced source (downloaded
// soundpack, user-supplied JSON) and you have a directory to root
// relative paths against.
//
// For trusted go:embed bytes use LoadEmbeddedPlatformSoundpack.
func LoadJSONSoundpackFromBytes(data []byte, baseDir string) (PathMapper, error) {
	slog.Debug("loading untrusted JSON soundpack from bytes",
		"data_size", len(data),
		"base_dir", baseDir)
	return loadJSONSoundpackUntrusted(data, baseDir)
}

// LoadEmbeddedPlatformSoundpack loads a TRUSTED JSON soundpack from
// go:embed bytes baked into the binary at build time. It skips the
// path-syntax validation (absolute-path rejection, `..` rejection,
// under-baseDir resolution) because shipped platform soundpacks
// legitimately reference absolute system paths like
// /System/Library/Sounds/Purr.aiff.
//
// The mappings-count cap and the existence check still apply — those
// are DoS guards, not trust checks. The byte-size cap is the caller's
// responsibility (embedded bytes are usually trusted to be small).
func LoadEmbeddedPlatformSoundpack(data []byte) (PathMapper, error) {
	slog.Debug("loading trusted embedded platform soundpack", "data_size", len(data))
	return loadJSONSoundpackTrusted(data)
}

// loadJSONSoundpackUntrusted is the internal entry point for untrusted
// JSONs. It parses the bytes, applies the mappings-count cap, runs
// validateMappingValue on every value (resolving relatives under
// baseDir; rejecting absolutes and `..` segments), then runs the
// existence check.
func loadJSONSoundpackUntrusted(data []byte, baseDir string) (PathMapper, error) {
	var soundpack JSONSoundpackFile
	if err := json.Unmarshal(data, &soundpack); err != nil {
		slog.Error("failed to parse untrusted JSON soundpack", "error", err)
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}

	if err := validateJSONSoundpackBasics(soundpack); err != nil {
		return nil, err
	}

	// Resolve and validate each mapping value through the trust boundary.
	resolved := make(map[string]string, len(soundpack.Mappings))
	for key, value := range soundpack.Mappings {
		abs, err := validateMappingValue(value, baseDir)
		if err != nil {
			slog.Error("mapping value rejected",
				"key", key,
				"value", value,
				"base_dir", baseDir,
				"error", err)
			return nil, fmt.Errorf("invalid mapping %q: %w", key, err)
		}
		resolved[key] = abs
	}
	soundpack.Mappings = resolved

	if err := validateMappingFilesExist(soundpack); err != nil {
		return nil, err
	}

	slog.Debug("untrusted JSON soundpack parsed",
		"name", soundpack.Name,
		"mappings_count", len(soundpack.Mappings))

	return NewJSONMapper(soundpack.Name, soundpack.Mappings), nil
}

// loadJSONSoundpackTrusted is the internal entry point for trusted
// embedded JSONs. It applies the mappings-count cap and the existence
// check but skips path-syntax validation, since embedded JSONs legitimately
// reference absolute system paths.
func loadJSONSoundpackTrusted(data []byte) (PathMapper, error) {
	var soundpack JSONSoundpackFile
	if err := json.Unmarshal(data, &soundpack); err != nil {
		slog.Error("failed to parse trusted JSON soundpack", "error", err)
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}

	if err := validateJSONSoundpackBasics(soundpack); err != nil {
		return nil, err
	}

	if err := validateMappingFilesExist(soundpack); err != nil {
		return nil, err
	}

	slog.Debug("trusted JSON soundpack parsed",
		"name", soundpack.Name,
		"mappings_count", len(soundpack.Mappings))

	return NewJSONMapper(soundpack.Name, soundpack.Mappings), nil
}

// ResolveJSONSoundpackMappings converts non-empty relative mapping values to
// absolute paths rooted at baseDir. Absolute values are preserved.
//
// Retained for backward compatibility with code paths that build a
// JSONSoundpackFile by hand and want to canonicalize relative values
// the same way the loader does. New code should use
// LoadJSONSoundpack(FromBytes) which now performs the strict validation.
func ResolveJSONSoundpackMappings(soundpack *JSONSoundpackFile, baseDir string) {
	if soundpack == nil || baseDir == "" {
		return
	}

	for relativePath, mappedPath := range soundpack.Mappings {
		if mappedPath == "" || filepath.IsAbs(mappedPath) {
			continue
		}
		soundpack.Mappings[relativePath] = filepath.Clean(filepath.Join(baseDir, mappedPath))
	}
}

// validateJSONSoundpackBasics checks structural invariants shared by
// the trusted and untrusted load paths: required fields, non-empty
// mappings, and the mappings-count cap.
func validateJSONSoundpackBasics(soundpack JSONSoundpackFile) error {
	if soundpack.Name == "" {
		return fmt.Errorf("JSON soundpack missing required 'name' field")
	}
	if len(soundpack.Mappings) == 0 {
		return fmt.Errorf("JSON soundpack missing or empty 'mappings' field")
	}
	if len(soundpack.Mappings) > MaxSoundpackMappings {
		return fmt.Errorf("soundpack mappings exceed limit of %d entries (got %d)",
			MaxSoundpackMappings, len(soundpack.Mappings))
	}
	return nil
}

// validateMappingFilesExist runs os.Stat on each mapping value and
// returns an error if any referenced file is missing. The mappings-count
// cap (validateJSONSoundpackBasics) bounds the number of stat calls.
func validateMappingFilesExist(soundpack JSONSoundpackFile) error {
	for relativePath, absolutePath := range soundpack.Mappings {
		if _, err := os.Stat(absolutePath); err != nil {
			slog.Error("sound file not found",
				"relative_path", relativePath,
				"absolute_path", absolutePath,
				"error", err)
			return fmt.Errorf("sound file not found for mapping '%s' -> '%s': %w",
				relativePath, absolutePath, err)
		}

		slog.Debug("sound file validation passed",
			"relative_path", relativePath,
			"absolute_path", absolutePath)
	}
	return nil
}

// validateJSONSoundpack is kept as a backwards-compatible wrapper that
// applies the full validation chain a non-loader caller might rely on
// (basics + existence check). It does NOT apply the path-syntax
// validation — callers that want that should use the loader.
func validateJSONSoundpack(soundpack JSONSoundpackFile) error {
	if err := validateJSONSoundpackBasics(soundpack); err != nil {
		return err
	}
	return validateMappingFilesExist(soundpack)
}

// CreateSoundpackMapper auto-detects soundpack type and creates appropriate mapper
func CreateSoundpackMapper(name, path string) (PathMapper, error) {
	slog.Debug("creating soundpack mapper", "name", name, "path", path)

	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		slog.Error("soundpack path does not exist", "path", path, "error", err)
		return nil, fmt.Errorf("soundpack path does not exist: %w", err)
	}

	// Auto-detect based on file extension and type
	if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".json") {
		slog.Debug("detected JSON soundpack", "path", path)
		return LoadJSONSoundpack(path)
	}

	// Assume directory soundpack for directories or other file types
	if info.IsDir() {
		slog.Debug("detected directory soundpack", "path", path)
		return NewDirectoryMapper(name, []string{path}), nil
	}

	// For non-JSON files, treat as an error for now
	slog.Error("unsupported soundpack type", "path", path, "is_dir", info.IsDir())
	return nil, fmt.Errorf("unsupported soundpack type: %s (must be directory or .json file)", path)
}

// CreateSoundpackMapperWithBasePaths creates a mapper with fallback to base paths
// This is used when the exact soundpack path doesn't exist but we have base directories to search
func CreateSoundpackMapperWithBasePaths(name, primaryPath string, basePaths []string) (PathMapper, error) {
	slog.Debug("creating soundpack mapper with base paths",
		"name", name,
		"primary_path", primaryPath,
		"base_paths", basePaths)

	// First try to create mapper with primary path
	mapper, err := CreateSoundpackMapper(name, primaryPath)
	if err == nil {
		slog.Debug("primary path succeeded", "primary_path", primaryPath)
		return mapper, nil
	}

	slog.Debug("primary path failed, falling back to base paths",
		"primary_path", primaryPath,
		"primary_error", err,
		"base_paths_count", len(basePaths))

	// If primary path fails, create directory mapper with base paths
	// This allows searching for soundpack in multiple directories
	if len(basePaths) == 0 {
		slog.Error("no base paths provided for fallback", "primary_path", primaryPath)
		return nil, fmt.Errorf("primary path failed and no base paths provided: %w", err)
	}

	// Create directory mapper with base paths for fallback
	slog.Debug("creating directory mapper with base paths",
		"name", name,
		"base_paths", basePaths)

	return NewDirectoryMapper(name, basePaths), nil
}
