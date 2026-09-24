package soundpack

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// JSONValidation is the per-mapping verdict for an on-disk JSON soundpack.
// Every non-empty mapping lands in exactly one of Resolved, Broken or
// Unsafe; empty mappings ("not mapped yet") land in none.
type JSONValidation struct {
	// File is the parsed soundpack with its mapping values as written.
	File JSONSoundpackFile
	// Resolved maps key -> absolute path of an existing regular file
	// inside the soundpack root.
	Resolved map[string]string
	// Broken maps key -> resolved path that is missing or not a regular file.
	Broken map[string]string
	// Unsafe maps key -> reason the value was rejected by the trust
	// boundary (absolute path, `..` segment, or symlink escaping the root).
	Unsafe map[string]string
}

// ValidateJSONSoundpack reads an untrusted soundpack JSON from path and
// checks every mapping through the same trust boundary the runtime loader
// uses (LoadJSONSoundpack). The returned error covers only unreadable or
// malformed files; per-mapping problems are reported in the result and
// summarized by Err.
func ValidateJSONSoundpack(path string) (*JSONValidation, error) {
	spFile, err := PeekJSONSoundpackFromFile(path)
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(path)
	v := &JSONValidation{
		File:     *spFile,
		Resolved: make(map[string]string),
		Broken:   make(map[string]string),
		Unsafe:   make(map[string]string),
	}
	for key, value := range spFile.Mappings {
		if value == "" {
			continue
		}
		resolved, err := validateMappingValue(value, baseDir)
		if err != nil {
			slog.Warn("unsafe soundpack mapping", "key", key, "value", value, "error", err)
			v.Unsafe[key] = err.Error()
			continue
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			slog.Warn("broken soundpack mapping", "key", key, "path", resolved)
			v.Broken[key] = resolved
			continue
		}
		v.Resolved[key] = resolved
	}
	return v, nil
}

// Err returns a non-nil error when any mapping is broken or unsafe.
func (v *JSONValidation) Err() error {
	if len(v.Broken) == 0 && len(v.Unsafe) == 0 {
		return nil
	}
	var parts []string
	if n := len(v.Broken); n > 0 {
		parts = append(parts, fmt.Sprintf("%d broken reference(s) [%s]", n, strings.Join(sortedKeys(v.Broken), ", ")))
	}
	if n := len(v.Unsafe); n > 0 {
		parts = append(parts, fmt.Sprintf("%d unsafe path(s) [%s]", n, strings.Join(sortedKeys(v.Unsafe), ", ")))
	}
	return fmt.Errorf("soundpack %q has %s", v.File.Name, strings.Join(parts, " and "))
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
