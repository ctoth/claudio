package soundpack

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// captureWarnings routes the default slog logger into a JSON buffer for the
// test and returns a func that decodes the WARN-or-higher records logged
// since. Callers must not run in parallel: the default logger is global.
func captureWarnings(t *testing.T) func() []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	return func() []map[string]any {
		var records []map[string]any
		dec := json.NewDecoder(&buf)
		for dec.More() {
			var rec map[string]any
			if err := dec.Decode(&rec); err != nil {
				t.Fatalf("decode log record: %v", err)
			}
			records = append(records, rec)
		}
		return records
	}
}

// A failed resolution must tell the user where claudio looked: one WARN
// record naming the soundpack and every absolute candidate path checked
// (issue #84 was undiagnosable because the log carried only counts).
func TestResolveSoundWithFallbackFailureLogsCandidatesOnce(t *testing.T) {
	baseA := filepath.Join(t.TempDir(), "pack-a")
	baseB := filepath.Join(t.TempDir(), "pack-b")
	resolver := NewSoundpackResolver(NewDirectoryMapper("diag-pack", []string{baseA, baseB}))
	warnings := captureWarnings(t)

	_, err := resolver.ResolveSoundWithFallback([]string{"loading/git-start.wav", "default.wav"})
	if !IsFileNotFoundError(err) {
		t.Fatalf("expected FileNotFoundError, got %v", err)
	}

	records := warnings()
	if len(records) != 1 {
		t.Fatalf("expected exactly one WARN record for the failed resolution, got %d: %v", len(records), records)
	}
	rec := records[0]
	if rec["soundpack"] != "diag-pack" {
		t.Errorf("soundpack = %v, want diag-pack", rec["soundpack"])
	}
	if rec["mapper_type"] != "directory" {
		t.Errorf("mapper_type = %v, want directory", rec["mapper_type"])
	}
	raw, ok := rec["candidates"].([]any)
	if !ok {
		t.Fatalf("WARN record has no candidates list: %v", rec)
	}
	var got []string
	for _, c := range raw {
		got = append(got, c.(string))
	}
	want := []string{
		filepath.Join(baseA, "loading/git-start.wav"),
		filepath.Join(baseB, "loading/git-start.wav"),
		filepath.Join(baseA, "default.wav"),
		filepath.Join(baseB, "default.wav"),
	}
	if !slices.Equal(got, want) {
		t.Errorf("candidates = %v, want %v", got, want)
	}
}

// Misses on the way to a hit are the normal fallback walk, not a problem:
// a successful resolution logs no warnings.
func TestResolveSoundWithFallbackSuccessLogsNoWarnings(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "default.wav"), []byte("wav"), 0o644); err != nil {
		t.Fatal(err)
	}
	resolver := NewSoundpackResolver(NewDirectoryMapper("quiet-pack", []string{base}))
	warnings := captureWarnings(t)

	if _, err := resolver.ResolveSoundWithFallback([]string{"loading/git-start.wav", "default.wav"}); err != nil {
		t.Fatalf("expected default.wav to resolve: %v", err)
	}
	if records := warnings(); len(records) != 0 {
		t.Errorf("expected no WARN records for a successful fallback, got %v", records)
	}
}
