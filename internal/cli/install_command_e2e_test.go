package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/install"
)

// TestRunInstallWorkflow_EndToEnd_NoDryRun exercises the full install
// workflow path: lock → read → generate → merge → write → verify-readback.
// Every existing install test in install_command_test.go passes --dry-run
// or constructs the cobra command directly without going through
// cli.Run; this test is the first to drive the production path end to
// end. Closes review finding #56.
//
// The test isolates HOME/XDG to t.TempDir() via testenv.IsolateXDG so
// no developer state is mutated, and opts in to the install package's
// go-test recognizer extension by setting
// CLAUDIO_TEST_RECOGNIZE_GO_TEST=1 so the verify step accepts the go
// test binary's basename (e.g. "cli.test.exe") as a valid claudio
// executable. The actual hook command path under test is the test
// binary itself — that's the "claudio" the install workflow advertises.
func TestRunInstallWorkflow_EndToEnd_NoDryRun(t *testing.T) {
	root := testenv.IsolateXDG(t)

	// Pre-create the .claude directory under the sandbox so the
	// lockfile step (which opens a flock at
	// filepath.Dir(settingsPath)/.claudio.lock) does not fail with
	// "path not found". The install workflow's WriteSettingsFile
	// would normally create this directory at write time, but the
	// lock happens BEFORE the write to serialise concurrent installs.
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0755); err != nil {
		t.Fatalf("failed to pre-create .claude dir: %v", err)
	}

	// Opt in to the recognizer's go-test extension so the verify step
	// accepts the go test binary. Production matches only "claudio"
	// and "claudio.exe"; under `go test` the executable basename is
	// e.g. "cli.test.exe". t.Setenv restores the prior value at test
	// end. This replaces the prior install.SwapExecutableRecognizer
	// helper, which forced the testing package into the production
	// binary.
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

	cli := NewCLI()
	stdin := strings.NewReader("")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	exitCode := cli.Run(
		[]string{"claudio", "install", "--quiet"},
		stdin, stdout, stderr,
	)
	if exitCode != 0 {
		t.Fatalf("install workflow exit code = %d; stderr=%q stdout=%q",
			exitCode, stderr.String(), stdout.String())
	}

	// settings.json must exist at the sandbox's ~/.claude/settings.json.
	settingsPath := filepath.Join(root, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		// On Windows BestConfigPath prefers USERPROFILE; check both.
		alt := filepath.Join(os.Getenv("USERPROFILE"), ".claude", "settings.json")
		if alt != settingsPath {
			if data2, err2 := os.ReadFile(alt); err2 == nil {
				data = data2
				settingsPath = alt
				err = nil
			}
		}
	}
	if err != nil {
		t.Fatalf("expected settings.json under sandbox, got error: %v", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v\ndata=%s", err, string(data))
	}

	hooksAny, ok := settings["hooks"]
	if !ok {
		t.Fatalf("settings.json has no 'hooks' key: %s", string(data))
	}
	hooks, ok := hooksAny.(map[string]interface{})
	if !ok {
		t.Fatalf("settings.json 'hooks' is %T, want map[string]interface{}", hooksAny)
	}

	// Every default-enabled Claude hook must be present AND must be
	// classified as a Claudio hook.
	for _, definition := range install.AgentClaude.EnabledHooks() {
		name := definition.Name
		val, exists := hooks[name]
		if !exists {
			t.Errorf("hook %q missing from settings.json after install", name)
			continue
		}
		if !install.IsClaudioHook(val) {
			t.Errorf("hook %q present but IsClaudioHook returned false: %+v", name, val)
		}
	}

	// Idempotency: a second install should leave the file in the
	// same shape.
	exitCode2 := cli.Run(
		[]string{"claudio", "install", "--quiet"},
		strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{},
	)
	if exitCode2 != 0 {
		t.Fatalf("second install exit code = %d (expected idempotent success)", exitCode2)
	}

	data2, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json disappeared after second install: %v", err)
	}
	var settings2 map[string]interface{}
	if err := json.Unmarshal(data2, &settings2); err != nil {
		t.Fatalf("settings.json after second install not valid JSON: %v", err)
	}
	hooks2, _ := settings2["hooks"].(map[string]interface{})
	for _, definition := range install.AgentClaude.EnabledHooks() {
		name := definition.Name
		if val, exists := hooks2[name]; !exists {
			t.Errorf("after second install hook %q missing", name)
		} else if !install.IsClaudioHook(val) {
			t.Errorf("after second install hook %q no longer claudio: %+v", name, val)
		}
	}
}
