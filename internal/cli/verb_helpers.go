package cli

import (
	"fmt"
	"log/slog"

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

// loadConfigForVerb loads the config a read-modify-write command starts
// from, with the shared loadConfig policy: a missing file means defaults.
// An unusable file is an error here, unlike read-only commands — writing on
// top of it would silently lose state the user might still want to recover.
func loadConfigForVerb(cmd *cobra.Command, cli *CLI) (*config.Config, error) {
	loaded := loadConfig(cmd, cli)
	if loaded.Err != nil {
		return nil, fmt.Errorf("load %w", loaded.Err)
	}
	return loaded.Config, nil
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

	cfg, err := loadConfigForVerb(cmd, cli)
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
	_, err = loadConfigForVerb(cmd, cli)
	return err
}
