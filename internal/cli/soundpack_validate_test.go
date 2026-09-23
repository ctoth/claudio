package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/testutil/wavfixture"
)

func TestDirectoryCoverageCountsOnlyResolvableKnownKeys(t *testing.T) {
	packDir := filepath.Join(t.TempDir(), "pack")
	wavfixture.Write(t, filepath.Join(packDir, "default.aif"))
	wavfixture.Write(t, filepath.Join(packDir, "success", "bash-success.mp3"))
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatal(err)
	}
	// More stray audio files than there are known keys.
	for i := range len(keys) + 10 {
		wavfixture.Write(t, filepath.Join(packDir, "extra", fmt.Sprintf("stray-%d.wav", i)))
	}

	result, err := validateDirectorySoundpack(packDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.MappedKeys) > len(result.AllKeys) {
		t.Fatalf("coverage exceeds 100%%: %d/%d", len(result.MappedKeys), len(result.AllKeys))
	}
	known := make(map[string]bool, len(keys))
	for _, k := range keys {
		known[k] = true
	}
	for key := range result.MappedKeys {
		if !known[key] {
			t.Errorf("unknown key %q counted as coverage", key)
		}
	}
	for _, key := range []string{"default.wav", "success/bash-success.wav"} {
		if _, ok := result.MappedKeys[key]; !ok {
			t.Errorf("expected %s to be covered (runtime resolves it), got %v", key, result.MappedKeys)
		}
	}
	if len(result.MappedKeys) != 2 {
		t.Errorf("expected exactly 2 covered keys, got %d: %v", len(result.MappedKeys), result.MappedKeys)
	}
}

func TestJSONValidateAcceptsAifWithoutFormatWarning(t *testing.T) {
	dir := t.TempDir()
	wavfixture.Write(t, filepath.Join(dir, "tone.aif"))
	path := writeJSONPack(t, dir, map[string]string{"default.wav": "tone.aif"})
	result, err := validateJSONSoundpackFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.FormatWarnings) != 0 {
		t.Fatalf("unexpected format warning for .aif: %v", result.FormatWarnings)
	}
}

func TestCountAudioFilesCountsAif(t *testing.T) {
	dir := t.TempDir()
	wavfixture.Write(t, filepath.Join(dir, "a.aif"))
	wavfixture.Write(t, filepath.Join(dir, "b.wav"))
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := countAudioFiles(dir); got != 2 {
		t.Fatalf("countAudioFiles = %d, want 2", got)
	}
}

func TestJSONCoverageIgnoresUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	wavfixture.Write(t, filepath.Join(dir, "tone.wav"))
	path := writeJSONPack(t, dir, map[string]string{"default.wav": "tone.wav", "custom/extra.wav": "tone.wav"})
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatal(err)
	}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio", "soundpack", "validate", path}, nil, stdout, stderr); code != 0 {
		t.Fatalf("validate failed: stdout=%q stderr=%q", stdout, stderr)
	}
	if want := fmt.Sprintf("1/%d", len(keys)); !strings.Contains(stdout.String(), want) {
		t.Fatalf("expected coverage %s, got %q", want, stdout)
	}
}
