package install

import "testing"

// SwapExecutableRecognizer overrides the package-private
// executableRecognizer for the duration of the test, restoring the
// previous value via t.Cleanup. Test-only — production code never
// calls this. The Chunk 3d analyst's F2 deferred this helper to "when
// Chunk 18 lands an end-to-end install test"; Chunk 18 needs it
// because go test binaries are named e.g. cli.test.exe, which the
// production recognizer (which only accepts claudio / claudio.exe)
// would reject during the install workflow's verify step.
func SwapExecutableRecognizer(t *testing.T, fn func(string) bool) {
	t.Helper()
	prev := executableRecognizer
	executableRecognizer = fn
	t.Cleanup(func() { executableRecognizer = prev })
}
