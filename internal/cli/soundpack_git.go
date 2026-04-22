package cli

import (
	"context"
	"encoding/json"
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
	"claudio.click/internal/soundpack"
	"github.com/adrg/xdg"
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

	registry, err := loadSoundpackRegistry()
	if err != nil {
		return err
	}
	if _, exists := registry.Packs[name]; exists && !replace {
		return fmt.Errorf("managed git soundpack %q already exists; use --replace to replace it", name)
	}

	clonePath := filepath.Join(gitSoundpackBaseDir(), name)
	if _, err := os.Stat(clonePath); err == nil && !replace {
		return fmt.Errorf("managed clone path already exists: %s", clonePath)
	}
	if replace {
		if err := removeManagedGitClone(clonePath); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
		return fmt.Errorf("failed to create git soundpack directory: %w", err)
	}
	if _, err := runGit("", "clone", url, clonePath); err != nil {
		return fmt.Errorf("failed to clone soundpack repo: %w", err)
	}
	if ref != "" {
		if _, err := runGit(clonePath, "checkout", ref); err != nil {
			_ = removeManagedGitClone(clonePath)
			return fmt.Errorf("failed to check out ref %q: %w", ref, err)
		}
	}

	playablePath, err := determineGitSoundpackPath(clonePath, name, subdir)
	if err != nil {
		_ = removeManagedGitClone(clonePath)
		return err
	}
	if !skipValidate {
		if err := validateSoundpackInstallPath(playablePath); err != nil {
			_ = removeManagedGitClone(clonePath)
			return fmt.Errorf("validation failed: %w", err)
		}
	}

	commit, err := currentGitCommit(clonePath)
	if err != nil {
		_ = removeManagedGitClone(clonePath)
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	registry.Packs[name] = gitSoundpackRecord{
		Name:           name,
		SourceType:     gitSoundpackSourceType,
		URL:            url,
		Ref:            ref,
		ResolvedCommit: commit,
		Subdir:         filepath.ToSlash(filepath.Clean(subdir)),
		Path:           clonePath,
		InstalledAt:    now,
		UpdatedAt:      now,
	}
	if subdir == "" {
		registry.Packs[name] = gitSoundpackRecord{
			Name:           name,
			SourceType:     gitSoundpackSourceType,
			URL:            url,
			Ref:            ref,
			ResolvedCommit: commit,
			Path:           clonePath,
			InstalledAt:    now,
			UpdatedAt:      now,
		}
	}

	if err := saveSoundpackRegistry(registry); err != nil {
		return err
	}
	if err := updateConfigForInstall(playablePath, name, setDefault); err != nil {
		return err
	}

	cmd.Printf("Added git soundpack '%s' from %s\n", name, source)
	cmd.Printf("Path: %s\n", playablePath)
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
		record, exists := registry.Packs[packName]
		if !exists {
			return fmt.Errorf("managed git soundpack %q not found", packName)
		}
		updated, err := updateGitSoundpack(record, force)
		if err != nil {
			return err
		}
		registry.Packs[packName] = updated
		cmd.Printf("Updated git soundpack '%s' to %s\n", packName, shortCommit(updated.ResolvedCommit))
	}

	return saveSoundpackRegistry(registry)
}

