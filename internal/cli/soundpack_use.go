package cli

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"claudio.click/internal/config"
	"github.com/spf13/cobra"
)

// newSoundpackUseCommand creates the soundpack use subcommand
func newSoundpackUseCommand(c *CLI) *cobra.Command {
	useCmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the active soundpack",
		Long: `Switch the active soundpack by updating default_soundpack in the config file.

The name must match an installed or embedded soundpack. Use 'claudio soundpack list'
to see available soundpacks.

Examples:
  claudio soundpack use windows
  claudio soundpack use my-custom-pack`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSoundpackUse(cmd, args[0])
		},
	}
	return useCmd
}

// runSoundpackUse executes the soundpack use command
func (c *CLI) runSoundpackUse(cmd *cobra.Command, name string) error {
	slog.Debug("running soundpack use", "name", name)

	alreadyActive := false
	var notFound error
	if err := c.mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		// Validate against the same soundpack_paths the runtime will read,
		// using the runtime's own lookup.
		if _, ok := lookupSoundpack(name, cfg.SoundpackPaths); !ok {
			notFound = soundpackNotFoundError(name, cfg.SoundpackPaths)
			return notFound
		}
		alreadyActive = cfg.DefaultSoundpack == name
		cfg.DefaultSoundpack = name
		return nil
	}); err != nil {
		if notFound != nil {
			return notFound
		}
		return fmt.Errorf("failed to update config: %w", err)
	}

	if alreadyActive {
		cmd.Printf("Soundpack '%s' is already active.\n", name)
	} else {
		cmd.Printf("Switched active soundpack to '%s'.\n", name)
	}

	slog.Info("soundpack use completed", "name", name, "was_already_active", alreadyActive)
	return nil
}

func soundpackNotFoundError(name string, configPaths []string) error {
	var available []string
	seen := make(map[string]struct{})
	for _, p := range discoverSoundpacksWithPaths(configPaths) {
		if _, dup := seen[p.Name]; !dup {
			seen[p.Name] = struct{}{}
			available = append(available, p.Name)
		}
	}
	sort.Strings(available)
	return fmt.Errorf("soundpack '%s' not found. Available soundpacks: %s", name, strings.Join(available, ", "))
}
