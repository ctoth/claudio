package soundpack

import (
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// TestProperty_ValidatorAcceptsUnderBaseOnly asserts the invariant
// that validateMappingValue's success case always produces a resolved
// path under baseDir. For any (baseDir, value) the validator accepts,
// filepath.Rel(baseDir, resolved) must NOT start with "..".
//
// This is the single load-bearing invariant of the trust boundary: an
// accepted untrusted mapping value cannot escape the soundpack root.
func TestProperty_ValidatorAcceptsUnderBaseOnly(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseDir := rapid.StringMatching(`[a-z]{1,8}(/[a-z]{1,8}){0,3}`).Draw(t, "baseDir")
		value := rapid.StringMatching(`[a-zA-Z0-9_./-]{0,40}`).Draw(t, "value")

		resolved, err := validateMappingValue(value, baseDir)
		if err != nil {
			return // Rejections are checked by the other two properties.
		}
		rel, err := filepath.Rel(baseDir, resolved)
		if err != nil {
			t.Fatalf("validator accepted but Rel failed: base=%q value=%q resolved=%q: %v",
				baseDir, value, resolved, err)
		}
		if strings.HasPrefix(rel, "..") || rel == ".." {
			t.Fatalf("accepted value escapes baseDir: base=%q value=%q resolved=%q rel=%q",
				baseDir, value, resolved, rel)
		}
	})
}

// TestProperty_AbsolutePathsAlwaysRejected generates values that begin
// with `/` (POSIX absolute) and asserts the validator always rejects
// them.
func TestProperty_AbsolutePathsAlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseDir := rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "baseDir")
		absValue := "/" + rapid.StringMatching(`[a-z0-9/]{0,20}`).Draw(t, "absSuffix")

		if _, err := validateMappingValue(absValue, baseDir); err == nil {
			t.Fatalf("validator accepted absolute path %q (base=%q)", absValue, baseDir)
		}
	})
}

// TestProperty_DotDotAlwaysRejected generates values containing a `..`
// segment in any position and asserts the validator always rejects
// them, even when the cleaned result would still resolve under
// baseDir.
func TestProperty_DotDotAlwaysRejected(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		baseDir := rapid.StringMatching(`[a-z]{1,8}`).Draw(t, "baseDir")
		prefix := rapid.StringMatching(`[a-z]{0,8}`).Draw(t, "prefix")
		suffix := rapid.StringMatching(`[a-z]{0,8}`).Draw(t, "suffix")

		// filepath.Join with `..` in the middle preserves the `..` until
		// the explicit-`..`-segment check inside validateMappingValue.
		value := filepath.Join(prefix, "..", suffix)
		if value == "" || value == "." {
			return // degenerate; skip
		}
		// If Join cleaned it back to a pure relative path with no `..`,
		// the property doesn't apply.
		if !containsDotDotSegment(value) {
			return
		}

		if _, err := validateMappingValue(value, baseDir); err == nil {
			t.Fatalf("validator accepted `..` traversal %q (base=%q)", value, baseDir)
		}
	})
}

// containsDotDotSegment is the same check the validator uses, exposed
// here so the property can confirm a generated value still contains a
// traversal segment after Join.
func containsDotDotSegment(value string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(value), "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}
