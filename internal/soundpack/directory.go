package soundpack

import (
	"log/slog"
	"path/filepath"
)

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
	}

	slog.Debug("directory mapping completed", 
		"relative_path", relativePath,
		"candidates_count", len(candidates),
		"mapper_name", d.name)

	return candidates, nil
}

// GetName returns the name of this directory mapper
func (d *DirectoryMapper) GetName() string {
	return d.name
}

// GetType returns the type identifier for directory mappers
func (d *DirectoryMapper) GetType() string {
	return "directory"
}