package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/cli"
)

// TestExecutableDirectoryDetection tests that we can detect the executable directory
func TestExecutableDirectoryDetection(t *testing.T) {
	// Test the core requirement: can we accurately detect the executable's directory?
	
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("Failed to get executable path: %v", err)
	}
	
	execDir := filepath.Dir(executable)
	t.Logf("Current executable: %s", executable)
	t.Logf("Current executable directory: %s", execDir)
	
	// Verify the executable directory exists
	if _, err := os.Stat(execDir); os.IsNotExist(err) {
		t.Error("Executable directory does not exist")
	}
}

// TestPlatformSoundpackBasic tests basic platform soundpack functionality
func TestPlatformSoundpackBasic(t *testing.T) {
	t.Run("CLI handles missing platform soundpack gracefully", func(t *testing.T) {
		// Create test hook input
		testInput := `{"session_id":"basic-test","transcript_path":"/test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"Bash","tool_response":{"stdout":"success","stderr":"","interrupted":false}}`
		
		// Test CLI with no config - should handle gracefully
		cli := cli.NewCLI()
		stdin := strings.NewReader(testInput)
		var stdout, stderr strings.Builder
		
		// Run CLI - should not crash even if no platform soundpack exists
		exitCode := cli.Run([]string{"claudio", "--silent"}, stdin, &stdout, &stderr)
		
		if exitCode != 0 {
			t.Errorf("CLI should handle missing soundpack gracefully, got exit code %d", exitCode)
			t.Logf("Stderr: %s", stderr.String())
		}
		
		t.Logf("CLI handled missing soundpack - Exit code: %d", exitCode)
	})
}

// truncateString helper for log output
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}