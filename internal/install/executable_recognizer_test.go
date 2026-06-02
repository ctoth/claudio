package install

import "testing"

// TestExecutableRecognizer_ProductionDefaults verifies that without the
// CLAUDIO_TEST_RECOGNIZE_GO_TEST env var the recognizer accepts only the
// production claudio basenames.
func TestExecutableRecognizer_ProductionDefaults(t *testing.T) {
	// Make sure the env var is not set for this case.
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "")

	cases := []struct {
		name string
		want bool
	}{
		{"claudio", true},
		{"claudio.exe", true},
		{"cli.test.exe", false},
		{"cli.test", false},
		{"some-other-binary", false},
		{"", false},
	}
	for _, c := range cases {
		if got := executableRecognizer(c.name); got != c.want {
			t.Errorf("executableRecognizer(%q) = %v; want %v", c.name, got, c.want)
		}
	}
}

// TestExecutableRecognizer_GoTestSuffixWhenEnvVarSet verifies that when
// the test opt-in env var is set, .test and .test.exe basenames are also
// recognized as the claudio executable so end-to-end install tests can
// thread the go test binary through the install workflow.
func TestExecutableRecognizer_GoTestSuffixWhenEnvVarSet(t *testing.T) {
	t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", "1")

	cases := []struct {
		name string
		want bool
	}{
		{"claudio", true},
		{"claudio.exe", true},
		{"cli.test.exe", true},
		{"cli.test", true},
		{"install.test", true},
		{"unrelated.exe", false},
		{"random", false},
	}
	for _, c := range cases {
		if got := executableRecognizer(c.name); got != c.want {
			t.Errorf("executableRecognizer(%q) with env opt-in = %v; want %v", c.name, got, c.want)
		}
	}
}

// TestExecutableRecognizer_EnvVarOtherValuesDoNotEnable verifies that
// only the literal "1" enables the test opt-in — any other non-empty
// value is ignored, matching the documented contract.
func TestExecutableRecognizer_EnvVarOtherValuesDoNotEnable(t *testing.T) {
	for _, val := range []string{"true", "yes", "0", "TRUE", "on"} {
		t.Setenv("CLAUDIO_TEST_RECOGNIZE_GO_TEST", val)
		if executableRecognizer("cli.test.exe") {
			t.Errorf("env value %q should NOT enable go-test recognition", val)
		}
	}
}
