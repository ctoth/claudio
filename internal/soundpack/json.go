package soundpack

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
)

// validateMappingValue checks an untrusted soundpack mapping value and
// resolves it relative to baseDir. Returns the resolved absolute path on
// success, or an error if the value is empty, absolute, contains a `..`
// segment, or resolves outside baseDir.
//
// This is the trust boundary for soundpack JSONs loaded from on-disk
// sources (untrusted). The trusted-embedded loader does NOT call this
// function — those JSONs may legitimately reference absolute system paths
// like /System/Library/Sounds/Purr.aiff.
func validateMappingValue(value, baseDir string) (resolved string, err error) {
	if value == "" {
		return "", fmt.Errorf("empty mapping value")
	}
	if filepath.IsAbs(value) {
		return "", fmt.Errorf("absolute paths not allowed: %q", value)
	}
	// Reject `..` segments BEFORE Clean — Clean would resolve `a/../b` to
	// `b` and silently lose the traversal attempt.
	for _, seg := range strings.Split(filepath.ToSlash(value), "/") {
		if seg == ".." {
			return "", fmt.Errorf("path traversal not allowed: %q", value)
		}
	}
	cleaned := filepath.Clean(filepath.Join(baseDir, value))
	// Defense in depth — Clean+Join should already have prevented escape,
	// but verify the result is still under baseDir.
	rel, err := filepath.Rel(baseDir, cleaned)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", fmt.Errorf("resolves outside soundpack root: %q", value)
	}
	return cleaned, nil
}

// JSONMapper maps relative paths to absolute paths defined in a JSON mapping
type JSONMapper struct {
	name    string
	mapping map[string]string
}

// NewJSONMapper creates a new JSON-based path mapper
func NewJSONMapper(name string, mapping map[string]string) PathMapper {
	slog.Debug("creating JSON mapper",
		"name", name,
		"mapping_keys_count", len(mapping))

	// Log some sample mappings for debugging
	sampleCount := 0
	for key, value := range mapping {
		if sampleCount < 3 { // Log first 3 mappings as samples
			slog.Debug("JSON mapping sample",
				"key", key,
				"value", value,
				"mapper_name", name)
			sampleCount++
		}
	}

	return &JSONMapper{
		name:    name,
		mapping: mapping,
	}
}

// MapPath converts a relative path to JSON-defined absolute path candidates
func (j *JSONMapper) MapPath(relativePath string) ([]string, error) {
	if relativePath == "" {
		return []string{}, nil
	}

	slog.Debug("mapping JSON path",
		"relative_path", relativePath,
		"mapper_name", j.name,
		"total_mappings", len(j.mapping))

	// Look up the relative path in the JSON mapping
	if absolutePath, exists := j.mapping[relativePath]; exists {
		slog.Debug("JSON mapping found",
			"relative_path", relativePath,
			"absolute_path", absolutePath,
			"mapper_name", j.name)

		return []string{absolutePath}, nil
	}

	slog.Debug("JSON mapping not found",
		"relative_path", relativePath,
		"mapper_name", j.name,
		"available_keys_count", len(j.mapping))

	// Return empty slice when no mapping exists
	return []string{}, nil
}

// GetName returns the name of this JSON mapper
func (j *JSONMapper) GetName() string {
	return j.name
}

// GetType returns the type identifier for JSON mappers
func (j *JSONMapper) GetType() string {
	return "json"
}
