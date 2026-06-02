package soundpack

import (
	"fmt"
	"log/slog"
	"os"
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
//
// Cross-platform absolute-path handling: filepath.IsAbs is GOOS-aware,
// so `/etc/shadow` is "not absolute" on Windows and `C:\Windows` is
// "not absolute" on Linux. A soundpack JSON crafted on one platform
// must not bypass validation when loaded on another. We therefore
// reject *any* value that looks absolute under *any* common convention
// in addition to the GOOS-aware check.
func validateMappingValue(value, baseDir string) (resolved string, err error) {
	if value == "" {
		return "", fmt.Errorf("empty mapping value")
	}
	if isAnyPlatformAbsolute(value) {
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
	// Symlink defense: the syntactic Rel check above is purely textual —
	// a symlink committed inside the soundpack root (e.g. preserved by
	// git from a malicious gh: source) that points outside would still
	// resolve to outside content at decode time. Resolve symlinks now
	// and re-verify the resolved path stays under baseDir.
	//
	// Gated on Lstat success because validation may run before the file
	// exists on disk in some flows (e.g. soundpack init scaffolds). If
	// the file isn't there yet, the syntactic check is all we can do;
	// the missing-file error will surface at load time.
	if _, statErr := os.Lstat(cleaned); statErr == nil {
		resolved, evalErr := filepath.EvalSymlinks(cleaned)
		if evalErr != nil {
			return "", fmt.Errorf("evaluating symlinks for %q: %w", value, evalErr)
		}
		absBase, baseErr := filepath.Abs(baseDir)
		if baseErr != nil {
			return "", fmt.Errorf("resolving baseDir %q: %w", baseDir, baseErr)
		}
		// EvalSymlinks resolves the absolute path of the target, so the
		// baseDir we compare against must also be absolute for Rel to
		// produce a meaningful answer.
		realBase, baseEvalErr := filepath.EvalSymlinks(absBase)
		if baseEvalErr != nil {
			// baseDir may not exist yet either (rare but possible);
			// fall back to absBase. The strict equality fails closed —
			// any non-existent base means we can't verify, so reject.
			realBase = absBase
		}
		resRel, relErr := filepath.Rel(realBase, resolved)
		if relErr != nil || strings.HasPrefix(resRel, "..") || resRel == ".." {
			return "", fmt.Errorf("symlink resolves outside soundpack root: %q -> %q", value, resolved)
		}
	}
	return cleaned, nil
}

// isAnyPlatformAbsolute returns true if value looks absolute under any
// of POSIX, Windows, or UNC conventions, regardless of the host GOOS.
// filepath.IsAbs is GOOS-aware, which is the wrong default for a trust
// boundary on portable data files like JSON soundpacks.
func isAnyPlatformAbsolute(value string) bool {
	if value == "" {
		return false
	}
	// GOOS-aware check first (handles platform-native shape).
	if filepath.IsAbs(value) {
		return true
	}
	// POSIX absolute: leading `/`.
	if value[0] == '/' {
		return true
	}
	// Windows backslash-rooted: leading `\` (e.g. `\Windows\System32`).
	if value[0] == '\\' {
		return true
	}
	// Windows drive-letter: `C:` or `C:/...` or `C:\...`.
	if len(value) >= 2 && value[1] == ':' {
		c := value[0]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
			return true
		}
	}
	return false
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
