package safeio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// renameFailFs wraps an afero.Fs and fails every Rename, so tests can
// prove a failed swap leaves the destination untouched and cleans up
// the temp file.
type renameFailFs struct {
	afero.Fs
}

var errRenameInjected = errors.New("injected rename failure")

func (r renameFailFs) Rename(oldname, newname string) error {
	return errRenameInjected
}

func listDir(t *testing.T, fs afero.Fs, dir string) []string {
	t.Helper()
	entries, err := afero.ReadDir(fs, dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestWriteFileAtomic_CreatesFileWithModeAndNoTempLeftovers(t *testing.T) {
	fs := afero.NewMemMapFs()
	dir := "/data"
	if err := fs.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "out.bin")

	if err := WriteFileAtomic(fs, path, []byte("hello"), 0600, ".out-*.tmp"); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, err := afero.ReadFile(fs, path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
	info, err := fs.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode() & os.ModePerm; perm != 0600 {
		t.Errorf("mode = %o, want 0600", perm)
	}
	if names := listDir(t, fs, dir); len(names) != 1 || names[0] != "out.bin" {
		t.Errorf("directory should hold only out.bin, got %v", names)
	}
}

func TestWriteFileAtomic_ReplacesExistingFile(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/data/out.json"
	if err := afero.WriteFile(fs, path, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomic(fs, path, []byte("new"), 0640, ".out-*.tmp"); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}

	got, _ := afero.ReadFile(fs, path)
	if string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
	info, _ := fs.Stat(path)
	if perm := info.Mode() & os.ModePerm; perm != 0640 {
		t.Errorf("mode = %o, want 0640", perm)
	}
}

func TestWriteFileAtomic_RenameFailureLeavesOriginalAndRemovesTemp(t *testing.T) {
	base := afero.NewMemMapFs()
	path := "/data/out.json"
	if err := afero.WriteFile(base, path, []byte("original"), 0644); err != nil {
		t.Fatal(err)
	}

	err := WriteFileAtomic(renameFailFs{base}, path, []byte("new"), 0644, ".out-*.tmp")
	if !errors.Is(err, errRenameInjected) {
		t.Fatalf("expected wrapped rename error, got %v", err)
	}

	got, _ := afero.ReadFile(base, path)
	if string(got) != "original" {
		t.Errorf("original clobbered: %q", got)
	}
	if names := listDir(t, base, "/data"); len(names) != 1 {
		t.Errorf("temp file not cleaned up: %v", names)
	}
}

func TestWriteFileAtomic_TempCreateFailureReturnsError(t *testing.T) {
	fs := afero.NewReadOnlyFs(afero.NewMemMapFs())
	if err := WriteFileAtomic(fs, "/data/out.json", []byte("x"), 0644, ".out-*.tmp"); err == nil {
		t.Fatal("expected error on read-only filesystem")
	}
}

func TestWriteFileAtomic_RealFilesystem(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "real.json")

	if err := WriteFileAtomic(afero.NewOsFs(), path, []byte(`{"a":1}`), 0644, ".real-*.tmp"); err != nil {
		t.Fatalf("WriteFileAtomic on OsFs: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"a":1}` {
		t.Errorf("content = %q", got)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file left behind: %s", e.Name())
		}
	}
}

func TestWriteJSONFile_PreservesExistingMode(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/cfg/settings.json"
	if err := afero.WriteFile(fs, path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}

	if err := WriteJSONFile(fs, path, map[string]int{"a": 1}, ".settings-*.tmp"); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}

	got, _ := afero.ReadFile(fs, path)
	if string(got) != "{\n  \"a\": 1\n}" {
		t.Errorf("content = %q", got)
	}
	info, _ := fs.Stat(path)
	if perm := info.Mode() & os.ModePerm; perm != 0600 {
		t.Errorf("mode = %o, want preserved 0600", perm)
	}
}

func TestWriteJSONFile_NewFileGets0644AndParentDir(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/fresh/nested/settings.json"

	if err := WriteJSONFile(fs, path, map[string]int{"a": 1}, ".settings-*.tmp"); err != nil {
		t.Fatalf("WriteJSONFile: %v", err)
	}
	info, err := fs.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode() & os.ModePerm; perm != 0644 {
		t.Errorf("mode = %o, want 0644", perm)
	}
}

type backupProbe struct {
	Volume float64 `json:"volume"`
}

func TestBackupJSONFile_CopiesValidSourceWithMode(t *testing.T) {
	fs := afero.NewMemMapFs()
	path := "/cfg/config.json"
	src := []byte(`{"volume":0.5}`)
	if err := afero.WriteFile(fs, path, src, 0600); err != nil {
		t.Fatal(err)
	}

	BackupJSONFile(fs, path, &backupProbe{}, ".config-bak-*.tmp")

	got, err := afero.ReadFile(fs, path+".bak")
	if err != nil {
		t.Fatalf(".bak not written: %v", err)
	}
	if string(got) != string(src) {
		t.Errorf(".bak content = %q, want %q", got, src)
	}
	info, _ := fs.Stat(path + ".bak")
	if perm := info.Mode() & os.ModePerm; perm != 0600 {
		t.Errorf(".bak mode = %o, want 0600", perm)
	}
	if names := listDir(t, fs, "/cfg"); len(names) != 2 {
		t.Errorf("expected only source and .bak, got %v", names)
	}
}

func TestBackupJSONFile_MissingSourceWritesNothing(t *testing.T) {
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/cfg", 0755); err != nil {
		t.Fatal(err)
	}

	BackupJSONFile(fs, "/cfg/config.json", &backupProbe{}, ".config-bak-*.tmp")

	if exists, _ := afero.Exists(fs, "/cfg/config.json.bak"); exists {
		t.Error(".bak created for a missing source")
	}
}

func TestBackupJSONFile_InvalidSourceKeepsExistingBak(t *testing.T) {
	cases := map[string]string{
		"malformed JSON":       `{not json`,
		"probe type mismatch":  `{"volume":"loud"}`,
		"non-object for probe": `[1,2,3]`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			fs := afero.NewMemMapFs()
			path := "/cfg/config.json"
			if err := afero.WriteFile(fs, path, []byte(body), 0644); err != nil {
				t.Fatal(err)
			}
			if err := afero.WriteFile(fs, path+".bak", []byte(`{"volume":1}`), 0644); err != nil {
				t.Fatal(err)
			}

			BackupJSONFile(fs, path, &backupProbe{}, ".config-bak-*.tmp")

			got, _ := afero.ReadFile(fs, path+".bak")
			if string(got) != `{"volume":1}` {
				t.Errorf("last-known-good .bak overwritten: %q", got)
			}
		})
	}
}
