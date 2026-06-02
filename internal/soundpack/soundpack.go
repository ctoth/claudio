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
	ResolveSoundWithFallback(paths []string, opts ...ResolveOption) (string, error)
	GetName() string
	GetType() string
}

// PathObserver receives one callback per logical-path candidate the resolver
// inspects during a fallback resolution. sequence is 1-based (loop index over
// the input candidates slice); exists indicates whether the candidate resolved
// to a file present on disk. Observers MUST NOT mutate the path or block —
// resolution is on the hot hook path.
//
// Concurrency: the current UnifiedSoundpackResolver fires the observer
// synchronously from a single goroutine, so a non-goroutine-safe observer
// is correct today. The PathObserver contract however reserves the right
// for future resolver implementations to fan resolution out across
// goroutines (e.g. parallel os.Stat), so observers SHOULD be goroutine-
// safe. tracking.LookupBuffer's observer takes an internal sync.Mutex
// precisely for this reason.
type PathObserver func(path string, sequence int, exists bool)

// ResolveOption configures a single ResolveSoundWithFallback call.
type ResolveOption func(*resolveConfig)

// WithObserver attaches an observer to a ResolveSoundWithFallback call. The
// observer fires once per candidate path in order, regardless of whether the
// resolver found a winner before reaching that candidate or kept walking.
//
// Use this to instrument resolution (e.g. tracking telemetry) without
// duplicating the os.Stat I/O that the resolver already performs.
func WithObserver(obs PathObserver) ResolveOption {
	return func(c *resolveConfig) { c.observer = obs }
}

// resolveConfig is the private config struct built from the variadic
// ResolveOptions passed to ResolveSoundWithFallback.
type resolveConfig struct {
	observer PathObserver
}

