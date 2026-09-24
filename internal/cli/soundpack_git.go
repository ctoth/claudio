package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claudio.click/internal/config"
	"claudio.click/internal/safeio"
	"claudio.click/internal/soundpack"
	"github.com/gofrs/flock"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	gitSoundpackRegistryVersion = 1
	gitSoundpackSourceType      = "git"
)

type soundpackRegistry struct {
	Version int                           `json:"version"`
	Packs   map[string]gitSoundpackRecord `json:"packs"`
}

type gitSoundpackRecord struct {
	Name           string `json:"name"`
	SourceType     string `json:"source_type"`
	URL            string `json:"url"`
	Ref            string `json:"ref,omitempty"`
	ResolvedCommit string `json:"resolved_commit"`
	Subdir         string `json:"subdir,omitempty"`
	Path           string `json:"path"`
	InstalledAt    string `json:"installed_at"`
	UpdatedAt      string `json:"updated_at"`
}

func newSoundpackAddCommand() *cobra.Command {
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
			return runSoundpackAdd(cmd, args[0], name, ref, subdir, setDefault, skipValidate, replace)
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

func newSoundpackRemoveCommand() *cobra.Command {
	var keepFiles bool
	var force bool

	removeCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a managed git soundpack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSoundpackRemove(cmd, args[0], keepFiles, force)
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

// gitAddRequest carries the validated inputs of `soundpack add`.
type gitAddRequest struct {
	source       string
	url          string
	name         string
	ref          string
	subdir       string
	setDefault   bool
	skipValidate bool
	replace      bool
}

func runSoundpackAdd(cmd *cobra.Command, source, requestedName, ref, subdir string, setDefault, skipValidate, replace bool) error {
	if err := requireGit(); err != nil {
		return err
	}

	url, err := expandGitSoundpackSource(source)
	if err != nil {
		return err
	}

	name := requestedName
	if name == "" {
		name = nameFromGitURL(url)
	}
	if err := validateManagedSoundpackName(name); err != nil {
		return err
	}
	if err := validateGitSubdir(subdir); err != nil {
		return err
	}
	if err := validateGitRef(ref); err != nil {
		return err
	}
	if err := validateConfigMutationTarget(cmd); err != nil {
		return fmt.Errorf("failed to load config before adding soundpack: %w", err)
	}
	req := gitAddRequest{
		source:       source,
		url:          url,
		name:         name,
		ref:          ref,
		subdir:       subdir,
		setDefault:   setDefault,
		skipValidate: skipValidate,
		replace:      replace,
	}
	return withNameLock(name, func() error {
		return addGitSoundpack(cmd, req)
	})
}

// addGitSoundpack clones, validates and activates a managed git soundpack.
// The caller holds the per-name lock.
func addGitSoundpack(cmd *cobra.Command, req gitAddRequest) error {
	name := req.name
	cleanedSubdir := ""
	if req.subdir != "" {
		cleaned := filepath.ToSlash(filepath.Clean(req.subdir))
		if cleaned != "." {
			cleanedSubdir = cleaned
		}
	}
	clonePath := filepath.Join(gitSoundpackBaseDir(), name)

	existing, exists, err := readRecord(name)
	if err != nil {
		return err
	}
	if exists && !req.replace {
		return repairExistingGitSoundpack(cmd, req, existing, cleanedSubdir, clonePath)
	}

	if info, statErr := os.Stat(clonePath); statErr == nil {
		if !exists {
			return fmt.Errorf("managed clone path exists without a registry record; refusing to replace unowned path: %s", clonePath)
		}
		if !info.IsDir() {
			return fmt.Errorf("managed clone path is not a directory: %s", clonePath)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to inspect managed clone path: %w", statErr)
	}
	if exists && filepath.Clean(existing.Path) != filepath.Clean(clonePath) {
		return fmt.Errorf("managed git soundpack %q has an unexpected clone path; refusing replacement", name)
	}

	if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
		return fmt.Errorf("failed to create git soundpack directory: %w", err)
	}
	stagingPath, err := os.MkdirTemp(filepath.Dir(clonePath), "."+name+"-clone-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary clone directory: %w", err)
	}
	stagingPresent := true
	defer func() {
		if stagingPresent {
			if err := removeManagedGitClone(stagingPath); err != nil {
				slog.Warn("failed to remove staging clone", "path", stagingPath, "error", err)
			}
		}
	}()
	ctx := commandContext(cmd)
	if _, err := runGit(ctx, "", "clone", "--", req.url, stagingPath); err != nil {
		return fmt.Errorf("failed to clone soundpack repo: %w", err)
	}
	if req.ref != "" {
		if err := checkoutGitRef(ctx, stagingPath, req.ref); err != nil {
			return fmt.Errorf("failed to check out ref %q: %w", req.ref, err)
		}
	}

	stagedPlayablePath, err := determineGitSoundpackPath(stagingPath, name, req.subdir)
	if err != nil {
		return err
	}
	if !req.skipValidate {
		if err := validateSoundpackInstallPath(stagedPlayablePath); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	commit, err := currentGitCommit(ctx, stagingPath)
	if err != nil {
		return err
	}
	playableRelative, err := filepath.Rel(stagingPath, stagedPlayablePath)
	if err != nil {
		return fmt.Errorf("failed to resolve staged soundpack path: %w", err)
	}
	playablePath := filepath.Join(clonePath, playableRelative)

	now := time.Now().UTC().Format(time.RFC3339)
	installedAt := now
	if exists && existing.InstalledAt != "" {
		installedAt = existing.InstalledAt
	}
	record := gitSoundpackRecord{
		Name:           name,
		SourceType:     gitSoundpackSourceType,
		URL:            req.url,
		Ref:            req.ref,
		ResolvedCommit: commit,
		Subdir:         cleanedSubdir,
		Path:           clonePath,
		InstalledAt:    installedAt,
		UpdatedAt:      now,
	}

	backupPath := ""
	activated := false
	err = withRegistry(func(registry *soundpackRegistry) error {
		var activateErr error
		backupPath, activateErr = activateManagedClone(stagingPath, clonePath)
		if activateErr != nil {
			return activateErr
		}
		stagingPresent = false
		activated = true
		registry.Packs[name] = record
		return nil
	})
	if err != nil {
		if activated {
			if rollbackErr := rollbackManagedClone(clonePath, backupPath); rollbackErr != nil {
				return fmt.Errorf("%w (also failed to restore previous clone: %w)", err, rollbackErr)
			}
		}
		return err
	}
	if backupPath != "" {
		if err := removeManagedGitClone(backupPath); err != nil {
			slog.Warn("failed to remove previous managed clone backup", "path", backupPath, "error", err)
		}
	}
	if err := updateConfigForManagedGitInstall(cmd, playablePath, clonePath, name, req.setDefault); err != nil {
		return err
	}

	cmd.Printf("Added git soundpack '%s' from %s\n", name, req.source)
	cmd.Printf("Path: %s\n", playablePath)
	return nil
}

// repairExistingGitSoundpack handles `add` for a name that is already
// registered with the same source: it re-checks the clone and repairs the
// config entry instead of cloning again.
func repairExistingGitSoundpack(cmd *cobra.Command, req gitAddRequest, existing gitSoundpackRecord, cleanedSubdir, clonePath string) error {
	name := req.name
	if existing.SourceType != gitSoundpackSourceType || existing.URL != req.url || existing.Ref != req.ref || existing.Subdir != cleanedSubdir || filepath.Clean(existing.Path) != filepath.Clean(clonePath) {
		return fmt.Errorf("managed git soundpack %q already exists; use --replace to replace it", name)
	}
	playablePath := playablePathForRecord(existing)
	if _, err := os.Stat(playablePath); err != nil {
		return fmt.Errorf("managed git soundpack %q exists but is not accessible; use --replace to repair it: %w", name, err)
	}
	if !req.skipValidate {
		if err := validateSoundpackInstallPath(playablePath); err != nil {
			return fmt.Errorf("managed git soundpack %q exists but is not usable; use --replace to repair it: %w", name, err)
		}
	}
	if err := updateConfigForManagedGitInstall(cmd, playablePath, clonePath, name, req.setDefault); err != nil {
		return err
	}
	cmd.Printf("Managed git soundpack '%s' already exists; repaired config\n", name)
	return nil
}

// activateManagedClone moves stagingPath to clonePath, first moving any
// existing clone aside. It returns the backup path, or "" when there was
// no previous clone. On failure the previous clone is restored.
func activateManagedClone(stagingPath, clonePath string) (string, error) {
	backupPath := ""
	if _, statErr := os.Stat(clonePath); statErr == nil {
		backupPath = stagingPath + ".previous"
		if err := os.Rename(clonePath, backupPath); err != nil {
			return "", fmt.Errorf("failed to preserve existing managed clone: %w", err)
		}
	} else if !os.IsNotExist(statErr) {
		return "", fmt.Errorf("failed to inspect managed clone before activation: %w", statErr)
	}
	if err := os.Rename(stagingPath, clonePath); err != nil {
		if backupPath != "" {
			if restoreErr := os.Rename(backupPath, clonePath); restoreErr != nil {
				return "", fmt.Errorf("failed to activate managed clone: %w; previous clone remains at %s because restoration failed: %w", err, backupPath, restoreErr)
			}
		}
		return "", fmt.Errorf("failed to activate managed clone: %w", err)
	}
	return backupPath, nil
}

// rollbackManagedClone undoes activateManagedClone: it removes the new
// clone and moves the backup (if any) back into place.
func rollbackManagedClone(clonePath, backupPath string) error {
	if err := removeManagedGitClone(clonePath); err != nil {
		return fmt.Errorf("failed to remove new clone: %w", err)
	}
	if backupPath != "" {
		if err := os.Rename(backupPath, clonePath); err != nil {
			return fmt.Errorf("failed to restore previous clone from %s: %w", backupPath, err)
		}
	}
	return nil
}

func runSoundpackUpdate(cmd *cobra.Command, name string, all, force bool) error {
	if err := requireGit(); err != nil {
		return err
	}

	registry, err := loadSoundpackRegistry()
	if err != nil {
		return err
	}

	names := []string{name}
	if all {
		names = make([]string, 0, len(registry.Packs))
		for packName := range registry.Packs {
			names = append(names, packName)
		}
		sort.Strings(names)
	}

	for _, packName := range names {
		updated, err := updateRegisteredGitSoundpack(commandContext(cmd), packName, force)
		if err != nil {
			return err
		}
		cmd.Printf("Updated git soundpack '%s' to %s\n", packName, shortCommit(updated.ResolvedCommit))
	}

	return nil
}

func updateRegisteredGitSoundpack(ctx context.Context, name string, force bool) (gitSoundpackRecord, error) {
	var updated gitSoundpackRecord
	err := withNameLock(name, func() error {
		record, exists, err := readRecord(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("managed git soundpack %q not found", name)
		}
		updated, err = updateGitSoundpack(ctx, record, force)
		if err != nil {
			return err
		}
		return withRegistry(func(registry *soundpackRegistry) error {
			if _, exists := registry.Packs[name]; !exists {
				return fmt.Errorf("managed git soundpack %q was removed during update", name)
			}
			registry.Packs[name] = updated
			return nil
		})
	})
	if err != nil {
		return gitSoundpackRecord{}, err
	}
	return updated, nil
}

func runSoundpackRemove(cmd *cobra.Command, name string, keepFiles, force bool) error {
	if err := validateConfigMutationTarget(cmd); err != nil {
		return fmt.Errorf("failed to load config before removing soundpack: %w", err)
	}
	return withNameLock(name, func() error {
		return removeGitSoundpack(cmd, name, keepFiles, force)
	})
}

// removeGitSoundpack deletes a managed git soundpack's clone, registry
// record and config entries. The caller holds the per-name lock.
func removeGitSoundpack(cmd *cobra.Command, name string, keepFiles, force bool) error {
	record, exists, err := readRecord(name)
	if err != nil {
		return err
	}
	if !exists {
		clonePath := filepath.Join(gitSoundpackBaseDir(), name)
		changed, err := removeConfigSoundpackPath(cmd, clonePath, clonePath, name)
		if err != nil {
			return err
		}
		if !changed {
			return fmt.Errorf("managed git soundpack %q not found", name)
		}
		cmd.Printf("Managed git soundpack '%s' was already removed; repaired config\n", name)
		return nil
	}
	playablePath := playablePathForRecord(record)

	if !keepFiles {
		if err := removeManagedGitClone(record.Path); err != nil {
			if !force {
				return err
			}
			slog.Warn("failed to remove managed git clone", "path", record.Path, "error", err)
		}
	}

	if err := withRegistry(func(registry *soundpackRegistry) error {
		delete(registry.Packs, name)
		return nil
	}); err != nil {
		return err
	}
	if _, err := removeConfigSoundpackPath(cmd, playablePath, record.Path, name); err != nil {
		return err
	}

	cmd.Printf("Removed git soundpack '%s'\n", name)
	return nil
}

func runSoundpackStatus(cmd *cobra.Command, name string) error {
	registry, err := loadSoundpackRegistry()
	if err != nil {
		return err
	}

	records := make([]gitSoundpackRecord, 0)
	if name != "" {
		record, exists := registry.Packs[name]
		if !exists {
			return fmt.Errorf("managed git soundpack %q not found", name)
		}
		records = append(records, record)
	} else {
		names := make([]string, 0, len(registry.Packs))
		for packName := range registry.Packs {
			names = append(names, packName)
		}
		sort.Strings(names)
		for _, packName := range names {
			records = append(records, registry.Packs[packName])
		}
	}

	cmd.Printf("%-24s%-10s%-12s%-16s%s\n", "NAME", "STATE", "COMMIT", "REF", "PATH")
	for _, record := range records {
		state := "missing"
		if _, err := os.Stat(record.Path); err == nil {
			state = "clean"
			dirty, err := gitWorktreeDirty(commandContext(cmd), record.Path)
			if err != nil {
				state = "error"
			} else if dirty {
				state = "dirty"
			}
		}
		cmd.Printf("%-24s%-10s%-12s%-16s%s\n",
			record.Name,
			state,
			shortCommit(record.ResolvedCommit),
			displayRef(record.Ref),
			playablePathForRecord(record))
	}

	return nil
}

func updateGitSoundpack(ctx context.Context, record gitSoundpackRecord, force bool) (gitSoundpackRecord, error) {
	if err := validateGitRef(record.Ref); err != nil {
		return record, fmt.Errorf("managed git soundpack %q has an invalid ref: %w", record.Name, err)
	}
	if _, err := os.Stat(record.Path); err != nil {
		return record, fmt.Errorf("managed clone for %q is missing: %w", record.Name, err)
	}

	dirty, err := gitWorktreeDirty(ctx, record.Path)
	if err != nil {
		return record, err
	}
	if dirty {
		if !force {
			return record, fmt.Errorf("managed clone for %q has local changes; use --force to discard them", record.Name)
		}
		if _, err := runGit(ctx, record.Path, "reset", "--hard"); err != nil {
			return record, err
		}
		if _, err := runGit(ctx, record.Path, "clean", "-fd"); err != nil {
			return record, err
		}
	}

	previousCommit := record.ResolvedCommit
	if previousCommit == "" {
		// Without a known-good commit a failed update could not be rolled
		// back, so refuse to start.
		previousCommit, err = currentGitCommit(ctx, record.Path)
		if err != nil {
			return record, fmt.Errorf("cannot determine current commit for %q: %w", record.Name, err)
		}
	}

	if _, err := runGit(ctx, record.Path, "fetch", "--all", "--tags", "--prune"); err != nil {
		return record, fmt.Errorf("failed to fetch updates for %q: %w", record.Name, err)
	}
	if record.Ref != "" {
		if err := checkoutGitRef(ctx, record.Path, record.Ref); err != nil {
			return record, fmt.Errorf("failed to check out ref %q for %q: %w", record.Ref, record.Name, err)
		}
	}
	branch, err := currentGitBranch(ctx, record.Path)
	if err != nil {
		return record, fmt.Errorf("failed to inspect branch of %q: %w", record.Name, err)
	}
	if branch != "" {
		if _, err := runGit(ctx, record.Path, "pull", "--ff-only"); err != nil {
			return record, fmt.Errorf("failed to update %q: %w", record.Name, err)
		}
	}

	playablePath := playablePathForRecord(record)
	if err := validateSoundpackInstallPath(playablePath); err != nil {
		if _, resetErr := runGit(ctx, record.Path, "reset", "--hard", previousCommit); resetErr != nil {
			return record, fmt.Errorf("validation failed after update for %q: %w (rollback to %s also failed: %w)", record.Name, err, shortCommit(previousCommit), resetErr)
		}
		return record, fmt.Errorf("validation failed after update for %q: %w", record.Name, err)
	}

	commit, err := currentGitCommit(ctx, record.Path)
	if err != nil {
		return record, err
	}
	record.ResolvedCommit = commit
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return record, nil
}

func discoverManagedGitSoundpacks() []soundpackInfo {
	registry, err := loadSoundpackRegistry()
	if err != nil {
		slog.Debug("could not load managed git soundpack registry", "error", err)
		return nil
	}

	packs := make([]soundpackInfo, 0, len(registry.Packs))
	for _, record := range registry.Packs {
		playablePath := playablePathForRecord(record)
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

func loadSoundpackRegistry() (*soundpackRegistry, error) {
	registry := &soundpackRegistry{
		Version: gitSoundpackRegistryVersion,
		Packs:   make(map[string]gitSoundpackRecord),
	}

	// Registry content is attacker-influenced: every `claudio soundpack
	// add gh:...` mutates this file with metadata derived from the
	// source repo. A malicious or compromised process could also
	// overwrite the file outright with a multi-GiB payload to OOM the
	// next CLI invocation. Cap the read at MaxSoundpackJSONBytes — same
	// 10 MiB cap used for soundpack JSONs, ample for tens of thousands
	// of registry entries.
	f, err := os.Open(soundpackRegistryPath())
	if os.IsNotExist(err) {
		return registry, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open soundpack registry: %w", err)
	}
	defer f.Close()

	data, err := safeio.ReadAllCapped(f, safeio.MaxSoundpackJSONBytes, "soundpack registry")
	if err != nil {
		return nil, fmt.Errorf("failed to read soundpack registry: %w", err)
	}
	if err := json.Unmarshal(data, registry); err != nil {
		return nil, fmt.Errorf("failed to parse soundpack registry: %w", err)
	}
	if registry.Packs == nil {
		registry.Packs = make(map[string]gitSoundpackRecord)
	}
	if registry.Version == 0 {
		registry.Version = gitSoundpackRegistryVersion
	}
	return registry, nil
}

func saveSoundpackRegistry(registry *soundpackRegistry) error {
	if registry.Version == 0 {
		registry.Version = gitSoundpackRegistryVersion
	}
	if registry.Packs == nil {
		registry.Packs = make(map[string]gitSoundpackRecord)
	}
	if err := safeio.WriteJSONFile(afero.NewOsFs(), soundpackRegistryPath(), registry, ".soundpacks-*.tmp"); err != nil {
		return fmt.Errorf("failed to save soundpack registry: %w", err)
	}
	return nil
}

func soundpackRegistryPath() string {
	return config.UserConfigPath("soundpacks.json")
}

func lockSoundpackRegistry() (*flock.Flock, error) {
	path := soundpackRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create soundpack registry directory: %w", err)
	}
	lock, err := safeio.LockFile(path + ".lock")
	if errors.Is(err, safeio.ErrLockHeld) {
		return nil, fmt.Errorf("another soundpack registry write is already running")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock soundpack registry: %w", err)
	}
	return lock, nil
}

func lockSoundpackName(name string) (*flock.Flock, error) {
	if err := validateManagedSoundpackName(name); err != nil {
		return nil, err
	}
	dir := gitSoundpackBaseDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create managed soundpack directory: %w", err)
	}
	lock, err := safeio.LockFile(filepath.Join(dir, "."+name+".lock"))
	if errors.Is(err, safeio.ErrLockHeld) {
		return nil, fmt.Errorf("another operation for managed soundpack %q is already running", name)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lock managed soundpack %q: %w", name, err)
	}
	return lock, nil
}

// withNameLock runs fn while holding the per-name operation lock, so add,
// update, remove and install of one name never interleave.
func withNameLock(name string, fn func() error) error {
	lock, err := lockSoundpackName(name)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release soundpack name lock", "name", name, "error", unlockErr)
		}
	}()
	return fn()
}

