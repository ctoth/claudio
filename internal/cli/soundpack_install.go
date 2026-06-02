package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
	"github.com/adrg/xdg"
	"github.com/spf13/cobra"
)

// newSoundpackInstallCommand creates the soundpack install subcommand
func newSoundpackInstallCommand() *cobra.Command {
	var setDefault bool
	var skipValidate bool

	installCmd := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a soundpack from a JSON file or directory",
		Long: `Install a soundpack by copying it to the XDG data directory and updating config.

JSON soundpacks are copied to <XDG_DATA_HOME>/claudio/<name>.json
Directory soundpacks are copied to <XDG_DATA_HOME>/claudio/soundpacks/<name>/

The installed path is added to config soundpack_paths (idempotent).
Use --default to also set the soundpack as the default.

Examples:
  claudio soundpack install my-pack.json
  claudio soundpack install /path/to/soundpack-dir
  claudio soundpack install my-pack.json --default
  claudio soundpack install my-pack.json --skip-validate`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackInstall(cmd, args[0], setDefault, skipValidate)
		},
	}

	installCmd.Flags().BoolVar(&setDefault, "default", false, "Set as the default soundpack")
	installCmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip validation before installing")

	return installCmd
}

// runSoundpackInstall executes the soundpack install command
func runSoundpackInstall(cmd *cobra.Command, srcPath string, setDefault, skipValidate bool) error {
	slog.Debug("running soundpack install", "path", srcPath, "set_default", setDefault, "skip_validate", skipValidate)

	// Check that the source path exists
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		slog.Error("cannot access source path", "path", srcPath, "error", err)
		return fmt.Errorf("cannot access source path: %w", err)
	}

	isDir := srcInfo.IsDir()

	// Determine soundpack name
	var name string
	if isDir {
		name = filepath.Base(srcPath)
	} else {
		name = strings.TrimSuffix(filepath.Base(srcPath), ".json")
	}
	slog.Info("determined soundpack name", "name", name, "is_directory", isDir)

	// Validate unless --skip-validate
	if !skipValidate {
		slog.Debug("validating soundpack before install")
		if isDir {
			_, valErr := validateDirectorySoundpack(srcPath)
			if valErr != nil {
				slog.Error("validation failed", "error", valErr)
				return fmt.Errorf("validation failed: %w", valErr)
			}
		} else {
			_, valErr := validateJSONSoundpackFile(srcPath)
			if valErr != nil {
				slog.Error("validation failed", "error", valErr)
				return fmt.Errorf("validation failed: %w", valErr)
			}
		}
		slog.Info("soundpack validation passed")
	}

	// For JSON files, read and extract name from the JSON content. Use
	// the permissive metadata peek (size cap + mappings count cap; does
	// NOT require name or mappings to be populated) — this site only
	// wants the name, and install-time JSONs may legitimately have
	// empty mappings (e.g. soundpack init scaffolds).
	if !isDir {
		spFile, peekErr := soundpack.PeekJSONSoundpackMetadataFromFile(srcPath)
		if peekErr != nil {
			slog.Error("failed to peek JSON file", "path", srcPath, "error", peekErr)
			return fmt.Errorf("failed to load JSON file: %w", peekErr)
		}
		if spFile.Name != "" {
			name = spFile.Name
			slog.Debug("using name from JSON file", "name", name)
		}
	}

	// Determine install target
	var installPath string
	if isDir {
		// Directories -> <xdg.DataHome>/claudio/soundpacks/<name>/
		installPath = filepath.Join(xdg.DataHome, "claudio", "soundpacks", name)
	} else {
		// JSON files -> <xdg.DataHome>/claudio/<name>.json
		installPath = filepath.Join(xdg.DataHome, "claudio", name+".json")
	}
	slog.Info("install target determined", "install_path", installPath)

	// Copy file/directory
	if isDir {
		if err := copyDirectory(srcPath, installPath); err != nil {
			slog.Error("failed to copy directory", "src", srcPath, "dst", installPath, "error", err)
			return fmt.Errorf("failed to copy directory: %w", err)
		}
	} else {
		if err := copyFile(srcPath, installPath); err != nil {
			slog.Error("failed to copy file", "src", srcPath, "dst", installPath, "error", err)
			return fmt.Errorf("failed to copy file: %w", err)
		}
	}
	slog.Info("soundpack copied successfully", "install_path", installPath)

	// Update config
	if err := updateConfigForInstall(installPath, name, setDefault); err != nil {
		slog.Error("failed to update config", "error", err)
		return fmt.Errorf("failed to update config: %w", err)
	}

	cmd.Printf("Installed soundpack '%s' to %s\n", name, installPath)
	return nil
}

// updateConfigForInstall loads the config, adds the install path, optionally sets default, and saves.
func updateConfigForInstall(installPath, name string, setDefault bool) error {
	slog.Debug("updating config for install", "install_path", installPath, "name", name, "set_default", setDefault)

	cm := config.NewConfigManager()

	// Load existing config
	cfg, err := cm.LoadConfig()
	if err != nil {
		slog.Warn("could not load existing config, using defaults", "error", err)
		cfg = cm.GetDefaultConfig()
	}

	// Add install path to soundpack_paths if not already present (idempotent)
	pathExists := false
	for _, p := range cfg.SoundpackPaths {
		if p == installPath {
			pathExists = true
			break
		}
	}
	if !pathExists {
		cfg.SoundpackPaths = append(cfg.SoundpackPaths, installPath)
		slog.Info("added install path to soundpack_paths", "path", installPath)
	} else {
		slog.Debug("install path already in soundpack_paths", "path", installPath)
	}

	// Set default if requested
	if setDefault {
		cfg.DefaultSoundpack = name
		slog.Info("set default soundpack", "name", name)
	}

	// Determine config file path
	xdgDirs := config.NewXDGDirs()
	configPaths := xdgDirs.GetConfigPaths("config.json")
	var configFilePath string
	if len(configPaths) > 0 {
		configFilePath = configPaths[0] // First path is user config (highest priority)
	} else {
		configFilePath = filepath.Join(xdg.ConfigHome, "claudio", "config.json")
	}
	slog.Debug("config file path determined", "path", configFilePath)

	// Save config
	if err := cm.SaveToFile(cfg, configFilePath); err != nil {
		slog.Error("failed to save config", "path", configFilePath, "error", err)
		return fmt.Errorf("failed to save config: %w", err)
	}

	slog.Info("config updated successfully", "path", configFilePath)
	return nil
}
