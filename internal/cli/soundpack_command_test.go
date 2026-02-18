package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"claudio.click/internal/soundpack"
)

func TestSoundpackInit_CreatesValidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init", "test-pack", "--dir", tmpDir}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	outputPath := filepath.Join(tmpDir, "test-pack.json")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected file at %s, got error: %v", outputPath, err)
	}

	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if spFile.Name != "test-pack" {
		t.Errorf("expected Name == 'test-pack', got %q", spFile.Name)
	}
}

func TestSoundpackInit_ContainsAllCategories(t *testing.T) {
	tmpDir := t.TempDir()

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init", "cat-test", "--dir", tmpDir}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "cat-test.json"))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Check that mappings contain keys from each expected category prefix
	requiredPrefixes := []string{"loading/", "success/", "error/", "interactive/", "completion/", "system/"}
	for _, prefix := range requiredPrefixes {
		found := false
		for key := range spFile.Mappings {
			if strings.HasPrefix(key, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected mappings to contain at least one key with prefix %q", prefix)
		}
	}

	// Check default.wav is present
	if _, ok := spFile.Mappings["default.wav"]; !ok {
		t.Error("expected mappings to contain 'default.wav'")
	}
}

func TestSoundpackInit_MappingValuesAreEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init", "empty-test", "--dir", tmpDir}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "empty-test.json"))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	for key, val := range spFile.Mappings {
		if val != "" {
			t.Errorf("expected mapping value for %q to be empty, got %q", key, val)
		}
	}
}

func TestSoundpackInit_FromPlatformPreFillsValues(t *testing.T) {
	tmpDir := t.TempDir()

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init", "prefill-test", "--dir", tmpDir, "--from-platform"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "prefill-test.json"))
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var spFile soundpack.JSONSoundpackFile
	if err := json.Unmarshal(data, &spFile); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	nonEmpty := 0
	for _, val := range spFile.Mappings {
		if val != "" {
			nonEmpty++
		}
	}

	if nonEmpty == 0 {
		t.Error("expected at least some mapping values to be non-empty with --from-platform")
	}
}

func TestSoundpackInit_FailsWithoutName(t *testing.T) {
	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init"}, nil, stdout, stderr)

	if exitCode == 0 {
		t.Error("expected non-zero exit code when name argument is missing")
	}
}

func TestSoundpackInit_OverwriteProtection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an existing file
	existingPath := filepath.Join(tmpDir, "existing.json")
	if err := os.WriteFile(existingPath, []byte(`{"existing": true}`), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "init", "existing", "--dir", tmpDir}, nil, stdout, stderr)

	if exitCode == 0 {
		t.Error("expected non-zero exit code when target file already exists")
	}

	combined := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(combined), "exists") {
		t.Errorf("expected error message to mention file exists, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestSoundpackList_ShowsEmbeddedPacks(t *testing.T) {
	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "list"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()

	// All three embedded platform packs must appear
	if !strings.Contains(output, "windows") {
		t.Error("expected output to contain 'windows'")
	}
	if !strings.Contains(output, "wsl") {
		t.Error("expected output to contain 'wsl'")
	}
	if !strings.Contains(output, "darwin") {
		t.Error("expected output to contain 'darwin'")
	}

	// All should be marked as embedded type
	if !strings.Contains(output, "embedded") {
		t.Error("expected output to contain 'embedded' type marker")
	}
}

func TestSoundpackList_OutputFormat(t *testing.T) {
	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "list"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()

	// Check column headers
	if !strings.Contains(output, "NAME") {
		t.Error("expected output to contain 'NAME' header")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("expected output to contain 'TYPE' header")
	}
	if !strings.Contains(output, "SOUNDS") {
		t.Error("expected output to contain 'SOUNDS' header")
	}
}

func TestSoundpackList_ShowsEmbeddedSoundCounts(t *testing.T) {
	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "list"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	lines := strings.Split(output, "\n")

	// Find lines containing embedded packs and verify they have non-zero sound counts
	// The output format is: NAME TYPE SOUNDS PATH
	// We look for lines with "embedded" and verify they don't show "0" sounds
	foundEmbedded := false
	for _, line := range lines {
		if strings.Contains(line, "embedded") {
			foundEmbedded = true
			// The line should not contain "0" as the sound count
			// We split by whitespace and check the SOUNDS column
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				soundCount := fields[2]
				if soundCount == "0" {
					t.Errorf("expected non-zero sound count for embedded pack, got line: %s", line)
				}
			}
		}
	}

	if !foundEmbedded {
		t.Error("expected at least one embedded pack in output")
	}
}

func TestSoundpackList_ExitsCleanly(t *testing.T) {
	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "list"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s", exitCode, stderr.String())
	}

	// stdout should have content (at least the headers + 3 embedded packs)
	if stdout.Len() == 0 {
		t.Error("expected non-empty stdout")
	}
}

func TestExtractAllSoundKeys(t *testing.T) {
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatalf("ExtractAllSoundKeys() returned error: %v", err)
	}

	// Scout report found 116 unique keys
	if len(keys) < 100 {
		t.Errorf("expected > 100 keys, got %d", len(keys))
	}

	// Assert keys are sorted
	if !sort.StringsAreSorted(keys) {
		t.Error("expected keys to be sorted")
	}

	// Assert contains default.wav
	found := false
	for _, k := range keys {
		if k == "default.wav" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected keys to contain 'default.wav'")
	}

	// Assert contains at least one from each category
	categories := []string{"loading/", "success/", "error/", "interactive/", "completion/", "system/"}
	for _, prefix := range categories {
		catFound := false
		for _, k := range keys {
			if strings.HasPrefix(k, prefix) {
				catFound = true
				break
			}
		}
		if !catFound {
			t.Errorf("expected keys to contain at least one with prefix %q", prefix)
		}
	}
}
