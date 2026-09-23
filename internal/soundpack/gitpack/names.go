package gitpack

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// NameFromURL derives a managed pack name from a git URL's last element.
func NameFromURL(url string) string {
	normalized := strings.TrimRight(strings.ReplaceAll(url, "\\", "/"), "/")
	base := path.Base(normalized)
	base = strings.TrimSuffix(base, ".git")
	return sanitizeName(base)
}

// ExpandSource turns a `gh:owner/repo` alias into a GitHub URL; any other
// source is returned trimmed.
func ExpandSource(source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", errors.New("git source cannot be empty")
	}

	if repo, ok := strings.CutPrefix(source, "gh:"); ok {
		if err := validateGitHubAliasRepo(repo); err != nil {
			return "", err
		}
		return "https://github.com/" + repo + ".git", nil
	}

	return source, nil
}

func validateGitHubAliasRepo(repo string) error {
	if repo == "" {
		return errors.New("gh alias must be in the form gh:owner/repo")
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return errors.New("gh alias must be in the form gh:owner/repo")
	}
	for _, part := range parts {
		if part == "." || part == ".." || sanitizeName(part) != part {
			return fmt.Errorf("gh alias contains invalid repository path: %s", repo)
		}
	}
	return nil
}

func sanitizeName(name string) string {
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

// ValidateName rejects names that are not safe as a single path element.
func ValidateName(name string) error {
	if name == "" {
		return errors.New("soundpack name cannot be empty")
	}
	if name != sanitizeName(name) {
		return fmt.Errorf("soundpack name %q may only contain letters, numbers, '.', '_', and '-'", name)
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid soundpack name: %q", name)
	}
	return nil
}

// ValidateSubdir requires a relative path that stays inside the repository.
func ValidateSubdir(subdir string) error {
	if subdir == "" {
		return nil
	}
	cleaned := filepath.Clean(subdir)
	if filepath.IsAbs(cleaned) || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return errors.New("subdir must be a relative path inside the repository")
	}
	return nil
}

// ValidateRef rejects refs git would parse as an option. Refs come from
// the --ref flag and from the registry file, which is attacker-influenced.
func ValidateRef(ref string) error {
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("invalid git ref %q: refs may not start with '-'", ref)
	}
	return nil
}

// ShortCommit abbreviates a commit id to seven characters.
func ShortCommit(commit string) string {
	if len(commit) <= 7 {
		return commit
	}
	return commit[:7]
}

// DisplayRef shows an empty ref as "(default)".
func DisplayRef(ref string) string {
	if ref == "" {
		return "(default)"
	}
	return ref
}
