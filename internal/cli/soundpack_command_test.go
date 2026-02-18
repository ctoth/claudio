package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"claudio.click/internal/config"
	"claudio.click/internal/soundpack"
	"github.com/adrg/xdg"
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

// createDummyWAV creates a minimal non-empty file with .wav extension for testing
func createDummyWAV(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	// Write minimal data - just needs to be non-empty with .wav extension
	if err := os.WriteFile(path, []byte("RIFF\x00\x00\x00\x00WAVEfmt "), 0644); err != nil {
		t.Fatalf("failed to create dummy WAV %s: %v", path, err)
	}
}

func TestSoundpackValidate_ValidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a few dummy WAV files
	wav1 := filepath.Join(tmpDir, "sounds", "click.wav")
	wav2 := filepath.Join(tmpDir, "sounds", "beep.wav")
	createDummyWAV(t, wav1)
	createDummyWAV(t, wav2)

	// Create a valid JSON soundpack with some mappings pointing to real files
	spFile := soundpack.JSONSoundpackFile{
		Name:        "test-valid",
		Description: "Test soundpack",
		Version:     "1.0.0",
		Mappings:    make(map[string]string),
	}

	// Get all known keys to populate mappings
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatalf("ExtractAllSoundKeys() failed: %v", err)
	}
	for _, key := range keys {
		spFile.Mappings[key] = ""
	}
	// Set a few to real files
	spFile.Mappings["loading/bash-start.wav"] = wav1
	spFile.Mappings["success/bash-success.wav"] = wav2

	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, "test-valid.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", jsonPath}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s, stdout: %s", exitCode, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Coverage Summary") {
		t.Error("expected output to contain 'Coverage Summary'")
	}
	if !strings.Contains(output, "test-valid") {
		t.Error("expected output to contain soundpack name 'test-valid'")
	}
	// 2 out of 107 mapped
	if !strings.Contains(output, "2/107") {
		t.Errorf("expected output to contain '2/107', got: %s", output)
	}
}

func TestSoundpackValidate_MissingFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JSON with mappings pointing to non-existent files
	spFile := soundpack.JSONSoundpackFile{
		Name:        "broken-pack",
		Description: "Pack with broken references",
		Version:     "1.0.0",
		Mappings: map[string]string{
			"loading/bash-start.wav":   filepath.Join(tmpDir, "nonexistent", "missing.wav"),
			"success/bash-success.wav": filepath.Join(tmpDir, "also", "missing.wav"),
			"default.wav":              "",
		},
	}

	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, "broken.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", jsonPath}, nil, stdout, stderr)

	if exitCode == 0 {
		t.Error("expected non-zero exit code when soundpack has broken references")
	}

	output := stdout.String()
	if !strings.Contains(output, "Broken References") && !strings.Contains(output, "not found") {
		t.Errorf("expected output to mention broken references, got: %s", output)
	}
}

func TestSoundpackValidate_EmptyMappings(t *testing.T) {
	tmpDir := t.TempDir()

	// Create JSON with all empty mapping values (like init output)
	spFile := soundpack.JSONSoundpackFile{
		Name:        "empty-pack",
		Description: "Empty soundpack template",
		Version:     "1.0.0",
		Mappings:    make(map[string]string),
	}

	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatalf("ExtractAllSoundKeys() failed: %v", err)
	}
	for _, key := range keys {
		spFile.Mappings[key] = ""
	}

	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, "empty.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", jsonPath}, nil, stdout, stderr)

	// Empty mappings are not errors — exit code should be 0
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for empty mappings, got %d, stderr: %s, stdout: %s", exitCode, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "0/107") && !strings.Contains(output, "0.0%") {
		t.Errorf("expected output to contain '0/107' or '0.0%%', got: %s", output)
	}
}

func TestSoundpackValidate_CoverageCalculation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create exactly 10 dummy WAV files
	wavFiles := make([]string, 10)
	for i := 0; i < 10; i++ {
		wavFiles[i] = filepath.Join(tmpDir, "sounds", fmt.Sprintf("sound%d.wav", i))
		createDummyWAV(t, wavFiles[i])
	}

	// Get all keys and fill exactly 10
	keys, err := ExtractAllSoundKeys()
	if err != nil {
		t.Fatalf("ExtractAllSoundKeys() failed: %v", err)
	}

	spFile := soundpack.JSONSoundpackFile{
		Name:        "coverage-test",
		Description: "Coverage test pack",
		Version:     "1.0.0",
		Mappings:    make(map[string]string),
	}
	for _, key := range keys {
		spFile.Mappings[key] = ""
	}

	// Fill first 10 keys with real files
	for i := 0; i < 10 && i < len(keys); i++ {
		spFile.Mappings[keys[i]] = wavFiles[i]
	}

	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, "coverage.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", jsonPath}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s, stdout: %s", exitCode, stderr.String(), stdout.String())
	}

	output := stdout.String()

	// Should show 10/107
	if !strings.Contains(output, "10/107") {
		t.Errorf("expected output to contain '10/107', got: %s", output)
	}

	// Per-category breakdown should appear
	if !strings.Contains(output, "loading:") {
		t.Errorf("expected output to contain 'loading:' category, got: %s", output)
	}
	if !strings.Contains(output, "success:") {
		t.Errorf("expected output to contain 'success:' category, got: %s", output)
	}
}