func runSoundpackRemove(cmd *cobra.Command, name string, keepFiles, force bool) error {
	registry, err := loadSoundpackRegistry()
	if err != nil {
		return err
	}
	record, exists := registry.Packs[name]
	if !exists {
		return fmt.Errorf("managed git soundpack %q not found", name)
	}

	if !keepFiles {
		if err := removeManagedGitClone(record.Path); err != nil {
			if !force {
				return err
			}
			slog.Warn("failed to remove managed git clone", "path", record.Path, "error", err)
		}
	}

	playablePath := playablePathForRecord(record)
	if err := removeConfigSoundpackPath(playablePath, record.Path, name); err != nil {
		return err
	}

	delete(registry.Packs, name)
	if err := saveSoundpackRegistry(registry); err != nil {
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
			dirty, err := gitWorktreeDirty(record.Path)
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

func updateGitSoundpack(record gitSoundpackRecord, force bool) (gitSoundpackRecord, error) {
	if _, err := os.Stat(record.Path); err != nil {
		return record, fmt.Errorf("managed clone for %q is missing: %w", record.Name, err)
	}

	dirty, err := gitWorktreeDirty(record.Path)
	if err != nil {
		return record, err
	}
	if dirty {
		if !force {
			return record, fmt.Errorf("managed clone for %q has local changes; use --force to discard them", record.Name)
		}
		if _, err := runGit(record.Path, "reset", "--hard"); err != nil {
			return record, err
		}
		if _, err := runGit(record.Path, "clean", "-fd"); err != nil {
			return record, err
		}
	}

	previousCommit := record.ResolvedCommit
	if previousCommit == "" {
		previousCommit, _ = currentGitCommit(record.Path)
	}

	if _, err := runGit(record.Path, "fetch", "--all", "--tags", "--prune"); err != nil {
		return record, fmt.Errorf("failed to fetch updates for %q: %w", record.Name, err)
	}
	if record.Ref != "" {
		if _, err := runGit(record.Path, "checkout", record.Ref); err != nil {
			return record, fmt.Errorf("failed to check out ref %q for %q: %w", record.Ref, record.Name, err)
		}
	}
	if branch, _ := currentGitBranch(record.Path); branch != "" {
		if _, err := runGit(record.Path, "pull", "--ff-only"); err != nil {
			return record, fmt.Errorf("failed to update %q: %w", record.Name, err)
		}
	}

	playablePath := playablePathForRecord(record)
	if err := validateSoundpackInstallPath(playablePath); err != nil {
		if previousCommit != "" {
			_, _ = runGit(record.Path, "reset", "--hard", previousCommit)
		}
		return record, fmt.Errorf("validation failed after update for %q: %w", record.Name, err)
	}

	commit, err := currentGitCommit(record.Path)
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
		info, err := os.Stat(playablePath)
		if err != nil {
			continue
		}
		count := 0
		if info.IsDir() {
			count = countAudioFiles(playablePath)
		} else if strings.HasSuffix(strings.ToLower(playablePath), ".json") {
			count = countJSONMappings(playablePath)
		}
		packs = append(packs, soundpackInfo{
			Name:       record.Name,
			Type:       "git",
			SoundCount: count,
			Path:       playablePath,
		})
	}
	return packs
}

func findManagedGitSoundpackPath(name string) string {
	registry, err := loadSoundpackRegistry()
	if err != nil {
		return ""
	}
	record, exists := registry.Packs[name]
	if !exists {
		return ""
	}
	playablePath := playablePathForRecord(record)
	if _, err := os.Stat(playablePath); err != nil {
		return ""
	}
	return playablePath
}

func loadSoundpackRegistry() (*soundpackRegistry, error) {
	registry := &soundpackRegistry{
		Version: gitSoundpackRegistryVersion,
		Packs:   make(map[string]gitSoundpackRecord),
	}

	data, err := os.ReadFile(soundpackRegistryPath())
	if os.IsNotExist(err) {
		return registry, nil
	}
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
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal soundpack registry: %w", err)
	}
	path := soundpackRegistryPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create soundpack registry directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write soundpack registry: %w", err)
	}
	return nil
}

func soundpackRegistryPath() string {
	return filepath.Join(xdg.ConfigHome, "claudio", "soundpacks.json")
}

func gitSoundpackBaseDir() string {
	return filepath.Join(xdg.DataHome, "claudio", "soundpack-repos")
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

func validateSoundpackInstallPath(playablePath string) error {
	info, err := os.Stat(playablePath)
	if err != nil {
		return fmt.Errorf("cannot access soundpack path: %w", err)
	}
	if info.IsDir() {
		_, err = validateDirectorySoundpack(playablePath)
		return err
	}
	if strings.HasSuffix(strings.ToLower(playablePath), ".json") {
		_, err = validateJSONSoundpackFile(playablePath)
		return err
	}
	return fmt.Errorf("unsupported soundpack path: %s", playablePath)
}

func removeConfigSoundpackPath(playablePath, clonePath, removedName string) error {
	cm := config.NewConfigManager()
	cfg, err := cm.LoadConfig()
	if err != nil {
		cfg = cm.GetDefaultConfig()
	}

	filtered := cfg.SoundpackPaths[:0]
	for _, existingPath := range cfg.SoundpackPaths {
		cleanExisting := filepath.Clean(existingPath)
		if cleanExisting == filepath.Clean(playablePath) || cleanExisting == filepath.Clean(clonePath) {
			continue
		}
		filtered = append(filtered, existingPath)
	}
	cfg.SoundpackPaths = filtered

	if cfg.DefaultSoundpack == removedName {
		cfg.DefaultSoundpack = cm.GetDefaultConfig().DefaultSoundpack
	}

	configPath := filepath.Join(xdg.ConfigHome, "claudio", "config.json")
	return cm.SaveToFile(cfg, configPath)
}

func countJSONMappings(jsonPath string) int {
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return 0
	}
	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
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

func runGit(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
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

func currentGitCommit(repoPath string) (string, error) {
	commit, err := runGit(repoPath, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(commit), nil
}

func currentGitBranch(repoPath string) (string, error) {
	branch, err := runGit(repoPath, "symbolic-ref", "--short", "-q", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(branch), nil
}

func gitWorktreeDirty(repoPath string) (bool, error) {
	status, err := runGit(repoPath, "status", "--porcelain")
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
