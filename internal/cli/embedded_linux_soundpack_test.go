package cli

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/hooks"
	"claudio.click/internal/sounds"
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

	// These are the category-level keys the sound mapper's fallback chains
	// emit (<category>/<category>.wav) plus the terminal default — i.e. the
	// keys linux.json must map for events to resolve.
	keys := []string{
		"success/success.wav",
		"error/error.wav",
		"loading/loading.wav",
		"interactive/interactive.wav",
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

// TestEmbeddedLinuxSoundpackSelectsDistinctCuesPerCategory drives the sound
// mapper — the same selection logic the integration suite's
// TestEndToEndSoundPathTracking exercises — against the embedded Linux pack,
// with the pack forced by name so it runs on any host (a developer box and
// WSL are detected as WSL and would otherwise use wsl.json). It guards the
// keys in linux.json: they must be the category-level paths the fallback
// chains actually emit (<category>/<category>.wav), or every event collapses
// to the terminal default.wav and Linux users hear one sound for everything.
func TestEmbeddedLinuxSoundpackSelectsDistinctCuesPerCategory(t *testing.T) {
	testenv.IsolateXDG(t)

	mapper, err := loadEmbeddedPlatformSoundpack("embedded:linux.json")
	if err != nil {
		t.Fatalf("loadEmbeddedPlatformSoundpack(embedded:linux.json) failed: %v", err)
	}
	soundMapper := sounds.NewSoundMapperWithResolver(soundpack.NewSoundpackResolver(mapper))

	success := json.RawMessage(`{"stdout":"ok","stderr":"","interrupted":false}`)
	failure := json.RawMessage(`{"stdout":"","stderr":"boom","interrupted":false}`)
	events := []hooks.HookEvent{
		{EventName: "PostToolUse", ToolName: stringPtr("Edit"), ToolResponse: &success},
		{EventName: "PostToolUse", ToolName: stringPtr("Read"), ToolResponse: &failure},
		{EventName: "PreToolUse", ToolName: stringPtr("Write"), ToolResponse: &success},
		{EventName: "UserPromptSubmit"},
	}

	selected := make(map[string]struct{})
	for _, e := range events {
		result := soundMapper.MapSound(context.Background(), e.GetContext())
		selected[result.SelectedPath] = struct{}{}
	}

	if len(selected) < 2 {
		paths := make([]string, 0, len(selected))
		for p := range selected {
			paths = append(paths, p)
		}
		t.Errorf("expected distinct cues across success/error/loading/interactive events, got %d: %v",
			len(selected), paths)
	}
}