func TestSoundpackValidate_DirectorySoundpack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create directory structure with some audio files
	createDummyWAV(t, filepath.Join(tmpDir, "mypack", "loading", "loading.wav"))
	createDummyWAV(t, filepath.Join(tmpDir, "mypack", "success", "success.wav"))

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", filepath.Join(tmpDir, "mypack")}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr: %s, stdout: %s", exitCode, stderr.String(), stdout.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "Coverage Summary") {
		t.Errorf("expected output to contain 'Coverage Summary', got: %s", output)
	}
	// Should detect some coverage in loading and success categories
	if !strings.Contains(output, "loading:") {
		t.Errorf("expected output to contain 'loading:' category, got: %s", output)
	}
	if !strings.Contains(output, "success:") {
		t.Errorf("expected output to contain 'success:' category, got: %s", output)
	}
}

func TestSoundpackValidate_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with malformed JSON
	badPath := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(badPath, []byte(`{this is not valid JSON`), 0644); err != nil {
		t.Fatalf("failed to write bad JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", badPath}, nil, stdout, stderr)

	if exitCode == 0 {
		t.Error("expected non-zero exit code for invalid JSON")
	}

	combined := stdout.String() + stderr.String()
	if !strings.Contains(strings.ToLower(combined), "json") && !strings.Contains(strings.ToLower(combined), "parse") {
		t.Errorf("expected error to mention JSON parse error, got stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestSoundpackValidate_FormatCheck(t *testing.T) {
	tmpDir := t.TempDir()

	// Create files with non-audio extensions
	txtFile := filepath.Join(tmpDir, "sounds", "oops.txt")
	if err := os.MkdirAll(filepath.Dir(txtFile), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(txtFile, []byte("not audio"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	spFile := soundpack.JSONSoundpackFile{
		Name:        "format-test",
		Description: "Format check test",
		Version:     "1.0.0",
		Mappings: map[string]string{
			"loading/bash-start.wav": txtFile,
			"default.wav":            "",
		},
	}

	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}

	jsonPath := filepath.Join(tmpDir, "format.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "validate", jsonPath}, nil, stdout, stderr)

	// Format issues should be warnings, not necessarily failures
	// but output should mention the format issue
	output := stdout.String()
	_ = exitCode // format warnings may or may not affect exit code
	if !strings.Contains(strings.ToLower(output), "format") {
		t.Errorf("expected output to mention format issues for .txt file, got: %s", output)
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

// --- Soundpack Install Tests ---

// setupInstallTestEnv sets XDG_DATA_HOME and XDG_CONFIG_HOME to temp dirs,
// calls xdg.Reload() so the library picks up the new values, and returns
// a cleanup function that restores the original env vars and reloads.
func setupInstallTestEnv(t *testing.T) (dataDir, configDir string, cleanup func()) {
	t.Helper()
	dataDir = filepath.Join(t.TempDir(), "data")
	configDir = filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("failed to create data dir: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	oldDataHome := os.Getenv("XDG_DATA_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_DATA_HOME", dataDir)
	os.Setenv("XDG_CONFIG_HOME", configDir)
	xdg.Reload()

	cleanup = func() {
		os.Setenv("XDG_DATA_HOME", oldDataHome)
		os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
		xdg.Reload()
	}
	return dataDir, configDir, cleanup
}

// createTestJSONSoundpack creates a minimal valid JSON soundpack file in the given directory.
func createTestJSONSoundpack(t *testing.T, dir, name string) string {
	t.Helper()
	spFile := soundpack.JSONSoundpackFile{
		Name:        name,
		Description: "Test soundpack for install",
		Version:     "1.0.0",
		Mappings:    map[string]string{},
	}
	jsonData, err := json.MarshalIndent(spFile, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	jsonPath := filepath.Join(dir, name+".json")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		t.Fatalf("failed to write JSON: %v", err)
	}
	return jsonPath
}

func TestSoundpackInstall_CopiesJSONToDataDir(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	// Create a valid JSON soundpack in a separate temp dir
	srcDir := t.TempDir()
	jsonPath := createTestJSONSoundpack(t, srcDir, "my-test-pack")

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "install", jsonPath, "--skip-validate"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	// Assert file exists at expected XDG data path: <dataDir>/claudio/<name>.json
	expectedPath := filepath.Join(dataDir, "claudio", "my-test-pack.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected installed file at %s, got error: %v", expectedPath, err)
	}

	// Assert stdout contains "Installed"
	if !strings.Contains(stdout.String(), "Installed") {
		t.Errorf("expected stdout to contain 'Installed', got: %s", stdout.String())
	}
}

func TestSoundpackInstall_CopiesDirectoryToDataDir(t *testing.T) {
	dataDir, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	// Create a directory soundpack with a dummy loading/loading.wav file
	srcDir := filepath.Join(t.TempDir(), "test-dir-pack")
	createDummyWAV(t, filepath.Join(srcDir, "loading", "loading.wav"))

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "install", srcDir, "--skip-validate"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	// Assert directory + file exist at expected XDG data path
	expectedDir := filepath.Join(dataDir, "claudio", "soundpacks", "test-dir-pack")
	expectedFile := filepath.Join(expectedDir, "loading", "loading.wav")
	if _, err := os.Stat(expectedDir); err != nil {
		t.Errorf("expected installed directory at %s, got error: %v", expectedDir, err)
	}
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected installed file at %s, got error: %v", expectedFile, err)
	}
}

func TestSoundpackInstall_UpdatesConfig(t *testing.T) {
	_, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	// Create a valid JSON soundpack
	srcDir := t.TempDir()
	jsonPath := createTestJSONSoundpack(t, srcDir, "config-test-pack")

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "install", jsonPath, "--skip-validate"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	// Load config from the temp config dir and verify soundpack_paths
	configPath := filepath.Join(configDir, "claudio", "config.json")
	cm := config.NewConfigManager()
	cfg, err := cm.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", configPath, err)
	}

	found := false
	for _, p := range cfg.SoundpackPaths {
		if strings.Contains(p, "config-test-pack") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected soundpack_paths to contain installed path, got: %v", cfg.SoundpackPaths)
	}
}

func TestSoundpackInstall_DefaultFlag(t *testing.T) {
	_, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	// Create a valid JSON soundpack
	srcDir := t.TempDir()
	jsonPath := createTestJSONSoundpack(t, srcDir, "default-flag-pack")

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "install", jsonPath, "--skip-validate", "--default"}, nil, stdout, stderr)

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d, stdout: %s, stderr: %s", exitCode, stdout.String(), stderr.String())
	}

	// Load config and verify default_soundpack
	configPath := filepath.Join(configDir, "claudio", "config.json")
	cm := config.NewConfigManager()
	cfg, err := cm.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", configPath, err)
	}

	if cfg.DefaultSoundpack != "default-flag-pack" {
		t.Errorf("expected default_soundpack == 'default-flag-pack', got %q", cfg.DefaultSoundpack)
	}
}

