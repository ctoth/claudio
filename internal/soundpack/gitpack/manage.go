package gitpack

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// Validator reports whether the soundpack at path is usable. The caller
// supplies it so this package does not own soundpack validation policy.
type Validator func(path string) error

// AddRequest carries the validated inputs of `soundpack add`.
type AddRequest struct {
	URL          string
	Name         string
	Ref          string
	Subdir       string
	Replace      bool
	SkipValidate bool
}

// AddResult says where the pack landed. Repaired is true when the name was
// already registered with the same source and nothing was cloned.
type AddResult struct {
	PlayablePath string
	ClonePath    string
	Repaired     bool
}

// Add clones, validates and registers a managed git soundpack. The caller
// holds the per-name lock (WithNameLock) and owns the config update.
func Add(ctx context.Context, req AddRequest, validate Validator) (AddResult, error) {
	name := req.Name
	cleanedSubdir := ""
	if req.Subdir != "" {
		cleaned := filepath.ToSlash(filepath.Clean(req.Subdir))
		if cleaned != "." {
			cleanedSubdir = cleaned
		}
	}
	clonePath := ClonePath(name)

	existing, exists, err := readRecord(name)
	if err != nil {
		return AddResult{}, err
	}
	if exists && !req.Replace {
		return repairExisting(req, existing, cleanedSubdir, clonePath, validate)
	}

	if info, statErr := os.Stat(clonePath); statErr == nil {
		if !exists {
			return AddResult{}, fmt.Errorf("managed clone path exists without a registry record; refusing to replace unowned path: %s", clonePath)
		}
		if !info.IsDir() {
			return AddResult{}, fmt.Errorf("managed clone path is not a directory: %s", clonePath)
		}
	} else if !os.IsNotExist(statErr) {
		return AddResult{}, fmt.Errorf("failed to inspect managed clone path: %w", statErr)
	}
	if exists && filepath.Clean(existing.Path) != filepath.Clean(clonePath) {
		return AddResult{}, fmt.Errorf("managed git soundpack %q has an unexpected clone path; refusing replacement", name)
	}

	if err := os.MkdirAll(filepath.Dir(clonePath), 0755); err != nil {
		return AddResult{}, fmt.Errorf("failed to create git soundpack directory: %w", err)
	}
	stagingPath, err := os.MkdirTemp(filepath.Dir(clonePath), "."+name+"-clone-*")
	if err != nil {
		return AddResult{}, fmt.Errorf("failed to create temporary clone directory: %w", err)
	}
	stagingPresent := true
	defer func() {
		if stagingPresent {
			if err := RemoveClone(stagingPath); err != nil {
				slog.Warn("failed to remove staging clone", "path", stagingPath, "error", err)
			}
		}
	}()
	if _, err := runGit(ctx, "", "clone", "--", req.URL, stagingPath); err != nil {
		return AddResult{}, fmt.Errorf("failed to clone soundpack repo: %w", err)
	}
	if req.Ref != "" {
		if err := checkoutGitRef(ctx, stagingPath, req.Ref); err != nil {
			return AddResult{}, fmt.Errorf("failed to check out ref %q: %w", req.Ref, err)
		}
	}

	stagedPlayablePath, err := determinePlayablePath(stagingPath, name, req.Subdir)
	if err != nil {
		return AddResult{}, err
	}
	if !req.SkipValidate {
		if err := validate(stagedPlayablePath); err != nil {
			return AddResult{}, fmt.Errorf("validation failed: %w", err)
		}
	}

	commit, err := currentGitCommit(ctx, stagingPath)
	if err != nil {
		return AddResult{}, err
	}
	playableRelative, err := filepath.Rel(stagingPath, stagedPlayablePath)
	if err != nil {
		return AddResult{}, fmt.Errorf("failed to resolve staged soundpack path: %w", err)
	}
	playablePath := filepath.Join(clonePath, playableRelative)

	now := time.Now().UTC().Format(time.RFC3339)
	installedAt := now
	if exists && existing.InstalledAt != "" {
		installedAt = existing.InstalledAt
	}
	record := Record{
		Name:           name,
		SourceType:     SourceType,
		URL:            req.URL,
		Ref:            req.Ref,
		ResolvedCommit: commit,
		Subdir:         cleanedSubdir,
		Path:           clonePath,
		InstalledAt:    installedAt,
		UpdatedAt:      now,
	}

	backupPath := ""
	activated := false
	err = withRegistry(func(registry *Registry) error {
		var activateErr error
		backupPath, activateErr = activateClone(stagingPath, clonePath)
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
			if rollbackErr := rollbackClone(clonePath, backupPath); rollbackErr != nil {
				return AddResult{}, fmt.Errorf("%w (also failed to restore previous clone: %w)", err, rollbackErr)
			}
		}
		return AddResult{}, err
	}
	if backupPath != "" {
		if err := RemoveClone(backupPath); err != nil {
			slog.Warn("failed to remove previous managed clone backup", "path", backupPath, "error", err)
		}
	}
	return AddResult{PlayablePath: playablePath, ClonePath: clonePath}, nil
}

