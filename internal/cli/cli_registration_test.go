package cli

import (
	"sort"
	"testing"
)

// TestNewCLI_RegistersAllExpectedSubcommands locks in the set of
// top-level subcommands NewCLI is required to register. Closes the
// lint gap that let install-commands slip past CI: an isolated
// subcommand file that is constructed and tested but never
// AddCommand'd into NewCLI would now fail this test.
//
// Future subcommands silently missing from NewCLI() will fail this
// test instead of slipping past CI.
func TestNewCLI_RegistersAllExpectedSubcommands(t *testing.T) {
	expected := []string{
		"install",
		"uninstall",
		"analyze",
		"soundpack",
		"install-commands",
		"uninstall-commands",
		"volume",
		"mute",
		"unmute",
		"status",
	}

	cli := NewCLI()
	registered := make(map[string]bool)
	for _, sub := range cli.rootCmd.Commands() {
		registered[sub.Name()] = true
	}

	var missing []string
	for _, name := range expected {
		if !registered[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		var got []string
		for name := range registered {
			got = append(got, name)
		}
		sort.Strings(got)
		t.Errorf("NewCLI missing expected subcommands %v (registered: %v)", missing, got)
	}
}
