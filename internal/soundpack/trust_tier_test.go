package soundpack

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadJSONSoundpack_RejectsAbsolutePath asserts the untrusted on-disk
// loader rejects a mapping value that is an absolute path. Even if the
// path points to a real file, the new trust tier refuses to dereference
// it.
func TestLoadJSONSoundpack_RejectsAbsolutePath(t *testing.T) {
	tempDir := t.TempDir()

	// Create a real file the value could point to.
	realFile := filepath.Join(tempDir, "real.wav")
	if err := os.WriteFile(realFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("create real file: %v", err)
	}

	soundpackData := map[string]interface{}{
		"name": "abs-test",
		"mappings": map[string]string{
			"success/test.wav": realFile, // absolute path
		},
	}
	jsonContent, err := json.MarshalIndent(soundpackData, "", "\t")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonPath := filepath.Join(tempDir, "pack.json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	_, err = LoadJSONSoundpack(jsonPath)
	if err == nil {
		t.Fatalf("expected absolute path to be rejected")
	}
	if !strings.Contains(err.Error(), "absolute paths not allowed") {
		t.Errorf("error %q should contain 'absolute paths not allowed'", err.Error())
	}
}

// TestLoadJSONSoundpack_RejectsDotDotTraversal asserts the untrusted
// loader rejects any mapping value containing a `..` segment, even if
// the cleaned result would still be under baseDir.
func TestLoadJSONSoundpack_RejectsDotDotTraversal(t *testing.T) {
	tempDir := t.TempDir()

	jsonContent := []byte(`{
		"name": "traversal-test",
		"mappings": {
			"success/test.wav": "sub/../../escape.wav"
		}
	}`)

	jsonPath := filepath.Join(tempDir, "pack.json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	_, err := LoadJSONSoundpack(jsonPath)
	if err == nil {
		t.Fatalf("expected traversal to be rejected")
	}
	if !strings.Contains(err.Error(), "path traversal not allowed") {
		t.Errorf("error %q should contain 'path traversal not allowed'", err.Error())
	}
}

// TestLoadJSONSoundpack_AcceptsRelativeUnderBase asserts that a relative
// mapping value is resolved under the soundpack directory and loaded
// successfully when the referenced file exists.
func TestLoadJSONSoundpack_AcceptsRelativeUnderBase(t *testing.T) {
	tempDir := t.TempDir()

	soundFile := filepath.Join(tempDir, "sounds", "x.wav")
	if err := os.MkdirAll(filepath.Dir(soundFile), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(soundFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	jsonContent := []byte(`{
		"name": "relative-test",
		"mappings": {
			"success/test.wav": "sounds/x.wav"
		}
	}`)
	jsonPath := filepath.Join(tempDir, "pack.json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	mapper, err := LoadJSONSoundpack(jsonPath)
	if err != nil {
		t.Fatalf("expected relative-under-base to load, got: %v", err)
	}

	got, err := mapper.MapPath("success/test.wav")
	if err != nil {
		t.Fatalf("MapPath: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d candidates, want 1: %v", len(got), got)
	}
	if filepath.Clean(got[0]) != filepath.Clean(soundFile) {
		t.Errorf("got resolved %q, want %q", got[0], soundFile)
	}
}

// TestLoadEmbeddedPlatformSoundpack_AcceptsAbsolutePaths asserts the
// trusted entry point does NOT reject absolute paths. This is the
// hard-stop test from the brief: shipped platform JSONs use absolute
// system paths like /System/Library/Sounds/Purr.aiff and must keep
// loading.
func TestLoadEmbeddedPlatformSoundpack_AcceptsAbsolutePaths(t *testing.T) {
	tempDir := t.TempDir()
	soundFile := filepath.Join(tempDir, "embedded.wav")
	if err := os.WriteFile(soundFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	soundpackData := map[string]interface{}{
		"name": "embedded-test",
		"mappings": map[string]string{
			"success/test.wav": soundFile, // absolute, treated as-is
		},
	}
	jsonContent, err := json.MarshalIndent(soundpackData, "", "\t")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	mapper, err := LoadEmbeddedPlatformSoundpack(jsonContent)
	if err != nil {
		t.Fatalf("trusted loader should accept absolute paths, got: %v", err)
	}

	got, err := mapper.MapPath("success/test.wav")
	if err != nil {
		t.Fatalf("MapPath: %v", err)
	}
	if len(got) != 1 || filepath.Clean(got[0]) != filepath.Clean(soundFile) {
		t.Errorf("got %v, want [%s]", got, soundFile)
	}
}

// TestLoadJSONSoundpack_RejectsTooManyMappings asserts the 10K cap
// fires before existence checks would run.
func TestLoadJSONSoundpack_RejectsTooManyMappings(t *testing.T) {
	tempDir := t.TempDir()

	mappings := make(map[string]string, MaxSoundpackMappings+1)
	for i := 0; i <= MaxSoundpackMappings; i++ {
		mappings[fmt.Sprintf("key-%d", i)] = "x.wav"
	}
	soundpackData := map[string]interface{}{
		"name":     "too-many",
		"mappings": mappings,
	}
	jsonContent, err := json.MarshalIndent(soundpackData, "", "\t")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	jsonPath := filepath.Join(tempDir, "pack.json")
	if err := os.WriteFile(jsonPath, jsonContent, 0644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	_, err = LoadJSONSoundpack(jsonPath)
	if err == nil {
		t.Fatalf("expected mappings cap to fire")
	}
	wantNeedle := fmt.Sprintf("limit of %d entries", MaxSoundpackMappings)
	if !strings.Contains(err.Error(), wantNeedle) {
		t.Errorf("error %q should mention %q", err.Error(), wantNeedle)
	}
}

// TestLoadEmbeddedPlatformSoundpack_RejectsTooManyMappings asserts the
// 10K cap is also enforced on the trusted variant — it's a DoS guard,
// not a trust check.
func TestLoadEmbeddedPlatformSoundpack_RejectsTooManyMappings(t *testing.T) {
	mappings := make(map[string]string, MaxSoundpackMappings+1)
	for i := 0; i <= MaxSoundpackMappings; i++ {
		mappings[fmt.Sprintf("key-%d", i)] = "/tmp/x.wav"
	}
	soundpackData := map[string]interface{}{
		"name":     "too-many-trusted",
		"mappings": mappings,
	}
	jsonContent, err := json.MarshalIndent(soundpackData, "", "\t")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	_, err = LoadEmbeddedPlatformSoundpack(jsonContent)
	if err == nil {
		t.Fatalf("expected mappings cap to fire on trusted path too")
	}
	wantNeedle := fmt.Sprintf("limit of %d entries", MaxSoundpackMappings)
	if !strings.Contains(err.Error(), wantNeedle) {
		t.Errorf("error %q should mention %q", err.Error(), wantNeedle)
	}
}
