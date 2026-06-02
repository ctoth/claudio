package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
)

// newSoundpackListCommand creates the soundpack list subcommand
func newSoundpackListCommand() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List all discoverable soundpacks",
		Long: `List all soundpacks from embedded platform packs, XDG data directories,
and config soundpack_paths.

Shows name, type (embedded/json/directory), sound count, and path for each pack.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackList(cmd)
		},
	}
	return listCmd
}

// runSoundpackList executes the soundpack list command
func runSoundpackList(cmd *cobra.Command) error {
	slog.Debug("running soundpack list")

	packs, err := discoverSoundpacks()
	if err != nil {
		slog.Error("failed to discover soundpacks", "error", err)
		return fmt.Errorf("failed to discover soundpacks: %w", err)
	}

	slog.Info("discovered soundpacks", "count", len(packs))

	// Calculate column widths for tabular formatting
	nameWidth := len("NAME")
	typeWidth := len("TYPE")
	soundsWidth := len("SOUNDS")
	for _, p := range packs {
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
		if len(p.Type) > typeWidth {
			typeWidth = len(p.Type)
		}
		countStr := fmt.Sprintf("%d", p.SoundCount)
		if len(countStr) > soundsWidth {
			soundsWidth = len(countStr)
		}
	}

	// Add padding
	nameWidth += 2
	typeWidth += 2
	soundsWidth += 2

	// Print header
	format := fmt.Sprintf("%%-%ds%%-%ds%%-%ds%%s\n", nameWidth, typeWidth, soundsWidth)
	cmd.Printf(format, "NAME", "TYPE", "SOUNDS", "PATH")

	// Print rows
	for _, p := range packs {
		cmd.Printf(format, p.Name, p.Type, fmt.Sprintf("%d", p.SoundCount), p.Path)
	}

	return nil
}
