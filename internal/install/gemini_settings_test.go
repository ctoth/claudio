package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindGeminiSettingsPathsGlobalScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	paths, err := FindGeminiSettingsPaths("global")
	if err != nil {
		t.Fatalf("FindGeminiSettingsPaths returned error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one global Gemini settings path")
	}

	want := filepath.Join(home, ".gemini", "settings.json")
	if paths[0] != want {
		t.Errorf("first global Gemini path = %q, want %q", paths[0], want)
	}
}

func TestFindGeminiSettingsPathsLegacyUserScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	paths, err := FindGeminiSettingsPaths("user")
	if err != nil {
		t.Fatalf("FindGeminiSettingsPaths returned error: %v", err)
	}

	want := filepath.Join(home, ".gemini", "settings.json")
	if paths[0] != want {
		t.Errorf("legacy user Gemini path = %q, want %q", paths[0], want)
	}
}

func TestFindGeminiSettingsPathsProjectScope(t *testing.T) {
	paths, err := FindGeminiSettingsPaths("project")
	if err != nil {
		t.Fatalf("FindGeminiSettingsPaths returned error: %v", err)
	}

	want := []string{
		filepath.Join(".", ".gemini", "settings.json"),
		filepath.Join(".gemini", "settings.json"),
	}
	if len(paths) != len(want) {
		t.Fatalf("project Gemini path count = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("project Gemini path %d = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestFindGeminiSettingsPathsInvalidScope(t *testing.T) {
	if _, err := FindGeminiSettingsPaths("bogus"); err == nil {
		t.Error("expected invalid Gemini scope error")
	}
}

func TestFindBestGeminiPathPrefersExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")

	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindBestGeminiPath("global")
	if err != nil {
		t.Fatalf("FindBestGeminiPath returned error: %v", err)
	}
	if got != settingsPath {
		t.Errorf("FindBestGeminiPath = %q, want %q", got, settingsPath)
	}
}
