// Package gitpack manages git-backed soundpacks: the clones under the XDG
// data dir, the registry (soundpacks.json) that records them, and the locks
// that keep concurrent add/update/remove/install operations apart.
package gitpack

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"claudio.click/internal/config"
	"claudio.click/internal/safeio"
	"github.com/gofrs/flock"
	"github.com/spf13/afero"
)

const (
	registryVersion = 1
	// SourceType is the source_type recorded for git-backed packs.
	SourceType = "git"
)

// Registry is the on-disk list of managed git soundpacks.
type Registry struct {
	Version int               `json:"version"`
	Packs   map[string]Record `json:"packs"`
}

// Record describes one managed git soundpack.
type Record struct {
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

// SortedNames returns the registered pack names in lexical order.
func (r *Registry) SortedNames() []string {
	names := make([]string, 0, len(r.Packs))
	for name := range r.Packs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// LoadRegistry reads the registry, returning an empty one when the file does
// not exist.
func LoadRegistry() (*Registry, error) {
	registry := &Registry{
		Version: registryVersion,
		Packs:   make(map[string]Record),
	}

	// Registry content is attacker-influenced: every `claudio soundpack
	// add gh:...` mutates this file with metadata derived from the
	// source repo. A malicious or compromised process could also
	// overwrite the file outright with a multi-GiB payload to OOM the
	// next CLI invocation. Cap the read at MaxSoundpackJSONBytes — same
	// 10 MiB cap used for soundpack JSONs, ample for tens of thousands
	// of registry entries.
	f, err := os.Open(RegistryPath())
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
		registry.Packs = make(map[string]Record)
	}
	if registry.Version == 0 {
		registry.Version = registryVersion
	}
	return registry, nil
}

// SaveRegistry writes the registry atomically.
func SaveRegistry(registry *Registry) error {
	if registry.Version == 0 {
		registry.Version = registryVersion
	}
	if registry.Packs == nil {
		registry.Packs = make(map[string]Record)
	}
	if err := safeio.WriteJSONFile(afero.NewOsFs(), RegistryPath(), registry, ".soundpacks-*.tmp"); err != nil {
		return fmt.Errorf("failed to save soundpack registry: %w", err)
	}
	return nil
}

// RegistryPath is the location of soundpacks.json.
func RegistryPath() string {
	return config.UserConfigPath("soundpacks.json")
}

// BaseDir is the directory that holds every managed clone.
func BaseDir() string {
	return config.UserDataPath("soundpack-repos")
}

// ClonePath is where the managed clone for name lives.
func ClonePath(name string) string {
	return filepath.Join(BaseDir(), name)
}

func lockRegistry() (*flock.Flock, error) {
	path := RegistryPath()
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

// LockName takes the per-name operation lock. Most callers want WithNameLock.
func LockName(name string) (*flock.Flock, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	dir := BaseDir()
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

// WithNameLock runs fn while holding the per-name operation lock, so add,
// update, remove and install of one name never interleave.
func WithNameLock(name string, fn func() error) error {
	lock, err := LockName(name)
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
func underRegistryLock(fn func(*Registry) error) error {
	lock, err := lockRegistry()
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := lock.Unlock(); unlockErr != nil {
			slog.Warn("failed to release soundpack registry lock", "error", unlockErr)
		}
	}()
	registry, err := LoadRegistry()
	if err != nil {
		return err
	}
	return fn(registry)
}

// withRegistry is the registry's locked read-modify-write: it loads the
// registry under the registry lock, lets fn mutate it, and saves it when fn
// returns nil. Nothing is saved when fn fails.
func withRegistry(fn func(*Registry) error) error {
	return underRegistryLock(func(registry *Registry) error {
		if err := fn(registry); err != nil {
			return err
		}
		return SaveRegistry(registry)
	})
}

// readRecord returns the registry record for name, read under the
// registry lock.
func readRecord(name string) (Record, bool, error) {
	var record Record
	var exists bool
	err := underRegistryLock(func(registry *Registry) error {
		record, exists = registry.Packs[name]
		return nil
	})
	return record, exists, err
}

// determinePlayablePath picks the path inside a clone that claudio plays:
// the subdir when given, else soundpack.json or <name>.json, else the clone.
func determinePlayablePath(clonePath, name, subdir string) (string, error) {
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

// PlayablePath is the path claudio plays for a registered pack.
func (r Record) PlayablePath() string {
	if r.Subdir == "" {
		playablePath, err := determinePlayablePath(r.Path, r.Name, "")
		if err == nil {
			return playablePath
		}
		return r.Path
	}
	return filepath.Join(r.Path, filepath.FromSlash(r.Subdir))
}

// RemoveClone deletes a path under BaseDir, refusing anything outside it.
func RemoveClone(clonePath string) error {
	cleanBase, err := filepath.Abs(BaseDir())
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
