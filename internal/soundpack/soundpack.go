package soundpack

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
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
			slog.Info("sound path resolved successfully",
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
			slog.Info("fallback resolution successful",
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

// LoadJSONSoundpack loads a JSON soundpack file and creates a JSONMapper
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
	fileData, err := io.ReadAll(file)
	if err != nil {
		slog.Error("failed to read JSON soundpack file", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to read JSON soundpack file: %w", err)
	}

	// Parse JSON
	var soundpack JSONSoundpackFile
	err = json.Unmarshal(fileData, &soundpack)
	if err != nil {
		slog.Error("failed to parse JSON soundpack", "file_path", filePath, "error", err)
		return nil, fmt.Errorf("failed to parse JSON soundpack: %w", err)
	}

	// Validate required fields
	if soundpack.Name == "" {
		err := fmt.Errorf("JSON soundpack missing required 'name' field")
		slog.Error("JSON soundpack validation failed", "file_path", filePath, "error", err)
		return nil, err
	}

	if soundpack.Mappings == nil || len(soundpack.Mappings) == 0 {
		err := fmt.Errorf("JSON soundpack missing or empty 'mappings' field")
		slog.Error("JSON soundpack validation failed", "file_path", filePath, "error", err)
		return nil, err
	}

	slog.Debug("JSON soundpack parsed successfully", 
		"file_path", filePath,
		"name", soundpack.Name,
		"mappings_count", len(soundpack.Mappings))

	// Validate that all referenced sound files exist
	for relativePath, absolutePath := range soundpack.Mappings {
		if _, err := os.Stat(absolutePath); err != nil {
			slog.Error("sound file not found", 
				"relative_path", relativePath,
				"absolute_path", absolutePath,
				"error", err)
			return nil, fmt.Errorf("sound file not found for mapping '%s' -> '%s': %w", 
				relativePath, absolutePath, err)
		}
		
		slog.Debug("sound file validation passed", 
			"relative_path", relativePath,
			"absolute_path", absolutePath)
	}

	slog.Info("JSON soundpack loaded successfully",
		"file_path", filePath,
		"name", soundpack.Name,
		"valid_mappings", len(soundpack.Mappings))

	// Create and return JSONMapper
	return NewJSONMapper(soundpack.Name, soundpack.Mappings), nil
}