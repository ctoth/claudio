package install

import (
	"reflect"
	"testing"

	"github.com/spf13/afero"
)

// TestCodexInstallMergesIntoHooksJSON installs through a real settings file
// round-trip: the user's hook survives and claudio's hooks are written.
func TestCodexInstallMergesIntoHooksJSON(t *testing.T) {
	fsys := afero.NewMemMapFs()
	path := "/home/u/.codex/hooks.json"

	existing := []byte(`{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/usr/bin/logger"}]}]}}`)
	if err := afero.WriteFile(fsys, path, existing, 0644); err != nil {
		t.Fatal(err)
	}

	settings, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := InstallAgentHooks(settings, AgentCodex, "/usr/local/bin/claudio")
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSettingsFile(fsys, path, merged); err != nil {
		t.Fatal(err)
	}

	readBack, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ClaudioHookNames(readBack), enabledHookNames(AgentCodex); len(got) != len(want) {
		t.Errorf("claudio hooks after install = %v, want %v", got, want)
	}
	pre := (*readBack)["hooks"].(map[string]any)["PreToolUse"].([]any)
	if user := group("*", cmd("/usr/bin/logger")); !reflect.DeepEqual(pre[0], user) {
		t.Errorf("user hook = %v, want %v", pre[0], user)
	}
}
