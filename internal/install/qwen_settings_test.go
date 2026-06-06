package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindQwenSettingsPathsGlobalScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	paths, err := FindQwenSettingsPaths("global")
	if err != nil {
		t.Fatalf("FindQwenSettingsPaths returned error: %v", err)
	}
	want := filepath.Join(home, ".qwen", "settings.json")
	if len(paths) == 0 || paths[0] != want {
		t.Fatalf("first global Qwen path = %v, want %q", paths, want)
	}
}

func TestFindQwenSettingsPathsProjectScope(t *testing.T) {
	paths, err := FindQwenSettingsPaths("project")
	if err != nil {
		t.Fatalf("FindQwenSettingsPaths returned error: %v", err)
	}
	want := []string{
		filepath.Join(".", ".qwen", "settings.json"),
		filepath.Join(".qwen", "settings.json"),
	}
	if len(paths) != len(want) {
		t.Fatalf("project Qwen path count = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("project Qwen path %d = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestFindBestQwenPathPrefersExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	settingsPath := filepath.Join(home, ".qwen", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindBestQwenPath("global")
	if err != nil {
		t.Fatalf("FindBestQwenPath returned error: %v", err)
	}
	if got != settingsPath {
		t.Errorf("FindBestQwenPath = %q, want %q", got, settingsPath)
	}
}

func TestFindQwenSettingsPathsInvalidScope(t *testing.T) {
	if _, err := FindQwenSettingsPaths("bogus"); err == nil {
		t.Error("expected invalid Qwen scope error")
	}
}
