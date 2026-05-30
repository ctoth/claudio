package install

import "testing"

// SwapExecutableRecognizer is a TEST-ONLY seam that overrides the
// package-private executableRecognizer for the duration of the test,
// restoring the previous value via t.Cleanup.
//
// Production code MUST NOT call this — the *testing.T parameter makes
// it impossible to invoke from non-test code without importing the
// testing package. This is the idiomatic Go pattern for exposing a
// test seam across package boundaries (sibling tests in
// internal/cli's end-to-end install test need to install a recognizer
// that accepts the go test binary's basename, e.g. "cli.test.exe",
// because the production default — which accepts only "claudio" and
// "claudio.exe" — would otherwise cause the install workflow's
// verify step at install_command.go:270 to fail when run from within
// `go test`).
//
// Chunk 3d's analyst F2 deferred this helper to "when Chunk 18 lands
// an end-to-end install test"; Chunk 18 lands it.
func SwapExecutableRecognizer(t *testing.T, fn func(string) bool) {
	t.Helper()
	prev := executableRecognizer
	executableRecognizer = fn
	t.Cleanup(func() { executableRecognizer = prev })
}
