package install

import "testing"

func TestSwapExecutableRecognizer_RestoresAfterTest(t *testing.T) {
	prevName := "claudio"
	if !executableRecognizer(prevName) {
		t.Fatalf("precondition: production recognizer should accept %q", prevName)
	}

	called := false
	t.Run("inside-subtest-with-swap", func(t *testing.T) {
		SwapExecutableRecognizer(t, func(name string) bool {
			called = true
			return name == "cli.test.exe"
		})
		if !executableRecognizer("cli.test.exe") {
			t.Error("swapped recognizer should accept cli.test.exe")
		}
		if executableRecognizer("claudio") {
			t.Error("swapped recognizer should reject claudio")
		}
	})

	if !called {
		t.Error("swapped recognizer was never invoked inside the subtest")
	}

	// After the subtest's t.Cleanup runs, the original recognizer
	// must be restored.
	if !executableRecognizer("claudio") {
		t.Error("recognizer not restored to production behaviour after t.Cleanup")
	}
	if executableRecognizer("cli.test.exe") {
		t.Error("test-only recognizer leaked past t.Cleanup")
	}
}
