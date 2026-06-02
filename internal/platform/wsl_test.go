package platform

import (
	"testing"
)

func TestIsWSL(t *testing.T) {
	tests := []struct {
		name           string
		procVersion    string
		wslEnv         string
		expectedResult bool
	}{
		{
			name:           "WSL1 detected via /proc/version",
			procVersion:    "Linux version 4.4.0-19041-Microsoft (Microsoft@Microsoft.com) (gcc version 5.4.0 (Ubuntu 5.4.0-6ubuntu1~16.04.12) ) #1237-Microsoft Sat Sep 11 14:32:00 PST 2021",
			wslEnv:         "",
			expectedResult: true,
		},
		{
			name:           "WSL2 detected via /proc/version",
			procVersion:    "Linux version 5.15.74.2-microsoft-standard-WSL2 (gcc (GCC) 11.2.0) #1 SMP Wed Oct 5 20:57:03 UTC 2022",
			wslEnv:         "",
			expectedResult: true,
		},
		{
			name:           "WSL detected via WSL_DISTRO_NAME env var",
			procVersion:    "",
			wslEnv:         "Ubuntu",
			expectedResult: true,
		},
		{
			name:           "Native Linux - no WSL indicators",
			procVersion:    "Linux version 5.15.0-56-generic (buildd@lcy02-amd64-044) (gcc (Ubuntu 11.3.0-1ubuntu1~22.04) #62-Ubuntu SMP Tue Nov 22 19:54:14 UTC 2022",
			wslEnv:         "",
			expectedResult: false,
		},
		{
			name:           "Empty proc version and no env var",
			procVersion:    "",
			wslEnv:         "",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectWSLFromData(tt.procVersion, tt.wslEnv)
			if result != tt.expectedResult {
				t.Errorf("expected %v, got %v", tt.expectedResult, result)
			}
		})
	}
}

// TestHelperFunctions verifies the detectWSLFromData helper directly.
func TestHelperFunctions(t *testing.T) {
	t.Run("detectWSLFromData should be implemented", func(t *testing.T) {
		result := detectWSLFromData("Linux version 5.15.74.2-microsoft-standard-WSL2", "")
		if !result {
			t.Error("should detect WSL2 from proc version")
		}

		result = detectWSLFromData("", "Ubuntu")
		if !result {
			t.Error("should detect WSL from environment variable")
		}

		result = detectWSLFromData("regular linux", "")
		if result {
			t.Error("should not detect WSL from regular linux")
		}
	})
}

// TestIsWSLDoesNotPanic exercises the real IsWSL() entry point. The
// returned value depends on the runtime environment; we only assert the
// call returns without panicking and produces a boolean.
func TestIsWSLDoesNotPanic(t *testing.T) {
	result := IsWSL()
	t.Logf("Real system WSL detection: %v", result)
	_ = result
}