func TestSoundpackInstall_IdempotentPathAddition(t *testing.T) {
	_, configDir, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	// Create a valid JSON soundpack
	srcDir := t.TempDir()
	jsonPath := createTestJSONSoundpack(t, srcDir, "idempotent-pack")

	// Install twice
	for i := 0; i < 2; i++ {
		cli := NewCLI()
		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}
		exitCode := cli.Run([]string{"claudio", "soundpack", "install", jsonPath, "--skip-validate"}, nil, stdout, stderr)

		if exitCode != 0 {
			t.Fatalf("install #%d: expected exit code 0, got %d, stdout: %s, stderr: %s", i+1, exitCode, stdout.String(), stderr.String())
		}
	}

	// Load config and verify path appears exactly once
	configPath := filepath.Join(configDir, "claudio", "config.json")
	cm := config.NewConfigManager()
	cfg, err := cm.LoadFromFile(configPath)
	if err != nil {
		t.Fatalf("failed to load config from %s: %v", configPath, err)
	}

	count := 0
	for _, p := range cfg.SoundpackPaths {
		if strings.Contains(p, "idempotent-pack") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected soundpack_paths to contain path exactly once, found %d times in: %v", count, cfg.SoundpackPaths)
	}
}

func TestSoundpackInstall_FailsOnInvalidPath(t *testing.T) {
	_, _, cleanup := setupInstallTestEnv(t)
	defer cleanup()

	cli := NewCLI()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	exitCode := cli.Run([]string{"claudio", "soundpack", "install", "/nonexistent/path/that/does/not/exist"}, nil, stdout, stderr)

	if exitCode == 0 {
		t.Error("expected non-zero exit code for non-existent path")
	}
}
