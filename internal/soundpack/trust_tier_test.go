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

func TestLoadEmbeddedPlatformSoundpack_ResolvesRelativePathsAgainstBasePaths(t *testing.T) {
	baseWithoutFile := t.TempDir()
	baseWithFile := t.TempDir()
	soundFile := filepath.Join(baseWithFile, "embedded-relative.wav")
	if err := os.WriteFile(soundFile, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("write wav: %v", err)
	}

	soundpackData := map[string]interface{}{
		"name": "embedded-relative-test",
		"mappings": map[string]string{
			"success/test.wav": "embedded-relative.wav",
		},
	}
	jsonContent, err := json.MarshalIndent(soundpackData, "", "\t")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	mapper, err := LoadEmbeddedPlatformSoundpack(jsonContent, baseWithoutFile, baseWithFile)
	if err != nil {
		t.Fatalf("trusted loader should resolve relative mappings against base paths, got: %v", err)
	}

	got, err := mapper.MapPath("success/test.wav")
	if err != nil {
		t.Fatalf("MapPath: %v", err)
	}
	if len(got) != 1 || filepath.Clean(got[0]) != filepath.Clean(soundFile) {
		t.Errorf("got %v, want [%s]", got, soundFile)
	}
}

// TestValidateMappingValue_RejectsSymlinkEscapingBase asserts the
// trust-boundary validator looks through symlinks. A symlink placed
// inside the soundpack root that points outside the root is the
// path-traversal vector the syntactic Clean+Rel check misses: a
// malicious git-cloned soundpack can preserve a committed symlink.
// EvalSymlinks resolves the target; the resolved path must still be
// under baseDir.
func TestValidateMappingValue_RejectsSymlinkEscapingBase(t *testing.T) {
	base := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret.wav")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	link := filepath.Join(base, "innocuous.wav")
	if err := os.Symlink(outside, link); err != nil {
		// On Windows, symlink creation typically requires admin or
		// developer mode. Skipping is the correct response; the POSIX
		// runs and CI cover the assertion.
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	if _, err := validateMappingValue("innocuous.wav", base); err == nil {
		t.Error("expected validator to reject symlink escaping baseDir")
	}
}

// TestValidateMappingValue_AcceptsSymlinkStayingInsideBase asserts the
// symlink defense is precise: a symlink whose target IS inside the
// soundpack root must still be accepted. Otherwise we'd over-reject
// and break legitimate soundpacks that use symlinks for deduplication.
func TestValidateMappingValue_AcceptsSymlinkStayingInsideBase(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "real.wav")
	if err := os.WriteFile(target, []byte("real"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(base, "alias.wav")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported on this platform: %v", err)
	}

	if _, err := validateMappingValue("alias.wav", base); err != nil {
		t.Errorf("expected validator to accept in-root symlink, got: %v", err)
	}
}

// TestValidateMappingValue_AcceptsMissingFile asserts the symlink
// defense is gated on Lstat success: when the referenced file does not
// exist yet, the syntactic check stands and the validator does NOT
// reject. The missing-file error surfaces at load time (existence
// check), not at validation.
func TestValidateMappingValue_AcceptsMissingFile(t *testing.T) {
	base := t.TempDir()

	// "nope.wav" is not on disk. validateMappingValue should still
	// return a resolved path so the existence check can produce the
	// real "file not found" error.
	if _, err := validateMappingValue("nope.wav", base); err != nil {
		t.Errorf("expected validator to accept non-existent file (syntax check only), got: %v", err)
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