// buildResolveConfig folds the variadic options into a resolveConfig.
func buildResolveConfig(opts []ResolveOption) resolveConfig {
	cfg := resolveConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
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

// ResolveSoundWithFallback tries multiple sound paths in order until one is
// found. Optional ResolveOptions configure per-call behavior — most notably
// WithObserver(...) which fires a PathObserver callback for every candidate
// the resolver inspects, in input order, regardless of whether resolution
// short-circuited on an earlier win. The observer is invoked once per input
// candidate (deduplication and chain composition is the caller's concern).
//
// The observer is invoked with exists=true ONLY when the candidate resolved
// to a physical file present on disk. A mapping miss (ResolveSound returns
// an error) is reported as exists=false.
func (u *UnifiedSoundpackResolver) ResolveSoundWithFallback(paths []string, opts ...ResolveOption) (string, error) {
	cfg := buildResolveConfig(opts)

	if len(paths) == 0 {
		err := fmt.Errorf("no fallback paths provided")
		slog.Error("fallback resolution failed", "error", err)
		return "", err
	}

	slog.Debug("resolving sound with fallback",
		"paths", paths,
		"mapper_type", u.mapper.GetType())

	var lastErr error
	var winner string
	winnerFound := false
	for i, path := range paths {
		sequence := i + 1
		slog.Debug("trying fallback path", "index", i, "path", path)

		// Once a winner is found, observer still fires for remaining
		// candidates with exists=false — the observer's contract is one
		// call per INPUT candidate, in order. (Today no caller depends
		// on the post-winner tail; emitting it preserves the "lookups
		// reflect the full chain shape" telemetry intent of Chunk 12.)
		if winnerFound {
			if cfg.observer != nil {
				cfg.observer(path, sequence, false)
			}
			continue
		}

		resolved, err := u.ResolveSound(path)
		if err == nil {
			if cfg.observer != nil {
				cfg.observer(path, sequence, true)
			}

			slog.Debug("fallback resolution successful",
				"resolved_path", resolved,
				"fallback_index", i,
				"fallback_path", path,
				"mapper_type", u.mapper.GetType())

			winner = resolved
			winnerFound = true
			continue
		}

		if cfg.observer != nil {
			cfg.observer(path, sequence, false)
		}

		lastErr = err
		slog.Debug("fallback path failed", "index", i, "path", path, "error", err)
	}

	if winnerFound {
		return winner, nil
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
// bytes baked into the binary via go:embed at build time. It skips the
// path-syntax validation (absolute-path rejection, `..` rejection,
// under-baseDir resolution) because shipped platform soundpacks
// legitimately reference absolute system paths like
// /System/Library/Sounds/Purr.aiff.
// Relative mapping values are resolved against the optional basePaths
// before the existence check. This lets embedded platform packs point at
// files installed under XDG soundpack directories without depending on
// the hook process's current working directory.
//
// The mappings-count cap and the existence check still apply — those
// are DoS guards, not trust checks. The byte-size cap is the caller's
// responsibility (embedded bytes are usually trusted to be small).
func LoadEmbeddedPlatformSoundpack(data []byte, basePaths ...string) (PathMapper, error) {
	slog.Debug("loading trusted embedded platform soundpack", "data_size", len(data))
	return loadJSONSoundpackTrusted(data, basePaths)
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
func loadJSONSoundpackTrusted(data []byte, basePaths []string) (PathMapper, error) {
	var soundpack JSONSoundpackFile
	if err := json.Unmarshal(data, &soundpack); err != nil {
		slog.Error("failed to parse trusted JSON soundpack", "error", err)
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}

	if err := validateJSONSoundpackBasics(soundpack); err != nil {
		return nil, err
	}

	resolveTrustedRelativeMappings(&soundpack, basePaths)

	if err := validateMappingFilesExist(soundpack); err != nil {
		return nil, err
	}

	slog.Debug("trusted JSON soundpack parsed",
		"name", soundpack.Name,
		"mappings_count", len(soundpack.Mappings))

	return NewJSONMapper(soundpack.Name, soundpack.Mappings), nil
}

func resolveTrustedRelativeMappings(soundpack *JSONSoundpackFile, basePaths []string) {
	if soundpack == nil || len(basePaths) == 0 {
		return
	}

	for key, value := range soundpack.Mappings {
		if value == "" || isAnyPlatformAbsolute(value) {
			continue
		}

		var firstCandidate string
		for _, basePath := range basePaths {
			if basePath == "" {
				continue
			}
			candidate := filepath.Clean(filepath.Join(basePath, value))
			if firstCandidate == "" {
				firstCandidate = candidate
			}
			if _, err := os.Stat(candidate); err == nil {
				soundpack.Mappings[key] = candidate
				break
			}
		}
		if soundpack.Mappings[key] == value && firstCandidate != "" {
			soundpack.Mappings[key] = firstCandidate
		}
	}
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

// PeekJSONSoundpackFromBytes parses a JSON soundpack from byte data and
// returns its struct, applying the basics check (name + non-empty
// mappings) and the mappings-count cap, but NOT the path-syntax
// validation or the existence check. It is intended for discovery,
// listing, and metadata-only consumers that need to inspect a soundpack
// without dereferencing its files — e.g. counting mappings, extracting
// the name, listing keys across multiple soundpacks where some
// referenced files may not exist on the host platform.
//
// Callers that intend to USE the soundpack (resolve sounds at runtime)
// must use LoadJSONSoundpack(FromBytes) or LoadEmbeddedPlatformSoundpack
// instead.
func PeekJSONSoundpackFromBytes(data []byte) (*JSONSoundpackFile, error) {
	var sp JSONSoundpackFile
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}
	if err := validateJSONSoundpackBasics(sp); err != nil {
		return nil, err
	}
	return &sp, nil
}

// PeekJSONSoundpackFromFile opens an on-disk soundpack JSON, reads it
// under the safeio cap, and parses it via PeekJSONSoundpackFromBytes.
// Same semantics as the bytes variant: applies the size cap, basics
// check, and mappings-count cap; skips path-syntax validation and the
// existence check.
func PeekJSONSoundpackFromFile(path string) (*JSONSoundpackFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON soundpack file: %w", err)
	}
	defer f.Close()

	data, err := safeio.ReadAllCapped(f, safeio.MaxSoundpackJSONBytes, "soundpack JSON")
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON soundpack file: %w", err)
	}
	return PeekJSONSoundpackFromBytes(data)
}

// PeekJSONSoundpackMetadataFromFile is a permissive variant of
// PeekJSONSoundpackFromFile that applies ONLY the size cap and the
// mappings-count cap. It does NOT require `name` or non-empty
// `mappings` to be set, so it can read partially-populated soundpacks
// (e.g. install command extracting the name from a JSON whose mappings
// are filled in later, or a soundpack init scaffold).
//
// Use this when you need to inspect metadata from an arbitrary
// soundpack JSON without rejecting work-in-progress files.
func PeekJSONSoundpackMetadataFromFile(path string) (*JSONSoundpackFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON soundpack file: %w", err)
	}
	defer f.Close()

	data, err := safeio.ReadAllCapped(f, safeio.MaxSoundpackJSONBytes, "soundpack JSON")
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON soundpack file: %w", err)
	}

	var sp JSONSoundpackFile
	if err := json.Unmarshal(data, &sp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}
	if len(sp.Mappings) > MaxSoundpackMappings {
		return nil, fmt.Errorf("soundpack mappings exceed limit of %d entries (got %d)",
			MaxSoundpackMappings, len(sp.Mappings))
	}
	return &sp, nil
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
