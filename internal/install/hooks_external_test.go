package install_test

import (
	"testing"

	"claudio.click/internal/install"
)

// TestIsClaudioHookRejectsDotTestPaths asserts the production hook
// recognizer does NOT match basenames ending in ".test" — a user hook
// command like "/usr/local/bin/lint.test" must not be classified as
// Claudio. Regression test for review finding #11 (Chunk 3 / F2).
//
// This test lives in package install_test (external test package) so it
// compiles into the install test binary WITHOUT the ability to mutate the
// unexported executableRecognizer. That means it exercises the real
// production classifier, not any in-package test override.
func TestIsClaudioHookRejectsDotTestPaths(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"unix go test binary path", "/usr/local/bin/install.test"},
		{"unix arbitrary .test suffix", "/usr/local/bin/lint.test"},
		{"windows go test binary path", `C:\Users\dev\install.test.exe`},
		{"windows arbitrary .test.exe", `C:\Users\dev\lint.test.exe`},
		{"bare basename install.test", "install.test"},
		{"bare basename uninstall.test", "uninstall.test"},
		{"bare basename cli.test", "cli.test"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if install.IsClaudioHook(tc.cmd) {
				t.Errorf("IsClaudioHook(%q) = true; want false", tc.cmd)
			}
		})
	}
}

// TestIsClaudioHookAcceptsProductionNames asserts the recognizer DOES
// match the production names — sanity check that the regression above
// didn't accidentally over-restrict.
func TestIsClaudioHookAcceptsProductionNames(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"bare claudio", "claudio"},
		{"unix absolute", "/usr/local/bin/claudio"},
		{"windows claudio.exe", `C:\Program Files\claudio.exe`},
		{"bare claudio.exe", "claudio.exe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !install.IsClaudioHook(tc.cmd) {
				t.Errorf("IsClaudioHook(%q) = false; want true", tc.cmd)
			}
		})
	}
}
