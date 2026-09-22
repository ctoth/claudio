package soundpack

import (
	"os"
	"path/filepath"
	"testing"
)

// writePack writes a soundpack.json plus the named sound files into a temp
// directory and returns the manifest path.
func writePack(t *testing.T, manifest string, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("RIFF"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(dir, "soundpack.json")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mapped(t *testing.T, m PathMapper, key string) bool {
	t.Helper()
	paths, err := m.MapPath(key)
	return err == nil && len(paths) > 0
}

// Empty values are "not mapped yet" (what `soundpack init` scaffolds and what
// `soundpack validate` reports as unmapped). They must not fail the load.
func TestLoadJSONSoundpackSkipsEmptyMappingValues(t *testing.T) {
	path := writePack(t, `{"name":"p","mappings":{
		"success/success.wav":"s.wav",
		"error/error.wav":""
	}}`, "s.wav")

	m, err := LoadJSONSoundpack(path)
	if err != nil {
		t.Fatalf("LoadJSONSoundpack with an empty value failed: %v", err)
	}
	if !mapped(t, m, "success/success.wav") {
		t.Error("non-empty mapping was lost")
	}
	if mapped(t, m, "error/error.wav") {
		t.Error("empty mapping should be treated as unmapped")
	}
}

// A mapping whose file is missing must not take the whole pack down: the
// runtime loader drops that entry so its fallback chain continues.
func TestLoadJSONSoundpackDropsMissingFiles(t *testing.T) {
	path := writePack(t, `{"name":"p","mappings":{
		"success/success.wav":"s.wav",
		"error/error.wav":"missing.wav"
	}}`, "s.wav")

	m, err := LoadJSONSoundpack(path)
	if err != nil {
		t.Fatalf("one missing file failed the whole pack: %v", err)
	}
	if !mapped(t, m, "success/success.wav") {
		t.Error("valid mapping was lost")
	}
	if mapped(t, m, "error/error.wav") {
		t.Error("mapping to a missing file should be dropped")
	}
}

func TestLoadEmbeddedPlatformSoundpackDropsMissingFiles(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.wav")
	if err := os.WriteFile(present, []byte("RIFF"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"name":"platform","mappings":{
		"success/success.wav":` + jsonString(present) + `,
		"error/error.wav":` + jsonString(filepath.Join(dir, "absent.wav")) + `
	}}`)

	m, err := LoadEmbeddedPlatformSoundpack(data)
	if err != nil {
		t.Fatalf("missing system sound failed the platform pack: %v", err)
	}
	if !mapped(t, m, "success/success.wav") || mapped(t, m, "error/error.wav") {
		t.Error("expected present mapping kept and absent mapping dropped")
	}
}

// Security checks stay fatal: leniency is only for empty and missing entries.
func TestLoadJSONSoundpackStillRejectsTraversal(t *testing.T) {
	path := writePack(t, `{"name":"p","mappings":{
		"success/success.wav":"s.wav",
		"error/error.wav":"../outside.wav"
	}}`, "s.wav")
	if _, err := LoadJSONSoundpack(path); err == nil {
		t.Fatal("path traversal must still fail the load")
	}
}

func jsonString(s string) string {
	b := []byte{'"'}
	for _, r := range filepath.ToSlash(s) {
		if r == '"' || r == '\\' {
			b = append(b, '\\')
		}
		b = append(b, string(r)...)
	}
	return string(append(b, '"'))
}
