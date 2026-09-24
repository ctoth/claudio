package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"claudio.click/internal/config"
	"claudio.click/internal/soundpack/gitpack"
	"github.com/spf13/cobra"
)

func newSoundpackAddCommand(c *CLI) *cobra.Command {
	var name string
	var ref string
	var subdir string
	var setDefault bool
	var skipValidate bool
	var replace bool

	addCmd := &cobra.Command{
		Use:   "add <git-url>",
		Short: "Add a git-backed soundpack",
		Long: `Clone a soundpack from a git repository into Claudio's managed data directory.

The cloned soundpack remains updateable with 'claudio soundpack update'. The
playable soundpack path is added to config soundpack_paths.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSoundpackAdd(cmd, args[0], name, ref, subdir, setDefault, skipValidate, replace)
		},
	}

	addCmd.Flags().StringVar(&name, "name", "", "Name for the installed soundpack")
	addCmd.Flags().StringVar(&ref, "ref", "", "Branch, tag, or commit to check out")
	addCmd.Flags().StringVar(&subdir, "subdir", "", "Directory or JSON file within the repository to use as the soundpack")
	addCmd.Flags().BoolVar(&setDefault, "default", false, "Set as the default soundpack")
	addCmd.Flags().BoolVar(&skipValidate, "skip-validate", false, "Skip validation before adding")
	addCmd.Flags().BoolVar(&replace, "replace", false, "Replace an existing managed git soundpack with the same name")

	return addCmd
}

func newSoundpackUpdateCommand() *cobra.Command {
	var all bool
	var force bool

	updateCmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Update git-backed soundpacks",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) == 0 {
				return nil
			}
			if !all && len(args) == 1 {
				return nil
			}
			if all && len(args) > 0 {
				return fmt.Errorf("use either --all or a soundpack name, not both")
			}
			return fmt.Errorf("provide a soundpack name or --all")
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runSoundpackUpdate(cmd, name, all, force)
		},
	}

	updateCmd.Flags().BoolVar(&all, "all", false, "Update all managed git soundpacks")
	updateCmd.Flags().BoolVar(&force, "force", false, "Discard local clone changes before updating")

	return updateCmd
}

func newSoundpackRemoveCommand(c *CLI) *cobra.Command {
	var keepFiles bool
	var force bool

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a managed git soundpack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.runSoundpackRemove(cmd, args[0], keepFiles, force)
		},
	}

	removeCmd.Flags().BoolVar(&keepFiles, "keep-files", false, "Remove registry/config entries but leave the clone on disk")
	removeCmd.Flags().BoolVar(&force, "force", false, "Remove registry/config entries even if clone deletion fails")

	return removeCmd
}

func newSoundpackStatusCommand() *cobra.Command {
	statusCmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show managed git soundpack status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := ""
			if len(args) == 1 {
				name = args[0]
			}
			return runSoundpackStatus(cmd, name)
		},
	}
	return statusCmd
}

func (c *CLI) runSoundpackAdd(cmd *cobra.Command, source, requestedName, ref, subdir string, setDefault, skipValidate, replace bool) error {
	if err := gitpack.RequireGit(); err != nil {
		return err
	}

	url, err := gitpack.ExpandSource(source)
	if err != nil {
		return err
	}

	name := requestedName
	if name == "" {
		name = gitpack.NameFromURL(url)
	}
	if err := gitpack.ValidateName(name); err != nil {
		return err
	}
	if err := gitpack.ValidateSubdir(subdir); err != nil {
		return err
	}
	if err := gitpack.ValidateRef(ref); err != nil {
		return err
	}
	if err := c.validateConfigMutationTarget(cmd); err != nil {
		return fmt.Errorf("failed to load config before adding soundpack: %w", err)
	}
	req := gitpack.AddRequest{
		URL:          url,
		Name:         name,
		Ref:          ref,
		Subdir:       subdir,
		Replace:      replace,
		SkipValidate: skipValidate,
	}
	return gitpack.WithNameLock(name, func() error {
		result, err := gitpack.Add(commandContext(cmd), req, validateSoundpackInstallPath)
		if err != nil {
			return err
		}
		if err := c.updateConfigForManagedGitInstall(cmd, result.PlayablePath, result.ClonePath, name, setDefault); err != nil {
			return err
		}
		if result.Repaired {
			cmd.Printf("Managed git soundpack '%s' already exists; repaired config\n", name)
			return nil
		}
		cmd.Printf("Added git soundpack '%s' from %s\n", name, source)
		cmd.Printf("Path: %s\n", result.PlayablePath)
		return nil
	})
}

func runSoundpackUpdate(cmd *cobra.Command, name string, all, force bool) error {
	if err := gitpack.RequireGit(); err != nil {
		return err
	}

	registry, err := gitpack.LoadRegistry()
	if err != nil {
		return err
	}

	names := []string{name}
	if all {
		names = registry.SortedNames()
	}

	for _, packName := range names {
		updated, err := gitpack.Update(commandContext(cmd), packName, force, validateSoundpackInstallPath)
		if err != nil {
			return err
		}
		cmd.Printf("Updated git soundpack '%s' to %s\n", packName, gitpack.ShortCommit(updated.ResolvedCommit))
	}

	return nil
}

func (c *CLI) runSoundpackRemove(cmd *cobra.Command, name string, keepFiles, force bool) error {
	if err := c.validateConfigMutationTarget(cmd); err != nil {
		return fmt.Errorf("failed to load config before removing soundpack: %w", err)
	}
	return gitpack.WithNameLock(name, func() error {
		result, err := gitpack.Remove(name, keepFiles, force)
		if err != nil {
			return err
		}
		changed, err := c.removeConfigSoundpackPath(cmd, result.PlayablePath, result.ClonePath, name)
		if err != nil {
			return err
		}
		if !result.Found {
			if !changed {
				return fmt.Errorf("managed git soundpack %q not found", name)
			}
			cmd.Printf("Managed git soundpack '%s' was already removed; repaired config\n", name)
			return nil
		}
		cmd.Printf("Removed git soundpack '%s'\n", name)
		return nil
	})
}

func runSoundpackStatus(cmd *cobra.Command, name string) error {
	entries, err := gitpack.Status(commandContext(cmd), name)
	if err != nil {
		return err
	}

	cmd.Printf("%-24s%-10s%-12s%-16s%s\n", "NAME", "STATE", "COMMIT", "REF", "PATH")
	for _, entry := range entries {
		cmd.Printf("%-24s%-10s%-12s%-16s%s\n",
			entry.Name,
			entry.State,
			gitpack.ShortCommit(entry.ResolvedCommit),
			gitpack.DisplayRef(entry.Ref),
			entry.PlayablePath())
	}

	return nil
}

func discoverManagedGitSoundpacks() []soundpackInfo {
	registry, err := gitpack.LoadRegistry()
	if err != nil {
		slog.Debug("could not load managed git soundpack registry", "error", err)
		return nil
	}

	packs := make([]soundpackInfo, 0, len(registry.Packs))
	for _, record := range registry.Packs {
		playablePath := record.PlayablePath()
		if _, err := os.Stat(playablePath); err != nil {
			continue
		}
		packs = append(packs, soundpackInfo{
			Name: record.Name,
			Type: "git",
			Path: playablePath,
		})
	}
	return packs
}

// validateSoundpackInstallPath fails when the pack at playablePath is
// unreadable, malformed, or has broken or unsafe mappings.
func validateSoundpackInstallPath(playablePath string) error {
	info, err := os.Stat(playablePath)
	if err != nil {
		return fmt.Errorf("cannot access soundpack path: %w", err)
	}
	if !info.IsDir() && !strings.HasSuffix(strings.ToLower(playablePath), ".json") {
		return fmt.Errorf("unsupported soundpack path: %s", playablePath)
	}
	result, err := validateSoundpackPath(playablePath)
	if err != nil {
		return err
	}
	return result.Err()
}

var errNoSoundpackConfigChange = errors.New("soundpack config has no matching entry")

func (c *CLI) updateConfigForManagedGitInstall(cmd *cobra.Command, playablePath, clonePath, name string, setDefault bool) error {
	return c.mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		filtered := make([]string, 0, len(cfg.SoundpackPaths)+1)
		for _, existingPath := range cfg.SoundpackPaths {
			if samePathOrWithin(existingPath, clonePath) {
				continue
			}
			filtered = append(filtered, existingPath)
		}
		filtered = append(filtered, playablePath)
		cfg.SoundpackPaths = filtered
		if setDefault {
			cfg.DefaultSoundpack = name
		}
		return nil
	})
}

func (c *CLI) removeConfigSoundpackPath(cmd *cobra.Command, playablePath, clonePath, removedName string) (bool, error) {
	changed := false
	err := c.mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		filtered := make([]string, 0, len(cfg.SoundpackPaths))
		for _, existingPath := range cfg.SoundpackPaths {
			if samePathOrWithin(existingPath, playablePath) || samePathOrWithin(existingPath, clonePath) {
				changed = true
				continue
			}
			filtered = append(filtered, existingPath)
		}
		cfg.SoundpackPaths = filtered
		if cfg.DefaultSoundpack == removedName {
			cfg.DefaultSoundpack = config.NewConfigManager().GetDefaultConfig().DefaultSoundpack
			changed = true
		}
		if !changed {
			return errNoSoundpackConfigChange
		}
		return nil
	})
	if errors.Is(err, errNoSoundpackConfigChange) {
		return false, nil
	}
	return changed, err
}

func samePathOrWithin(candidate, base string) bool {
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return false
	}
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(baseAbs, candidateAbs)
	if err != nil {
		return false
	}
	return rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// commandContext returns the cobra command's context, or Background when
// the command was run without one.
func commandContext(cmd *cobra.Command) context.Context {
	if cmd != nil {
		if ctx := cmd.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}
