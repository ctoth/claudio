package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindCopilotSettingsPathsGlobalScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("COPILOT_HOME", "")

	paths, err := FindCopilotSettingsPaths("global")
	if err != nil {
		t.Fatalf("FindCopilotSettingsPaths returned error: %v", err)
	}
	want := filepath.Join(home, ".copilot", "settings.json")
	if len(paths) == 0 || paths[0] != want {
		t.Fatalf("first global Copilot path = %v, want %q", paths, want)
	}
}

func TestFindCopilotSettingsPathsHonorsCOPILOTHOME(t *testing.T) {
	copilotHome := t.TempDir()
	t.Setenv("COPILOT_HOME", copilotHome)

	paths, err := FindCopilotSettingsPaths("global")
	if err != nil {
		t.Fatalf("FindCopilotSettingsPaths returned error: %v", err)
	}
	want := filepath.Join(copilotHome, "settings.json")
	if len(paths) == 0 || paths[0] != want {
		t.Fatalf("first global Copilot path = %v, want %q", paths, want)
	}
}

func TestFindCopilotSettingsPathsProjectScope(t *testing.T) {
	paths, err := FindCopilotSettingsPaths("project")
	if err != nil {
		t.Fatalf("FindCopilotSettingsPaths returned error: %v", err)
	}
	want := []string{
		filepath.Join(".", ".github", "copilot", "settings.local.json"),
		filepath.Join(".github", "copilot", "settings.local.json"),
		filepath.Join(".", ".github", "copilot", "settings.json"),
		filepath.Join(".github", "copilot", "settings.json"),
	}
	if len(paths) != len(want) {
		t.Fatalf("project Copilot path count = %d, want %d", len(paths), len(want))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("project Copilot path %d = %q, want %q", i, paths[i], want[i])
		}
	}
}

func TestFindBestCopilotPathPrefersExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	t.Setenv("COPILOT_HOME", "")

	settingsPath := filepath.Join(home, ".copilot", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(`{"hooks":{}}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := FindBestCopilotPath("global")
	if err != nil {
		t.Fatalf("FindBestCopilotPath returned error: %v", err)
	}
	if got != settingsPath {
		t.Errorf("FindBestCopilotPath = %q, want %q", got, settingsPath)
	}
}

func TestFindCopilotSettingsPathsInvalidScope(t *testing.T) {
	if _, err := FindCopilotSettingsPaths("bogus"); err == nil {
		t.Error("expected invalid Copilot scope error")
	}
}
