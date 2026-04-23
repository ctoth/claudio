package soundpack

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var directoryAudioExtensions = []string{".wav", ".mp3", ".aiff", ".aif", ".mpeg"}

// DirectoryMapper maps relative paths to directory-based candidates
type DirectoryMapper struct {
	name      string
	basePaths []string
}

// NewDirectoryMapper creates a new directory-based path mapper
func NewDirectoryMapper(name string, basePaths []string) PathMapper {
	slog.Debug("creating directory mapper",
		"name", name,
		"base_paths", basePaths,
		"base_paths_count", len(basePaths))

	return &DirectoryMapper{
		name:      name,
		basePaths: basePaths,
	}
}

// MapPath converts a relative path to directory-based candidate absolute paths
func (d *DirectoryMapper) MapPath(relativePath string) ([]string, error) {
	if relativePath == "" {
		return []string{}, nil
	}

	slog.Debug("mapping directory path",
		"relative_path", relativePath,
		"base_paths_count", len(d.basePaths),
		"mapper_name", d.name)

	var candidates []string
	for i, basePath := range d.basePaths {
		candidate := filepath.Join(basePath, relativePath)
		candidates = append(candidates, candidate)

		slog.Debug("generated directory candidate",
			"index", i,
			"base_path", basePath,
			"relative_path", relativePath,
			"candidate", candidate)

		for _, alternateCandidate := range existingAlternateAudioPaths(basePath, relativePath) {
			candidates = append(candidates, alternateCandidate)

			slog.Debug("generated alternate directory candidate",
				"index", i,
				"base_path", basePath,
				"relative_path", relativePath,
				"candidate", alternateCandidate)
		}
	}

	slog.Debug("directory mapping completed",
		"relative_path", relativePath,
		"candidates_count", len(candidates),
		"mapper_name", d.name)

	return candidates, nil
}

func existingAlternateAudioPaths(basePath, relativePath string) []string {
	ext := strings.ToLower(filepath.Ext(relativePath))
	if ext == "" {
		return nil
	}

	isAudioPath := false
	for _, audioExt := range directoryAudioExtensions {
		if ext == audioExt {
			isAudioPath = true
			break
		}
	}
	if !isAudioPath {
		return nil
	}

	stem := strings.TrimSuffix(relativePath, filepath.Ext(relativePath))
	seen := map[string]struct{}{
		strings.ToLower(filepath.Join(basePath, relativePath)): {},
	}
	var alternates []string

	for _, audioExt := range directoryAudioExtensions {
		alternate := filepath.Join(basePath, stem+audioExt)
		key := strings.ToLower(alternate)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		if _, err := os.Stat(alternate); err == nil {
			alternates = append(alternates, alternate)
		}
	}

	return alternates
}

// GetName returns the name of this directory mapper
func (d *DirectoryMapper) GetName() string {
	return d.name
}

// GetType returns the type identifier for directory mappers
func (d *DirectoryMapper) GetType() string {
	return "directory"
}
