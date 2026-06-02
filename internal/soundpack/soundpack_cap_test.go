package soundpack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/safeio"
)

// TestLoadJSONSoundpack_RejectsOversizedFile writes a soundpack JSON file
// that exceeds MaxSoundpackJSONBytes by 1 byte and asserts the loader
// reports the cap error.
func TestLoadJSONSoundpack_RejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.json")

	// Write MaxSoundpackJSONBytes+1 bytes. Use a sparse-ish payload (one
	// big string) so the JSON parser would also reject — but the cap fires
	// first, so the test asserts that path.
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.Write([]byte(`{"name":"big","mappings":{"a":"`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Fill body with 'a' bytes until just over the cap.
	const chunk = 64 * 1024
	body := make([]byte, chunk)
	for i := range body {
		body[i] = 'a'
	}
	written := int64(len(`{"name":"big","mappings":{"a":"`))
	for written <= safeio.MaxSoundpackJSONBytes {
		n, err := f.Write(body)
		if err != nil {
			t.Fatalf("write body: %v", err)
		}
		written += int64(n)
	}
	if _, err := f.Write([]byte(`"}}`)); err != nil {
		t.Fatalf("write tail: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, err = LoadJSONSoundpack(path)
	if err == nil {
		t.Fatalf("expected error loading oversized soundpack JSON")
	}
	if !strings.Contains(err.Error(), "soundpack JSON") {
		t.Errorf("error %q should reference 'soundpack JSON' kind from safeio cap", err.Error())
	}
}
