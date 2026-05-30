package cli

import (
	"github.com/spf13/cobra"
)

// newSoundpackCommand creates the soundpack command group and wires every
// subcommand factory into it. Each subcommand is defined in its own file
// (soundpack_init.go, soundpack_list.go, soundpack_validate.go,
// soundpack_install.go, soundpack_use.go) and the add/update/remove/status
// subcommands live alongside the git-managed soundpack code in
// soundpack_git.go. Shared discovery helpers are in soundpack_helpers.go.
func newSoundpackCommand() *cobra.Command {
	soundpackCmd := &cobra.Command{
		Use:   "soundpack",
		Short: "Manage soundpacks",
		Long:  "Commands for creating, validating, and managing soundpacks",
	}
	soundpackCmd.AddCommand(newSoundpackInitCommand())
	soundpackCmd.AddCommand(newSoundpackListCommand())
	soundpackCmd.AddCommand(newSoundpackValidateCommand())
	soundpackCmd.AddCommand(newSoundpackInstallCommand())
	soundpackCmd.AddCommand(newSoundpackUseCommand())
	soundpackCmd.AddCommand(newSoundpackAddCommand())
	soundpackCmd.AddCommand(newSoundpackUpdateCommand())
	soundpackCmd.AddCommand(newSoundpackRemoveCommand())
	soundpackCmd.AddCommand(newSoundpackStatusCommand())
	return soundpackCmd
}
