package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// newMuteCommand returns the `claudio mute` subcommand. Persistent
// equivalent of the transient `--silent` flag — sets cfg.Enabled =
// false in config.json. CLAUDIO_ENABLED=true env var will still
// override at runtime.
func newMuteCommand(c *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "mute",
		Short: "Persistently disable claudio audio",
		Long: `Persistently disable claudio audio by setting enabled=false in config.json.

Persistent equivalent of the transient --silent flag. To re-enable,
run 'claudio unmute' or set enabled=true in your config file.

Note: the CLAUDIO_ENABLED=true environment variable, if set, will
still override this at runtime.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return c.setEnabledAndPersist(cmd, false, "audio muted") },
	}
}

// newUnmuteCommand returns the `claudio unmute` subcommand. Symmetric
// to mute — sets cfg.Enabled = true.
func newUnmuteCommand(c *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "unmute",
		Short: "Persistently enable claudio audio",
		Long: `Persistently enable claudio audio by setting enabled=true in config.json.

Symmetric counterpart to 'claudio mute'.

Note: the CLAUDIO_ENABLED=false environment variable, if set, will
still override this at runtime.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error { return c.setEnabledAndPersist(cmd, true, "audio unmuted") },
	}
}

// setEnabledAndPersist is the shared core for mute/unmute: one locked,
// validated read-modify-write through mutateConfigForCommand.
func (c *CLI) setEnabledAndPersist(cmd *cobra.Command, enabled bool, successMsg string) error {
	if err := c.mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		cfg.Enabled = enabled
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), successMsg)
	slog.Info("enabled persisted", "enabled", enabled)
	return nil
}
