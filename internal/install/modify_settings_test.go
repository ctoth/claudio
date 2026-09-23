package install

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

func TestModifySettingsWritesFunctionResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"a":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	fsys := afero.NewOsFs()
	err := ModifySettings(fsys, path, func(s *SettingsMap) (*SettingsMap, error) {
		return &SettingsMap{"a": (*s)["a"], "b": "two"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatal(err)
	}
	if want := (SettingsMap{"a": float64(1), "b": "two"}); !reflect.DeepEqual(*got, want) {
		t.Fatalf("settings = %v, want %v", *got, want)
	}
}

// TestModifySettingsSkipsWriteWhenUnchanged pins that a no-op edit (e.g.
// uninstall with nothing to remove) leaves the file and its backup alone.
func TestModifySettingsSkipsWriteWhenUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := []byte("{ \"a\" :   1 }")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	err := ModifySettings(afero.NewOsFs(), path, func(s *SettingsMap) (*SettingsMap, error) {
		return &SettingsMap{"a": float64(1)}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(original) {
		t.Fatalf("unchanged settings were rewritten: %q", got)
	}
	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatalf("unchanged settings produced a backup (err=%v)", err)
	}
}

func TestModifySettingsFunctionErrorLeavesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	original := []byte(`{"a":1}`)
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	boom := errors.New("boom")
	err := ModifySettings(afero.NewOsFs(), path, func(s *SettingsMap) (*SettingsMap, error) {
		return &SettingsMap{"a": 2}, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if got, _ := os.ReadFile(path); string(got) != string(original) {
		t.Fatalf("settings changed after fn error: %q", got)
	}
}

func TestModifySettingsReportsReadError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0644); err != nil {
		t.Fatal(err)
	}
	called := false
	err := ModifySettings(afero.NewOsFs(), path, func(s *SettingsMap) (*SettingsMap, error) {
		called = true
		return s, nil
	})
	if err == nil || called {
		t.Fatalf("err = %v, called = %v; want read error before fn", err, called)
	}
}

func TestModifySettingsReportsLockError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "settings.json")
	if err := ModifySettings(afero.NewOsFs(), path, func(s *SettingsMap) (*SettingsMap, error) { return s, nil }); err == nil {
		t.Fatal("expected error when the settings directory does not exist")
	}
}

func TestModifySettingsReportsWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	err := ModifySettings(afero.NewOsFs(), path, func(s *SettingsMap) (*SettingsMap, error) {
		return &SettingsMap{"bad": make(chan int)}, nil
	})
	if err == nil {
		t.Fatal("expected write error for an unencodable value")
	}
}

func TestAgentHooksReportUncopyableSettings(t *testing.T) {
	bad := SettingsMap{"bad": make(chan int)}
	if _, err := InstallAgentHooks(&bad, AgentCodex, "/usr/local/bin/claudio"); err == nil {
		t.Error("InstallAgentHooks: expected copy error")
	}
	if _, _, err := RemoveAgentHooks(&bad, AgentClaude); err == nil {
		t.Error("RemoveAgentHooks: expected copy error")
	}
}
