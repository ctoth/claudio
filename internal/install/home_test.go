package install

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestNormalizeMSYSPathPortable(t *testing.T) {
	// normalizeMSYSPath uses filepath.FromSlash, which differs by OS, so build
	// expectations portably rather than hardcoding backslashes.
	cases := map[string]string{
		"/c/Users/testuser": "C:" + filepath.FromSlash("/Users/testuser"),
		"/d/work":           "D:" + filepath.FromSlash("/work"),
		"already/plain":     "already/plain",
		"":                  "",
		"/notdrive":         "/notdrive",
	}
	for in, want := range cases {
		if got := normalizeMSYSPath(in); got != want {
			t.Errorf("normalizeMSYSPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetHomeDirectoryWindowsBranches(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only home resolution branches")
	}
	// MSYS-style HOME when USERPROFILE absent
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOME", "/c/Users/testuser")
	if got := getHomeDirectory(); got != `C:\Users\testuser` {
		t.Errorf("MSYS HOME normalization: got %q", got)
	}
	// HOMEDRIVE + HOMEPATH fallback
	t.Setenv("HOME", "")
	t.Setenv("HOMEDRIVE", "D:")
	t.Setenv("HOMEPATH", `\Users\testuser`)
	if got := getHomeDirectory(); got != `D:\Users\testuser` {
		t.Errorf("HOMEDRIVE+HOMEPATH: got %q", got)
	}
	// Nothing set -> empty
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	if got := getHomeDirectory(); got != "" {
		t.Errorf("expected empty home when no env set, got %q", got)
	}
}

func TestGetHomeDirectoryUnixBranches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only home resolution branch")
	}
	t.Setenv("HOME", "/home/test")
	if got := getHomeDirectory(); got != "/home/test" {
		t.Errorf("expected /home/test, got %q", got)
	}
	t.Setenv("HOME", "")
	if got := getHomeDirectory(); got != "" {
		t.Errorf("expected empty home when HOME unset, got %q", got)
	}
}
