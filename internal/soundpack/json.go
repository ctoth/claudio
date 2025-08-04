package soundpack

import (
	"log/slog"
)

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
