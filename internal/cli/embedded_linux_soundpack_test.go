package cli

import (
	"os"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/soundpack"
)

// TestEmbeddedLinuxSoundpackResolves proves the native-Linux default pack
// resolves its default tones out of the box on any host.
//
// Why load it explicitly instead of letting platform detection pick it: a
// developer box (and CI's only Linux-capable local shell, WSL) is detected as
// WSL and would use wsl.json, which points at /mnt/c system sounds. linux.json
// is the only pack whose mappings are bare filenames backed by tones embedded
// in the binary, so it is the only one that exercises the extraction path. The
// real-world native-Linux selection is covered by CI's ubuntu job.
//
// loadEmbeddedPlatformSoundpack runs validateMappingFilesExist (an os.Stat on
// every mapping) as part of the load, so a non-error return already proves the
// embedded tones were materialized to disk. The explicit ResolveSound calls
// below additionally pin the per-category resolution the sound mapper relies
// on, and that the cues are distinct files (not all collapsed to default.wav).
func TestEmbeddedLinuxSoundpackResolves(t *testing.T) {
	testenv.IsolateXDG(t)

	mapper, err := loadEmbeddedPlatformSoundpack("embedded:linux.json")
	if err != nil {
		t.Fatalf("loadEmbeddedPlatformSoundpack(embedded:linux.json) failed: %v", err)
	}

	resolver := soundpack.NewSoundpackResolver(mapper)

	keys := []string{
		"success/default",
		"error/default",
		"loading/default",
		"interactive/default",
		"default.wav",
	}

	resolved := make(map[string]string, len(keys))
	for _, key := range keys {
		path, err := resolver.ResolveSound(key)
		if err != nil {
			t.Errorf("ResolveSound(%q) failed: %v", key, err)
			continue
		}
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("ResolveSound(%q) returned %q which does not exist: %v", key, path, statErr)
			continue
		}
		resolved[key] = path
	}

	distinct := make(map[string]struct{}, len(resolved))
	for _, p := range resolved {
		distinct[p] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Errorf("expected the default cues to resolve to distinct files, got %d unique path(s): %v",
			len(distinct), resolved)
	}
}
