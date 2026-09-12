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

const installedSoundpackMarker = ".claudio-installed"

// newSoundpackInstallCommand creates the soundpack install subcommand
func newSoundpackInstallCommand() *cobra.Command {
	var setDefault bool
	var skipValidate bool

	installCmd := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a soundpack from a JSON file or directory",
		Long: `Install a soundpack by copying it to the XDG data directory and updating config.

JSON soundpacks are installed with their relative assets under
<XDG_DATA_HOME>/claudio/soundpacks/<name>/soundpack.json.
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
	installCmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip the preliminary coverage check (safe install validation still applies)")

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
	if err := validateManagedSoundpackName(name); err != nil {
		return fmt.Errorf("invalid soundpack name: %w", err)
	}

	// Determine install target
	installDir := filepath.Join(xdg.DataHome, "claudio", "soundpacks", name)
	installPath := installDir
	if !isDir {
		installPath = filepath.Join(installDir, "soundpack.json")
	}
	slog.Info("install target determined", "install_path", installPath)

	if err := validateConfigMutationTarget(cmd); err != nil {
		return fmt.Errorf("failed to load config before installing soundpack: %w", err)
	}
	if err := stageAndInstallSoundpack(srcPath, installDir, isDir); err != nil {
		slog.Error("failed to install soundpack files", "src", srcPath, "dst", installDir, "error", err)
		return err
	}
	slog.Info("soundpack copied successfully", "install_path", installPath)

	// Update config
	if err := updateConfigForInstall(cmd, installPath, name, setDefault); err != nil {
		slog.Error("failed to update config", "error", err)
		return fmt.Errorf("failed to update config: %w", err)
	}

	cmd.Printf("Installed soundpack '%s' to %s\n", name, installPath)
	return nil
}

// updateConfigForInstall loads the config, adds the install path, optionally sets default, and saves.
func updateConfigForInstall(cmd *cobra.Command, installPath, name string, setDefault bool) error {
	slog.Debug("updating config for install", "install_path", installPath, "name", name, "set_default", setDefault)
	return mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		pathExists := false
		for _, p := range cfg.SoundpackPaths {
			if p == installPath {
				pathExists = true
				break
			}
		}
		if !pathExists {
			cfg.SoundpackPaths = append(cfg.SoundpackPaths, installPath)
		}
		if setDefault {
			cfg.DefaultSoundpack = name
		}
		return nil
	})
}

func stageAndInstallSoundpack(srcPath, installDir string, isDir bool) error {
	parent := filepath.Dir(installDir)
	stagingParent := filepath.Dir(parent)
	if isDir {
		sourceAbs, err := filepath.Abs(srcPath)
		if err != nil {
			return fmt.Errorf("failed to resolve source directory: %w", err)
		}
		stagingAbs, err := filepath.Abs(stagingParent)
		if err != nil {
			return fmt.Errorf("failed to resolve staging directory: %w", err)
		}
		rel, err := filepath.Rel(sourceAbs, stagingAbs)
		if err != nil {
			return fmt.Errorf("failed to compare source and staging directories: %w", err)
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			return fmt.Errorf("refusing to install a directory that contains the Claudio soundpack destination: %s", srcPath)
		}
	}
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("failed to create soundpack directory: %w", err)
	}
	stageDir, err := os.MkdirTemp(stagingParent, ".soundpack-install-*")
	if err != nil {
		return fmt.Errorf("failed to create soundpack staging directory: %w", err)
	}
	defer os.RemoveAll(stageDir)

	if isDir {
		if err := copyDirectory(srcPath, stageDir); err != nil {
			return fmt.Errorf("failed to stage directory soundpack: %w", err)
		}
		if _, err := validateDirectorySoundpack(stageDir); err != nil {
			return fmt.Errorf("staged soundpack validation failed: %w", err)
		}
	} else {
		if err := stageJSONSoundpack(srcPath, stageDir); err != nil {
			return err
		}
		if _, err := soundpack.CreateSoundpackMapper("installed", filepath.Join(stageDir, "soundpack.json")); err != nil {
			return fmt.Errorf("staged soundpack validation failed: %w", err)
		}
	}
	if err := os.WriteFile(filepath.Join(stageDir, installedSoundpackMarker), []byte("claudio soundpack install\n"), 0644); err != nil {
		return fmt.Errorf("failed to mark installed soundpack: %w", err)
	}

	if info, err := os.Stat(installDir); err == nil {
		if !info.IsDir() || !isClaudioInstalledSoundpack(installDir) {
			return fmt.Errorf("refusing to overwrite existing unowned soundpack destination: %s", installDir)
		}
		backupDir := filepath.Join(stagingParent, ".soundpack-previous-"+filepath.Base(installDir))
		if _, err := os.Stat(backupDir); err == nil {
			return fmt.Errorf("cannot replace soundpack while backup path exists: %s", backupDir)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot inspect soundpack backup path: %w", err)
		}
		if err := os.Rename(installDir, backupDir); err != nil {
			return fmt.Errorf("failed to preserve previous soundpack: %w", err)
		}
		if err := os.Rename(stageDir, installDir); err != nil {
			_ = os.Rename(backupDir, installDir)
			return fmt.Errorf("failed to activate staged soundpack: %w", err)
		}
		if err := os.RemoveAll(backupDir); err != nil {
			return fmt.Errorf("installed soundpack but failed to remove previous copy: %w", err)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot inspect soundpack destination: %w", err)
	}
	if err := os.Rename(stageDir, installDir); err != nil {
		return fmt.Errorf("failed to activate staged soundpack: %w", err)
	}
	return nil
}

func stageJSONSoundpack(srcPath, stageDir string) error {
	metadata, err := soundpack.PeekJSONSoundpackMetadataFromFile(srcPath)
	if err != nil {
		return fmt.Errorf("failed to read soundpack manifest: %w", err)
	}
	sourceDir := filepath.Dir(srcPath)
	seen := make(map[string]struct{})
	for key, value := range metadata.Mappings {
		if value == "" {
			continue
		}
		rel := filepath.Clean(filepath.FromSlash(value))
		if filepath.IsAbs(rel) || filepath.VolumeName(rel) != "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("mapping %q must be a relative path inside the soundpack: %q", key, value)
		}
		if rel == "." || strings.EqualFold(rel, installedSoundpackMarker) || strings.EqualFold(rel, "soundpack.json") {
			return fmt.Errorf("mapping %q uses reserved install path %q", key, value)
		}
		if _, ok := seen[rel]; ok {
			continue
		}
		seen[rel] = struct{}{}
		sourceFile := filepath.Join(sourceDir, rel)
		info, err := os.Stat(sourceFile)
		if err != nil {
			return fmt.Errorf("cannot access mapped file %q: %w", value, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("mapped path %q is not a regular file", value)
		}
		if err := copyFile(sourceFile, filepath.Join(stageDir, rel)); err != nil {
			return fmt.Errorf("failed to copy mapped file %q: %w", value, err)
		}
	}
	if err := copyFile(srcPath, filepath.Join(stageDir, "soundpack.json")); err != nil {
		return fmt.Errorf("failed to copy soundpack manifest: %w", err)
	}
	return nil
}

func isClaudioInstalledSoundpack(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, installedSoundpackMarker))
	return err == nil && string(data) == "claudio soundpack install\n"
}
