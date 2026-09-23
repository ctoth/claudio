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
func resolveWritableConfigPath(cmd *cobra.Command) (string, error) {
	if flag, _ := cmd.Flags().GetString("config"); flag != "" {
		return flag, nil
	}
	paths := config.ConfigPaths("config.json")
	if len(paths) == 0 {
		return "", fmt.Errorf("no XDG config path available")
	}
	return paths[0], nil
}

// loadConfigForVerb loads the config a read-modify-write command starts
// from, with the shared loadConfig policy: a missing file means defaults.
// An unusable file is an error here, unlike read-only commands — writing on
// top of it would silently lose state the user might still want to recover.
func (c *CLI) loadConfigForVerb(cmd *cobra.Command) (*config.Config, error) {
	loaded := c.loadConfig(cmd)
	if loaded.Err != nil {
		return nil, fmt.Errorf("load %w", loaded.Err)
	}
	return loaded.Config, nil
}

// withConfigLock runs fn while holding the advisory lock on the config
// selected by --config/XDG precedence, passing it that path.
func withConfigLock(cmd *cobra.Command, fn func(configPath string) error) error {
	configPath, err := resolveWritableConfigPath(cmd)
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
	return fn(configPath)
}

// mutateConfigForCommand performs one locked read-modify-write operation on
// the config selected by --config/XDG precedence. Existing malformed configs
// are returned as errors and are never replaced with defaults.
func (c *CLI) mutateConfigForCommand(cmd *cobra.Command, mutate func(*config.Config) error) error {
	return withConfigLock(cmd, func(configPath string) error {
		cfg, err := c.loadConfigForVerb(cmd)
		if err != nil {
			return err
		}
		if err := mutate(cfg); err != nil {
			return err
		}
		if err := c.configManager.ValidateConfig(cfg); err != nil {
			return err
		}
		if err := config.WriteConfigFile(afero.NewOsFs(), configPath, cfg); err != nil {
			return fmt.Errorf("write config %s: %w", configPath, err)
		}
		return nil
	})
}

// validateConfigMutationTarget verifies that the selected config can be read
// before a command performs an irreversible filesystem operation.
func (c *CLI) validateConfigMutationTarget(cmd *cobra.Command) error {
	return withConfigLock(cmd, func(string) error {
		_, err := c.loadConfigForVerb(cmd)
		return err
	})
}
