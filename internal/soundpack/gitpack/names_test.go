package gitpack

import "testing"

func TestExpandSource_GitHubAlias(t *testing.T) {
	got, err := ExpandSource("gh:ctoth/whatever")
	if err != nil {
		t.Fatalf("ExpandSource returned error: %v", err)
	}

	want := "https://github.com/ctoth/whatever.git"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestExpandSource_RejectsInvalidGitHubAlias(t *testing.T) {
	_, err := ExpandSource("gh:ctoth")
	if err == nil {
		t.Fatal("expected invalid gh alias to fail")
	}
}

func TestValidateRefRejectsOptionLikeRefs(t *testing.T) {
	for _, ref := range []string{"-", "--orphan=evil", "-b", "--upload-pack=touch pwned"} {
		if err := ValidateRef(ref); err == nil {
			t.Errorf("ValidateRef(%q) accepted an option-like ref", ref)
		}
	}
	for _, ref := range []string{"", "main", "v1.2.3", "feature/x", "0123abcd"} {
		if err := ValidateRef(ref); err != nil {
			t.Errorf("ValidateRef(%q) rejected a valid ref: %v", ref, err)
		}
	}
}

func TestNameLockRejectsTraversal(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	lock, err := LockName("../outside")
	if err == nil {
		_ = lock.Unlock()
		t.Fatal("accepted traversal in lock name")
	}
}
