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
func newSoundpackUseCommand() *cobra.Command {
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
			return runSoundpackUse(cmd, args[0])
		},
	}
	return useCmd
}

// runSoundpackUse executes the soundpack use command
func runSoundpackUse(cmd *cobra.Command, name string) error {
	slog.Debug("running soundpack use", "name", name)

	// Discover available soundpacks to validate the name
	packs, err := discoverSoundpacks()
	if err != nil {
		slog.Error("failed to discover soundpacks", "error", err)
		return fmt.Errorf("failed to discover soundpacks: %w", err)
	}

	// Check if the requested name matches any discovered soundpack
	found := false
	for _, p := range packs {
		if p.Name == name {
			found = true
			break
		}
	}

	if !found {
		// Build list of available names for the error message
		var available []string
		for _, p := range packs {
			available = append(available, p.Name)
		}
		sort.Strings(available)
		slog.Error("soundpack not found", "name", name, "available", available)
		return fmt.Errorf("soundpack '%s' not found. Available soundpacks: %s", name, strings.Join(available, ", "))
	}

	alreadyActive := false
	if err := mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		alreadyActive = cfg.DefaultSoundpack == name
		cfg.DefaultSoundpack = name
		return nil
	}); err != nil {
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
