package install

import "strings"

// init() runs only when the test binary is built. It extends the production
// executableRecognizer so end-to-end tests whose hook values flow through
// GetExecutablePath() (which returns names like "install.test" or "cli.test"
// under `go test`) still self-recognize. The production binary never sees
// this override.
func init() {
	base := executableRecognizer
	executableRecognizer = func(name string) bool {
		if base(name) {
			return true
		}
		return strings.HasSuffix(name, ".test") || strings.HasSuffix(name, ".test.exe")
	}
}