// underRegistryLock runs fn on the registry loaded under the registry lock.
func underRegistryLock(fn func(*soundpackRegistry) error) error {
	lock, err := lockSoundpackRegistry()
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release soundpack registry lock", "error", unlockErr)
		}
	}()
	registry, err := loadSoundpackRegistry()
	if err != nil {
		return err
	}
	return fn(registry)
}

// withRegistry is the registry's locked read-modify-write: it loads the
// registry under the registry lock, lets fn mutate it, and saves it when fn
// returns nil. Nothing is saved when fn fails.
func withRegistry(fn func(*soundpackRegistry) error) error {
	return underRegistryLock(func(registry *soundpackRegistry) error {
		if err := fn(registry); err != nil {
			return err
		}
		return saveSoundpackRegistry(registry)
	})
}

// readRecord returns the registry record for name, read under the
// registry lock.
func readRecord(name string) (gitSoundpackRecord, bool, error) {
	var record gitSoundpackRecord
	var exists bool
	err := underRegistryLock(func(registry *soundpackRegistry) error {
		record, exists = registry.Packs[name]
		return nil
	})
	return record, exists, err
}

func gitSoundpackBaseDir() string {
	return config.UserDataPath("soundpack-repos")
}

func determineGitSoundpackPath(clonePath, name, subdir string) (string, error) {
	if subdir != "" {
		playablePath := filepath.Join(clonePath, filepath.FromSlash(filepath.Clean(subdir)))
		if _, err := os.Stat(playablePath); err != nil {
			return "", fmt.Errorf("soundpack subdir does not exist: %w", err)
		}
		return playablePath, nil
	}

	for _, candidate := range []string{
		filepath.Join(clonePath, "soundpack.json"),
		filepath.Join(clonePath, name+".json"),
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return clonePath, nil
}

func playablePathForRecord(record gitSoundpackRecord) string {
	if record.Subdir == "" {
		playablePath, err := determineGitSoundpackPath(record.Path, record.Name, "")
		if err == nil {
			return playablePath
		}
		return record.Path
	}
	return filepath.Join(record.Path, filepath.FromSlash(record.Subdir))
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

func updateConfigForManagedGitInstall(cmd *cobra.Command, playablePath, clonePath, name string, setDefault bool) error {
	return mutateConfigForCommand(cmd, func(cfg *config.Config) error {
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

func removeConfigSoundpackPath(cmd *cobra.Command, playablePath, clonePath, removedName string) (bool, error) {
	changed := false
	err := mutateConfigForCommand(cmd, func(cfg *config.Config) error {
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

// countJSONMappings returns the number of non-empty mappings in an
// on-disk soundpack JSON. Routes through PeekJSONSoundpackMetadataFromFile
// so the size cap (MaxSoundpackJSONBytes) and the 10K mappings cap apply
// — the JSON path here came from a user-supplied gh:owner/repo source and
// must not be read raw.
func countJSONMappings(jsonPath string) int {
	spFile, err := soundpack.PeekJSONSoundpackMetadataFromFile(jsonPath)
	if err != nil {
		return 0
	}
	return countNonEmptyMappings(spFile.Mappings)
}

func requireGit() error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git executable not found in PATH")
	}
	return nil
}

// gitCommandTimeout bounds every git invocation so a hung remote cannot
// wedge the CLI.
const gitCommandTimeout = 2 * time.Minute

// gitCommand builds a non-interactive git invocation. GIT_TERMINAL_PROMPT=0
// makes git fail instead of blocking on a credential prompt nobody can
// answer (hooks and detached workers have no terminal).
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	return cmd
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

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	cmd := gitCommand(ctx, dir, args...)
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if ctx.Err() == context.DeadlineExceeded {
		return text, fmt.Errorf("git %s timed out", strings.Join(args, " "))
	}
	if err != nil {
		if text != "" {
			return text, fmt.Errorf("git %s failed: %s", strings.Join(args, " "), text)
		}
		return text, fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return text, nil
}

// validateGitRef rejects refs git would parse as an option. Refs come from
// the --ref flag and from the registry file, which is attacker-influenced.
func validateGitRef(ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid git ref %q: refs may not start with '-'", ref)
	}
	return nil
}

// checkoutGitRef detaches HEAD at ref. The remote-tracking branch
// (origin/<ref>) wins over a local name so branch refs advance after a
// fetch; tags and commit ids resolve directly. --detach makes git treat the
// argument as a commit, never as a pathspec.
func checkoutGitRef(ctx context.Context, repoPath, ref string) error {
	if err := validateGitRef(ref); err != nil {
		return err
	}
	commit, err := resolveGitRef(ctx, repoPath, ref)
	if err != nil {
		return err
	}
	_, err = runGit(ctx, repoPath, "checkout", "--detach", commit)
	return err
}

func resolveGitRef(ctx context.Context, repoPath, ref string) (string, error) {
	for _, candidate := range []string{"refs/remotes/origin/" + ref, ref} {
		commit, err := runGit(ctx, repoPath, "rev-parse", "--verify", "--quiet", candidate+"^{commit}")
		if err == nil && commit != "" {
			return commit, nil
		}
	}
	return "", fmt.Errorf("cannot resolve git ref %q", ref)
}

func currentGitCommit(ctx context.Context, repoPath string) (string, error) {
	commit, err := runGit(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commit), nil
}

func currentGitBranch(ctx context.Context, repoPath string) (string, error) {
	branch, err := runGit(ctx, repoPath, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		// With -q a detached HEAD exits 1 with no output; that means "no
		// branch", not failure. Anything else is a real error.
		var exitErr *exec.ExitError
		if branch == "" && errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(branch), nil
}

func gitWorktreeDirty(ctx context.Context, repoPath string) (bool, error) {
	status, err := runGit(ctx, repoPath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) != "", nil
}

func removeManagedGitClone(clonePath string) error {
	cleanBase, err := filepath.Abs(gitSoundpackBaseDir())
	if err != nil {
		return err
	}
	cleanTarget, err := filepath.Abs(clonePath)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return fmt.Errorf("refusing to remove path outside managed soundpack directory: %s", clonePath)
	}
	return os.RemoveAll(cleanTarget)
}

func nameFromGitURL(url string) string {
	normalized := strings.TrimRight(strings.ReplaceAll(url, "\\", "/"), "/")
	base := path.Base(normalized)
	base = strings.TrimSuffix(base, ".git")
	return sanitizeManagedSoundpackName(base)
}

func expandGitSoundpackSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("git source cannot be empty")
	}

	if strings.HasPrefix(source, "gh:") {
		repo := strings.TrimPrefix(source, "gh:")
		if err := validateGitHubAliasRepo(repo); err != nil {
			return "", err
		}
		return "https://github.com/" + repo + ".git", nil
	}

	return source, nil
}

func validateGitHubAliasRepo(repo string) error {
	if repo == "" {
		return fmt.Errorf("gh alias must be in the form gh:owner/repo")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("gh alias must be in the form gh:owner/repo")
	}
	for _, part := range parts {
		if part == "." || part == ".." || sanitizeManagedSoundpackName(part) != part {
			return fmt.Errorf("gh alias contains invalid repository path: %s", repo)
		}
	}
	return nil
}

func sanitizeManagedSoundpackName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-.")
}

func validateManagedSoundpackName(name string) error {
	if name == "" {
		return fmt.Errorf("soundpack name cannot be empty")
	}
	if name != sanitizeManagedSoundpackName(name) {
		return fmt.Errorf("soundpack name %q may only contain letters, numbers, '.', '_', and '-'", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid soundpack name: %q", name)
	}
	return nil
}

func validateGitSubdir(subdir string) error {
	if subdir == "" {
		return nil
	}
	cleaned := filepath.Clean(subdir)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("subdir must be a relative path inside the repository")
	}
	return nil
}

func shortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}

func displayRef(ref string) string {
	if ref == "" {
		return "(default)"
	}
	return ref
}
