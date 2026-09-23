package soundpack

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// audioExtensions lists the file extensions the audio decoders play, in the
// order directory packs try them as alternates for a sound key.
var audioExtensions = []string{".wav", ".mp3", ".aiff", ".aif", ".mpeg"}

// IsAudioExt reports whether ext (with its leading dot, any case) is a
// playable audio file extension.
func IsAudioExt(ext string) bool {
	return slices.Contains(audioExtensions, strings.ToLower(ext))
}

// DirectoryMapper maps relative paths to directory-based candidates
type DirectoryMapper struct {
	name      string
	basePaths []string
}

// NewDirectoryMapper creates a new directory-based path mapper
func NewDirectoryMapper(name string, basePaths []string) PathMapper {
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

	var candidates []string
	for _, basePath := range d.basePaths {
		candidates = append(candidates, filepath.Join(basePath, relativePath))
		candidates = append(candidates, existingAlternateAudioPaths(basePath, relativePath)...)
	}

	slog.Debug("directory mapping completed",
		"relative_path", relativePath,
		"candidates_count", len(candidates),
		"mapper_name", d.name)

	return candidates, nil
}

func existingAlternateAudioPaths(basePath, relativePath string) []string {
	if !IsAudioExt(filepath.Ext(relativePath)) {
		return nil
	}

	stem := strings.TrimSuffix(relativePath, filepath.Ext(relativePath))
	seen := map[string]struct{}{
		strings.ToLower(filepath.Join(basePath, relativePath)): {},
	}
	var alternates []string

	for _, audioExt := range audioExtensions {
		alternate := filepath.Join(basePath, stem+audioExt)
		key := strings.ToLower(alternate)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}

		if info, err := os.Stat(alternate); err == nil && info.Mode().IsRegular() {
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
