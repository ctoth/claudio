package soundpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/safeio"
)

// TestPeekJSONSoundpackFromBytes_AppliesBasics asserts that peek
// rejects missing name and empty mappings (same basics as the loader),
// but skips path-syntax validation.
func TestPeekJSONSoundpackFromBytes_AppliesBasics(t *testing.T) {
	t.Run("rejects missing name", func(t *testing.T) {
		_, err := PeekJSONSoundpackFromBytes([]byte(`{"mappings":{"a":"b"}}`))
		if err == nil || !strings.Contains(err.Error(), "missing required 'name'") {
			t.Fatalf("got %v, want missing-name error", err)
		}
	})

	t.Run("rejects empty mappings", func(t *testing.T) {
		_, err := PeekJSONSoundpackFromBytes([]byte(`{"name":"x","mappings":{}}`))
		if err == nil || !strings.Contains(err.Error(), "missing or empty 'mappings'") {
			t.Fatalf("got %v, want empty-mappings error", err)
		}
	})

	t.Run("accepts absolute path values", func(t *testing.T) {
		// Path validation is INTENTIONALLY skipped by peek; the caller
		// only wants metadata, not a resolved/usable mapper.
		_, err := PeekJSONSoundpackFromBytes([]byte(`{"name":"x","mappings":{"a":"/etc/passwd"}}`))
		if err != nil {
			t.Fatalf("peek should not reject abs paths, got: %v", err)
		}
	})

	t.Run("rejects too-many mappings", func(t *testing.T) {
		var sb strings.Builder
		sb.WriteString(`{"name":"x","mappings":{`)
		for i := 0; i <= MaxSoundpackMappings; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			fmt.Fprintf(&sb, `"k%d":"v"`, i)
		}
		sb.WriteString(`}}`)

		_, err := PeekJSONSoundpackFromBytes([]byte(sb.String()))
		if err == nil || !strings.Contains(err.Error(), "limit of") {
			t.Fatalf("got %v, want mappings-cap error", err)
		}
	})
}

// TestPeekJSONSoundpackFromFile_AppliesSizeCap asserts the file peek
// applies the safeio cap.
func TestPeekJSONSoundpackFromFile_AppliesSizeCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "huge.json")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := f.Write([]byte(`{"name":"big","mappings":{"a":"`)); err != nil {
		t.Fatalf("write: %v", err)
	}
	body := make([]byte, 64*1024)
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

	_, err = PeekJSONSoundpackFromFile(path)
	if err == nil || !strings.Contains(err.Error(), "soundpack JSON") {
		t.Fatalf("got %v, want size-cap error", err)
	}
}

// TestPeekJSONSoundpackMetadataFromFile_AcceptsEmptyMappings asserts
// the permissive metadata peek allows empty-mappings packs (e.g.
// install scaffolds).
func TestPeekJSONSoundpackMetadataFromFile_AcceptsEmptyMappings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scaffold.json")
	if err := os.WriteFile(path, []byte(`{"name":"scaffold","mappings":{}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	sp, err := PeekJSONSoundpackMetadataFromFile(path)
	if err != nil {
		t.Fatalf("expected metadata peek to accept empty mappings, got: %v", err)
	}
	if sp.Name != "scaffold" {
		t.Errorf("got name %q, want %q", sp.Name, "scaffold")
	}
}

// TestPeekJSONSoundpackMetadataFromFile_EnforcesMappingsCap asserts the
// metadata peek still applies the mappings count cap (DoS guard).
func TestPeekJSONSoundpackMetadataFromFile_EnforcesMappingsCap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "too-many.json")

	var sb strings.Builder
	sb.WriteString(`{"name":"x","mappings":{`)
	for i := 0; i <= MaxSoundpackMappings; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `"k%d":"v"`, i)
	}
	sb.WriteString(`}}`)
	if err := os.WriteFile(path, []byte(sb.String()), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := PeekJSONSoundpackMetadataFromFile(path)
	if err == nil || !strings.Contains(err.Error(), "limit of") {
		t.Fatalf("got %v, want mappings cap error", err)
	}
}
