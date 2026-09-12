package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// resolveWritableConfigPath returns the config path the verb subcommands
// (volume/mute/unmute) should read-mutate-write.
//
// Precedence:
//  1. --config flag if set.
//  2. First entry of XDG GetConfigPaths("config.json") — the writable
//     user config path ($XDG_CONFIG_HOME/claudio/config.json).
//
// Never writes to a system-level config dir.
func resolveWritableConfigPath(cmd *cobra.Command, _ *CLI) (string, error) {
	if flag, _ := cmd.Flags().GetString("config"); flag != "" {
		return flag, nil
	}
	paths := config.NewXDGDirs().GetConfigPaths("config.json")
	if len(paths) == 0 {
		return "", fmt.Errorf("no XDG config path available")
	}
	return paths[0], nil
}

// loadConfigForVerb loads the config from the given path, falling back
// to GetDefaultConfig() when the file does not exist. A parse/validate
// error is surfaced to the caller — writing on top of an unreadable
// file would silently lose state the user might still want to recover.
func loadConfigForVerb(cli *CLI, configPath string) (*config.Config, error) {
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			slog.Debug("config file missing; using defaults for verb load", "path", configPath)
			return cli.configManager.GetDefaultConfig(), nil
		}
		return nil, fmt.Errorf("stat config %s: %w", configPath, err)
	}
	cfg, err := cli.configManager.LoadFromFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config %s: %w", configPath, err)
	}
	return cfg, nil
}

// mutateConfigForCommand performs one locked read-modify-write operation on
// the config selected by --config/XDG precedence. Existing malformed configs
// are returned as errors and are never replaced with defaults.
func mutateConfigForCommand(cmd *cobra.Command, mutate func(*config.Config) error) error {
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}

	configPath, err := resolveWritableConfigPath(cmd, cli)
	if err != nil {
		return err
	}
	lock, err := config.LockConfigDir(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release config lock", "path", configPath, "error", unlockErr)
		}
	}()

	cfg, err := loadConfigForVerb(cli, configPath)
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	if err := cli.configManager.ValidateConfig(cfg); err != nil {
		return err
	}
	if err := config.WriteConfigFile(afero.NewOsFs(), configPath, cfg); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}
	return nil
}

// validateConfigMutationTarget verifies that the selected config can be read
// before a command performs an irreversible filesystem operation.
func validateConfigMutationTarget(cmd *cobra.Command) error {
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}
	configPath, err := resolveWritableConfigPath(cmd, cli)
	if err != nil {
		return err
	}
	lock, err := config.LockConfigDir(configPath)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release config validation lock", "path", configPath, "error", unlockErr)
		}
	}()
	_, err = loadConfigForVerb(cli, configPath)
	return err
}
