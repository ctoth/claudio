package tracking

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
)

// MockSoundpackResolver implements a simple mock for testing
type MockSoundpackResolver struct {
	mappings map[string]string // logical -> physical path mappings
}

func (m *MockSoundpackResolver) ResolveSound(relativePath string) (string, error) {
	if physical, exists := m.mappings[relativePath]; exists {
		return physical, nil
	}
	return "", fmt.Errorf("sound not found: %s", relativePath)
}

func (m *MockSoundpackResolver) ResolveSoundWithFallback(paths []string, opts ...soundpack.ResolveOption) (string, error) {
	for _, path := range paths {
		if resolved, err := m.ResolveSound(path); err == nil {
			return resolved, nil
		}
	}
	return "", fmt.Errorf("no sounds found in fallback chain")
}

func (m *MockSoundpackResolver) GetName() string { return "mock" }
func (m *MockSoundpackResolver) GetType() string { return "mock" }

// TestSoundChecker_WithResolver verifies NewSoundCheckerWithResolver correctly
// resolves logical paths to physical paths before checking existence on disk.
//
// SoundChecker is on the deletion list for the next commit (Chunk 14c); this
// test pins the existing behavior during the migration window.
func TestSoundChecker_WithResolver(t *testing.T) {
	tempDir := t.TempDir()
	bashSuccessFile := filepath.Join(tempDir, "bash-success.wav")
	defaultFile := filepath.Join(tempDir, "default.wav")

	err := os.WriteFile(bashSuccessFile, []byte("test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(defaultFile, []byte("test"), 0644)
	require.NoError(t, err)

	resolver := &MockSoundpackResolver{
		mappings: map[string]string{
			"success/bash-success.wav": bashSuccessFile,
			"default.wav":              defaultFile,
		},
	}

	checker := NewSoundCheckerWithResolver(resolver)
	context := &hooks.EventContext{Category: hooks.Success, ToolName: "bash"}

	logicalPaths := []string{
		"success/bash-success.wav",
		"success/tool-complete.wav",
		"default.wav",
	}

	results := checker.CheckPaths(context, "posttool", logicalPaths)

	if !results[0] {
		t.Errorf("Expected first path to exist after resolution, got false")
	}
	if !results[2] {
		t.Errorf("Expected default.wav to exist after resolution, got false")
	}
	if results[1] {
		t.Errorf("Expected second path to not exist (no mapping), got true")
	}
}

// TestLookupBuffer_CollectsObserverEvents asserts that LookupBuffer.Observer()
// returns a soundpack.PathObserver that appends one Lookup per callback in
// order, preserving the 1-based sequence and exists flag.
func TestLookupBuffer_CollectsObserverEvents(t *testing.T) {
	buf := NewLookupBuffer()
	obs := buf.Observer()

	// Feed three synthetic observer events as if the resolver had fired them.
	obs("loading/bash-thinking.wav", 1, false)
	obs("loading/bash-start.wav", 2, true)
	obs("loading/loading.wav", 3, false)

	got := buf.Lookups()
	if len(got) != 3 {
		t.Fatalf("Lookups() returned %d entries, want 3", len(got))
	}

	want := []Lookup{
		{Path: "loading/bash-thinking.wav", Sequence: 1, Found: false},
		{Path: "loading/bash-start.wav", Sequence: 2, Found: true},
		{Path: "loading/loading.wav", Sequence: 3, Found: false},
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("Lookups()[%d] = %+v, want %+v", i, got[i], w)
		}
	}
}

// TestLookupBuffer_EmptyBeforeObservation asserts a fresh buffer has zero
// lookups recorded. Sanity check on initial state.
func TestLookupBuffer_EmptyBeforeObservation(t *testing.T) {
	buf := NewLookupBuffer()
	if got := buf.Lookups(); len(got) != 0 {
		t.Errorf("fresh LookupBuffer has %d lookups, want 0", len(got))
	}
}

// TestLookupBuffer_ImplementsPathObserver pins that Observer() returns a
// value assignable to soundpack.PathObserver — the structural integration
// point with soundpack.WithObserver.
func TestLookupBuffer_ImplementsPathObserver(t *testing.T) {
	buf := NewLookupBuffer()
	var _ soundpack.PathObserver = buf.Observer()
}

// TestLookupBuffer_IntegratesWithSoundpackResolver wires a LookupBuffer
// through soundpack.WithObserver against the real UnifiedSoundpackResolver
// to assert the end-to-end observer→buffer flow records every candidate
// the resolver walked.
func TestLookupBuffer_IntegratesWithSoundpackResolver(t *testing.T) {
	tempDir := t.TempDir()
	soundpackDir := filepath.Join(tempDir, "success")
	require.NoError(t, os.MkdirAll(soundpackDir, 0755))
	// Only the 2nd candidate will resolve.
	present := filepath.Join(soundpackDir, "present.wav")
	require.NoError(t, os.WriteFile(present, []byte("data"), 0644))

	mapper := soundpack.NewDirectoryMapper("test", []string{tempDir})
	resolver := soundpack.NewSoundpackResolver(mapper)

	buf := NewLookupBuffer()
	winner, err := resolver.ResolveSoundWithFallback(
		[]string{"success/missing.wav", "success/present.wav"},
		soundpack.WithObserver(buf.Observer()),
	)
	require.NoError(t, err)
	if winner != present {
		t.Errorf("winner=%q want %q", winner, present)
	}

	got := buf.Lookups()
	if len(got) != 2 {
		t.Fatalf("Lookups returned %d entries, want 2", len(got))
	}
	if got[0].Path != "success/missing.wav" || got[0].Found {
		t.Errorf("Lookups[0]=%+v, want {missing,false}", got[0])
	}
	if got[1].Path != "success/present.wav" || !got[1].Found {
		t.Errorf("Lookups[1]=%+v, want {present,true}", got[1])
	}
}
