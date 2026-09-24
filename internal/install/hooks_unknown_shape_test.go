package install

import (
	"strings"
	"testing"
)

// TestMergeRejectsUnknownHookValueShape pins that an existing hook value
// that is neither a string nor an array is an error, not a "%v" command.
func TestMergeRejectsUnknownHookValueShape(t *testing.T) {
	for name, value := range map[string]interface{}{
		"number": float64(42),
		"object": map[string]interface{}{"command": "echo hi"},
		"bool":   true,
	} {
		t.Run(name, func(t *testing.T) {
			settings := SettingsMap{"hooks": map[string]interface{}{"PreToolUse": value}}
			claudio, err := GenerateClaudioHooksForAgent("/usr/local/bin/claudio", AgentClaude)
			if err != nil {
				t.Fatal(err)
			}
			_, err = MergeHooksIntoSettings(&settings, claudio)
			if err == nil {
				t.Fatal("expected error for unknown hook value shape")
			}
			if !strings.Contains(err.Error(), "PreToolUse") {
				t.Errorf("error %q does not name the hook", err)
			}
		})
	}
}