// repairExisting handles `add` for a name that is already registered with
// the same source: it re-checks the clone instead of cloning again, so the
// caller can repair the config entry.
func repairExisting(req AddRequest, existing Record, cleanedSubdir, clonePath string, validate Validator) (AddResult, error) {
	name := req.Name
	if existing.SourceType != SourceType || existing.URL != req.URL || existing.Ref != req.Ref || existing.Subdir != cleanedSubdir || filepath.Clean(existing.Path) != filepath.Clean(clonePath) {
		return AddResult{}, fmt.Errorf("managed git soundpack %q already exists; use --replace to replace it", name)
	}
	playablePath := existing.PlayablePath()
	if _, err := os.Stat(playablePath); err != nil {
		return AddResult{}, fmt.Errorf("managed git soundpack %q exists but is not accessible; use --replace to repair it: %w", name, err)
	}
	if !req.SkipValidate {
		if err := validate(playablePath); err != nil {
			return AddResult{}, fmt.Errorf("managed git soundpack %q exists but is not usable; use --replace to repair it: %w", name, err)
		}
	}
	return AddResult{PlayablePath: playablePath, ClonePath: clonePath, Repaired: true}, nil
}

// activateClone moves stagingPath to clonePath, first moving any existing
// clone aside. It returns the backup path, or "" when there was no previous
// clone. On failure the previous clone is restored.
func activateClone(stagingPath, clonePath string) (string, error) {
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

// rollbackClone undoes activateClone: it removes the new clone and moves the
// backup (if any) back into place.
func rollbackClone(clonePath, backupPath string) error {
	if err := RemoveClone(clonePath); err != nil {
		return fmt.Errorf("failed to remove new clone: %w", err)
	}
	if backupPath != "" {
		if err := os.Rename(backupPath, clonePath); err != nil {
			return fmt.Errorf("failed to restore previous clone from %s: %w", backupPath, err)
		}
	}
	return nil
}

// Update fetches and fast-forwards one registered pack under its name lock,
// validates the result (rolling back on failure) and records the new commit.
func Update(ctx context.Context, name string, force bool, validate Validator) (Record, error) {
	var updated Record
	err := WithNameLock(name, func() error {
		record, exists, err := readRecord(name)
		if err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("managed git soundpack %q not found", name)
		}
		updated, err = updateClone(ctx, record, force, validate)
		if err != nil {
			return err
		}
		return withRegistry(func(registry *Registry) error {
			if _, exists := registry.Packs[name]; !exists {
				return fmt.Errorf("managed git soundpack %q was removed during update", name)
			}
			registry.Packs[name] = updated
			return nil
		})
	})
	if err != nil {
		return Record{}, err
	}
	return updated, nil
}

func updateClone(ctx context.Context, record Record, force bool, validate Validator) (Record, error) {
	if err := ValidateRef(record.Ref); err != nil {
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

	playablePath := record.PlayablePath()
	if err := validate(playablePath); err != nil {
		if _, resetErr := runGit(ctx, record.Path, "reset", "--hard", previousCommit); resetErr != nil {
			return record, fmt.Errorf("validation failed after update for %q: %w (rollback to %s also failed: %w)", record.Name, err, ShortCommit(previousCommit), resetErr)
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

// RemoveResult names the paths whose config entries the caller should drop.
// Found is false when name has no registry record; the paths are then the
// default clone location, so a half-finished earlier removal can be repaired.
type RemoveResult struct {
	Found        bool
	PlayablePath string
	ClonePath    string
}

// Remove deletes a managed pack's clone (unless keepFiles) and its registry
// record. With force, a failed clone deletion is logged and the record is
// removed anyway. The caller holds the per-name lock and owns the config.
func Remove(name string, keepFiles, force bool) (RemoveResult, error) {
	record, exists, err := readRecord(name)
	if err != nil {
		return RemoveResult{}, err
	}
	if !exists {
		clonePath := ClonePath(name)
		return RemoveResult{PlayablePath: clonePath, ClonePath: clonePath}, nil
	}
	playablePath := record.PlayablePath()

	if !keepFiles {
		if err := RemoveClone(record.Path); err != nil {
			if !force {
				return RemoveResult{}, err
			}
			slog.Warn("failed to remove managed git clone", "path", record.Path, "error", err)
		}
	}

	if err := withRegistry(func(registry *Registry) error {
		delete(registry.Packs, name)
		return nil
	}); err != nil {
		return RemoveResult{}, err
	}
	return RemoveResult{Found: true, PlayablePath: playablePath, ClonePath: record.Path}, nil
}

// StatusEntry is one row of `soundpack status`.
type StatusEntry struct {
	Record
	// State is "missing", "clean", "dirty" or "error".
	State string
}

// Status reports the clone state of one registered pack, or of every pack
// (sorted by name) when name is empty.
func Status(ctx context.Context, name string) ([]StatusEntry, error) {
	registry, err := LoadRegistry()
	if err != nil {
		return nil, err
	}

	var records []Record
	if name != "" {
		record, exists := registry.Packs[name]
		if !exists {
			return nil, fmt.Errorf("managed git soundpack %q not found", name)
		}
		records = append(records, record)
	} else {
		for _, packName := range registry.SortedNames() {
			records = append(records, registry.Packs[packName])
		}
	}

	entries := make([]StatusEntry, 0, len(records))
	for _, record := range records {
		state := "missing"
		if _, err := os.Stat(record.Path); err == nil {
			state = "clean"
			dirty, err := gitWorktreeDirty(ctx, record.Path)
			if err != nil {
				state = "error"
			} else if dirty {
				state = "dirty"
			}
		}
		entries = append(entries, StatusEntry{Record: record, State: state})
	}
	return entries, nil
}
