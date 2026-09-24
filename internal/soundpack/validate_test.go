package soundpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeValidateFixture(t *testing.T, dir string, mappings map[string]string) string {
	t.Helper()
	data, err := json.Marshal(JSONSoundpackFile{Name: "fixture", Mappings: mappings})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "soundpack.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateJSONSoundpackClassifiesMappings(t *testing.T) {
	root := t.TempDir()
	packDir := filepath.Join(root, "pack")
	touch(t, filepath.Join(packDir, "sounds", "ok.wav"))
	touch(t, filepath.Join(root, "outside.wav"))
	if err := os.MkdirAll(filepath.Join(packDir, "adir"), 0755); err != nil {
		t.Fatal(err)
	}
	path := writeValidateFixture(t, packDir, map[string]string{
		"default.wav":              "sounds/ok.wav",
		"loading/bash-start.wav":   "sounds/missing.wav",
		"success/bash-success.wav": "../outside.wav",
		"error/bash-error.wav":     filepath.Join(root, "outside.wav"),
		"system/x.wav":             "adir",
		"interactive/empty.wav":    "",
	})

	v, err := ValidateJSONSoundpack(path)
	if err != nil {
		t.Fatalf("ValidateJSONSoundpack: %v", err)
	}
	if got := v.Resolved["default.wav"]; got != filepath.Join(packDir, "sounds", "ok.wav") {
		t.Errorf("default.wav resolved to %q", got)
	}
	for _, key := range []string{"loading/bash-start.wav", "system/x.wav"} {
		if _, ok := v.Broken[key]; !ok {
			t.Errorf("expected %s to be broken, got broken=%v", key, v.Broken)
		}
	}
	for _, key := range []string{"success/bash-success.wav", "error/bash-error.wav"} {
		if _, ok := v.Unsafe[key]; !ok {
			t.Errorf("expected %s to be unsafe, got unsafe=%v", key, v.Unsafe)
		}
		if _, ok := v.Resolved[key]; ok {
			t.Errorf("unsafe %s must not be resolved", key)
		}
	}
	if _, ok := v.Resolved["interactive/empty.wav"]; ok {
		t.Error("empty mapping must not be resolved")
	}
	if v.Err() == nil {
		t.Fatal("expected Err() to report broken and unsafe mappings")
	}
}

func TestValidateJSONSoundpackCleanPackHasNoError(t *testing.T) {
	packDir := t.TempDir()
	touch(t, filepath.Join(packDir, "ok.wav"))
	path := writeValidateFixture(t, packDir, map[string]string{"default.wav": "ok.wav", "loading/x.wav": ""})

	v, err := ValidateJSONSoundpack(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Err(); err != nil {
		t.Fatalf("clean pack reported error: %v", err)
	}
	if len(v.Resolved) != 1 {
		t.Fatalf("expected one resolved mapping, got %v", v.Resolved)
	}
}

func TestValidateJSONSoundpackRejectsMalformedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{nope"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateJSONSoundpack(path); err == nil {
		t.Fatal("expected parse error")
	}
}

// TestValidateAndLoaderAgreeOnAbsolutePaths pins issue #85: validate must
// reject every absolute mapping value the runtime loader rejects, with the
// same reason, for POSIX, Windows drive-letter, UNC and rooted forms on
// every host OS. The host-native absolute path points at a real file so the
// rejection cannot be a missing-file verdict in disguise.
func TestValidateAndLoaderAgreeOnAbsolutePaths(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real.wav")
	touch(t, real)

	values := map[string]string{
		"host-native":      real,
		"posix":            "/etc/sounds/x.wav",
		"drive-backslash":  `C:\Windows\Media\chimes.wav`,
		"drive-slash":      "C:/Windows/Media/chimes.wav",
		"drive-relative":   "c:chimes.wav",
		"unc-backslash":    `\server\share\x.wav`,
		"unc-slash":        "//server/share/x.wav",
		"rooted-backslash": `\Windows\Media\chimes.wav`,
	}
	for name, value := range values {
		t.Run(name, func(t *testing.T) {
			packDir := t.TempDir()
			const key = "loading/git-start.wav"
			path := writeValidateFixture(t, packDir, map[string]string{key: value})

			v, err := ValidateJSONSoundpack(path)
			if err != nil {
				t.Fatalf("ValidateJSONSoundpack: %v", err)
			}
			reason, ok := v.Unsafe[key]
			if !ok {
				t.Fatalf("validate accepted %q (resolved=%v broken=%v)", value, v.Resolved, v.Broken)
			}
			if v.Err() == nil {
				t.Error("Err() must report the unsafe mapping")
			}

			_, loadErr := LoadJSONSoundpack(path)
			if loadErr == nil {
				t.Fatalf("loader accepted %q that validate rejected", value)
			}
			if !strings.Contains(loadErr.Error(), reason) {
				t.Errorf("validate reason %q and loader error %q disagree", reason, loadErr)
			}
			if !strings.Contains(reason, "absolute paths not allowed") {
				t.Errorf("reason %q should say absolute paths are not allowed", reason)
			}
		})
	}
}
