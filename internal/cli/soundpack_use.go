package cli

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"claudio.click/internal/config"
	"github.com/adrg/xdg"
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

	// Load existing config
	cm := config.NewConfigManager()
	cfg, err := cm.LoadConfig()
	if err != nil {
		slog.Warn("could not load existing config, using defaults", "error", err)
		cfg = cm.GetDefaultConfig()
	}

	// Check if already active
	alreadyActive := cfg.DefaultSoundpack == name

	// Update default_soundpack
	cfg.DefaultSoundpack = name

	// Determine config file path
	xdgDirs := config.NewXDGDirs()
	configPaths := xdgDirs.GetConfigPaths("config.json")
	var configFilePath string
	if len(configPaths) > 0 {
		configFilePath = configPaths[0]
	} else {
		configFilePath = filepath.Join(xdg.ConfigHome, "claudio", "config.json")
	}
	slog.Debug("config file path determined", "path", configFilePath)

	// Save config
	if err := cm.SaveToFile(cfg, configFilePath); err != nil {
		slog.Error("failed to save config", "path", configFilePath, "error", err)
		return fmt.Errorf("failed to save config: %w", err)
	}

	if alreadyActive {
		cmd.Printf("Soundpack '%s' is already active.\n", name)
	} else {
		cmd.Printf("Switched active soundpack to '%s'.\n", name)
	}

	slog.Info("soundpack use completed", "name", name, "was_already_active", alreadyActive)
	return nil
}
