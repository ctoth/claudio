package soundpack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryResolverResolvesCanonicalWAVKeyToMP3File(t *testing.T) {
	tempDir := t.TempDir()
	successDir := filepath.Join(tempDir, "success")
	if err := os.MkdirAll(successDir, 0755); err != nil {
		t.Fatalf("failed to create soundpack dir: %v", err)
	}

	soundFile := filepath.Join(successDir, "bash-success.mp3")
	if err := os.WriteFile(soundFile, []byte("fake mp3 data"), 0644); err != nil {
		t.Fatalf("failed to create test sound file: %v", err)
	}

	mapper := NewDirectoryMapper("test", []string{tempDir})
	resolver := NewSoundpackResolver(mapper)

	path, err := resolver.ResolveSound("success/bash-success.wav")
	if err != nil {
		t.Fatalf("expected canonical wav key to resolve to mp3 file, got error: %v", err)
	}

	if path != soundFile {
		t.Fatalf("expected path %s, got %s", soundFile, path)
	}
}

func TestDirectoryResolverPrefersExactWAVFileOverAlternateFormats(t *testing.T) {
	tempDir := t.TempDir()
	successDir := filepath.Join(tempDir, "success")
	if err := os.MkdirAll(successDir, 0755); err != nil {
		t.Fatalf("failed to create soundpack dir: %v", err)
	}

	wavFile := filepath.Join(successDir, "bash-success.wav")
	if err := os.WriteFile(wavFile, []byte("fake wav data"), 0644); err != nil {
		t.Fatalf("failed to create wav test sound file: %v", err)
	}

	mp3File := filepath.Join(successDir, "bash-success.mp3")
	if err := os.WriteFile(mp3File, []byte("fake mp3 data"), 0644); err != nil {
		t.Fatalf("failed to create mp3 test sound file: %v", err)
	}

	mapper := NewDirectoryMapper("test", []string{tempDir})
	resolver := NewSoundpackResolver(mapper)

	path, err := resolver.ResolveSound("success/bash-success.wav")
	if err != nil {
		t.Fatalf("expected canonical wav key to resolve, got error: %v", err)
	}

	if path != wavFile {
		t.Fatalf("expected exact wav path %s, got %s", wavFile, path)
	}
}
