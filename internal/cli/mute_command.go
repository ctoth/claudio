package cli

import (
	"fmt"
	"log/slog"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// newMuteCommand returns the `claudio mute` subcommand. Persistent
// equivalent of the transient `--silent` flag — sets cfg.Enabled =
// false in config.json. CLAUDIO_ENABLED=true env var will still
// override at runtime.
func newMuteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "mute",
		Short: "Persistently disable claudio audio",
		Long: `Persistently disable claudio audio by setting enabled=false in config.json.

Persistent equivalent of the transient --silent flag. To re-enable,
run 'claudio unmute' or set enabled=true in your config file.

Note: the CLAUDIO_ENABLED=true environment variable, if set, will
still override this at runtime.`,
		Args: cobra.NoArgs,
		RunE: runMuteE,
	}
}

// newUnmuteCommand returns the `claudio unmute` subcommand. Symmetric
// to mute — sets cfg.Enabled = true.
func newUnmuteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unmute",
		Short: "Persistently enable claudio audio",
		Long: `Persistently enable claudio audio by setting enabled=true in config.json.

Symmetric counterpart to 'claudio mute'.

Note: the CLAUDIO_ENABLED=false environment variable, if set, will
still override this at runtime.`,
		Args: cobra.NoArgs,
		RunE: runUnmuteE,
	}
}

func runMuteE(cmd *cobra.Command, _ []string) error {
	return setEnabledAndPersist(cmd, false, "audio muted")
}

func runUnmuteE(cmd *cobra.Command, _ []string) error {
	return setEnabledAndPersist(cmd, true, "audio unmuted")
}

// setEnabledAndPersist is the shared core for mute/unmute. Acquires
// the config lock, loads existing config, flips Enabled, writes
// atomically.
func setEnabledAndPersist(cmd *cobra.Command, enabled bool, successMsg string) error {
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}
	cli.initializeConfigManager()

	configPath, err := resolveWritableConfigPath(cmd, cli)
	if err != nil {
		return err
	}

	lock, err := config.LockConfigDir(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			slog.Warn("failed to release config lock", "err", err)
		}
	}()

	cfg, err := loadConfigForVerb(cli, configPath)
	if err != nil {
		return err
	}

	cfg.Enabled = enabled
	if err := config.WriteConfigFile(afero.NewOsFs(), configPath, cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), successMsg)
	slog.Info("enabled persisted", "path", configPath, "enabled", enabled)
	return nil
}
