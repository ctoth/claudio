package soundpack

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnifiedSoundpackResolver(t *testing.T) {
	// TDD Test: Verify unified soundpack resolver with different mappers

	t.Run("directory mapper integration", func(t *testing.T) {
		// Create temporary soundpack directory
		tempDir := t.TempDir()
		soundpackDir := filepath.Join(tempDir, "success")
		err := os.MkdirAll(soundpackDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack dir: %v", err)
		}

		// Create test sound file
		soundFile := filepath.Join(soundpackDir, "bash.wav")
		err = os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Create directory mapper and resolver
		mapper := NewDirectoryMapper("test", []string{tempDir})
		resolver := NewSoundpackResolver(mapper)

		// Verify interface compliance
		var _ SoundpackResolver = resolver

		// Test basic interface methods
		if resolver.GetName() == "" {
			t.Error("GetName() should return non-empty name")
		}

		if resolver.GetType() == "" {
			t.Error("GetType() should return non-empty type")
		}

		// Test sound resolution
		path, err := resolver.ResolveSound("success/bash.wav")
		if err != nil {
			t.Errorf("Expected to resolve sound, got error: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "success", "bash.wav")
		if path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, path)
		}
	})

	t.Run("json mapper integration", func(t *testing.T) {
		// Create temporary sound file
		tempDir := t.TempDir()
		soundFile := filepath.Join(tempDir, "my-sound.wav")
		err := os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Create JSON mapping
		mapping := map[string]string{
			"success/bash.wav": soundFile,
			"default.wav":      soundFile,
		}

		// Create JSON mapper and resolver
		mapper := NewJSONMapper("test-json", mapping)
		resolver := NewSoundpackResolver(mapper)

		// Test sound resolution
		path, err := resolver.ResolveSound("success/bash.wav")
		if err != nil {
			t.Errorf("Expected to resolve sound, got error: %v", err)
		}

		if path != soundFile {
			t.Errorf("Expected path %s, got %s", soundFile, path)
		}

		// Test type identification
		if resolver.GetType() != "json" {
			t.Errorf("Expected type 'json', got '%s'", resolver.GetType())
		}
	})

	t.Run("supports fallback resolution", func(t *testing.T) {
		// Create temporary soundpack directory
		tempDir := t.TempDir()
		soundpackDir := filepath.Join(tempDir, "success")
		err := os.MkdirAll(soundpackDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack dir: %v", err)
		}

		// Create only the fallback sound file (not the specific one)
		soundFile := filepath.Join(soundpackDir, "success.wav")
		err = os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		mapper := NewDirectoryMapper("test", []string{tempDir})
		resolver := NewSoundpackResolver(mapper)

		// Test fallback resolution
		fallbackPaths := []string{
			"success/bash-specific.wav", // Won't exist
			"success/bash.wav",          // Won't exist
			"success/success.wav",       // This exists
			"default.wav",               // Fallback
		}

		path, err := resolver.ResolveSoundWithFallback(fallbackPaths)
		if err != nil {
			t.Errorf("Expected to resolve fallback sound, got error: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "success", "success.wav")
		if path != expectedPath {
			t.Errorf("Expected fallback path %s, got %s", expectedPath, path)
		}
	})

	t.Run("handles missing sounds gracefully", func(t *testing.T) {
		tempDir := t.TempDir()

		mapper := NewDirectoryMapper("test", []string{tempDir})
		resolver := NewSoundpackResolver(mapper)

		// Test missing sound
		_, err := resolver.ResolveSound("nonexistent/sound.wav")
		if err == nil {
			t.Error("Expected error for nonexistent sound")
		}

		// Should be a FileNotFoundError
		if !IsFileNotFoundError(err) {
			t.Errorf("Expected FileNotFoundError, got: %v", err)
		}

		// Test fallback with all missing
		fallbackPaths := []string{
			"missing1.wav",
			"missing2.wav",
		}

		_, err = resolver.ResolveSoundWithFallback(fallbackPaths)
		if err == nil {
			t.Error("Expected error when all fallback paths missing")
		}
	})
}

func TestDirectoryMapper(t *testing.T) {
	// TDD Test: Verify DirectoryMapper specific functionality

	t.Run("maps paths correctly", func(t *testing.T) {
		mapper := NewDirectoryMapper("test", []string{"/path1", "/path2"})

		candidates, err := mapper.MapPath("success/bash.wav")
		if err != nil {
			t.Errorf("MapPath should not error: %v", err)
		}

		expected := []string{
			"/path1/success/bash.wav",
			"/path2/success/bash.wav",
		}

		if len(candidates) != len(expected) {
			t.Errorf("Expected %d candidates, got %d", len(expected), len(candidates))
		}

		for i, exp := range expected {
			if candidates[i] != exp {
				t.Errorf("Candidate[%d] = %s, expected %s", i, candidates[i], exp)
			}
		}
	})

	t.Run("returns correct metadata", func(t *testing.T) {
		mapper := NewDirectoryMapper("my-soundpack", []string{"/test"})

		if mapper.GetName() != "my-soundpack" {
			t.Errorf("Expected name 'my-soundpack', got '%s'", mapper.GetName())
		}

		if mapper.GetType() != "directory" {
			t.Errorf("Expected type 'directory', got '%s'", mapper.GetType())
		}
	})
}

func TestJSONMapper(t *testing.T) {
	// TDD Test: Verify JSONMapper specific functionality

	t.Run("maps paths correctly", func(t *testing.T) {
		mapping := map[string]string{
			"success/bash.wav": "/absolute/path/to/sound1.wav",
			"error/error.wav":  "/absolute/path/to/sound2.wav",
		}

		mapper := NewJSONMapper("test-json", mapping)

		// Test existing mapping
		candidates, err := mapper.MapPath("success/bash.wav")
		if err != nil {
			t.Errorf("MapPath should not error: %v", err)
		}

		expected := []string{"/absolute/path/to/sound1.wav"}
		if len(candidates) != 1 || candidates[0] != expected[0] {
			t.Errorf("Expected %v, got %v", expected, candidates)
		}
	})

	t.Run("handles missing keys", func(t *testing.T) {
		mapping := map[string]string{
			"success/bash.wav": "/absolute/path/to/sound1.wav",
		}

		mapper := NewJSONMapper("test-json", mapping)

		// Test missing key
		candidates, err := mapper.MapPath("missing/sound.wav")
		if err != nil {
			t.Errorf("MapPath should not error for missing key: %v", err)
		}

		if len(candidates) != 0 {
			t.Errorf("Expected empty candidates for missing key, got %v", candidates)
		}
	})

	t.Run("returns correct metadata", func(t *testing.T) {
		mapper := NewJSONMapper("my-json-pack", map[string]string{})

		if mapper.GetName() != "my-json-pack" {
			t.Errorf("Expected name 'my-json-pack', got '%s'", mapper.GetName())
		}

		if mapper.GetType() != "json" {
			t.Errorf("Expected type 'json', got '%s'", mapper.GetType())
		}
	})
}

func TestJSONSoundpackLoading(t *testing.T) {
	// TDD Test: Load JSON soundpack files from disk with validation

	t.Run("loads valid JSON soundpack", func(t *testing.T) {
		// Create temporary JSON soundpack file
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "test-soundpack.json")

		// Create actual sound files that the JSON will reference
		soundFile1 := filepath.Join(tempDir, "success-sound.wav")
		soundFile2 := filepath.Join(tempDir, "error-sound.wav")
		err := os.WriteFile(soundFile1, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}
		err = os.WriteFile(soundFile2, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Create JSON soundpack content
		jsonContent := fmt.Sprintf(`{
			"name": "test-soundpack",
			"description": "Test soundpack for unit tests",
			"mappings": {
				"success/bash.wav": "%s",
				"error/error.wav": "%s",
				"default.wav": "%s"
			}
		}`, soundFile1, soundFile2, soundFile1)

		err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		// Load the JSON soundpack (function to be implemented)
		mapper, err := LoadJSONSoundpack(jsonFile)
		if err != nil {
			t.Errorf("Expected to load JSON soundpack, got error: %v", err)
		}

		// Verify mapper properties
		if mapper.GetName() != "test-soundpack" {
			t.Errorf("Expected name 'test-soundpack', got '%s'", mapper.GetName())
		}

		if mapper.GetType() != "json" {
			t.Errorf("Expected type 'json', got '%s'", mapper.GetType())
		}

		// Test sound resolution
		candidates, err := mapper.MapPath("success/bash.wav")
		if err != nil {
			t.Errorf("MapPath should not error: %v", err)
		}

		if len(candidates) != 1 || candidates[0] != soundFile1 {
			t.Errorf("Expected [%s], got %v", soundFile1, candidates)
		}
	})

	t.Run("validates sound file existence during load", func(t *testing.T) {
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "invalid-soundpack.json")

		// Create JSON with reference to non-existent sound file
		jsonContent := `{
			"name": "invalid-soundpack",
			"mappings": {
				"success/bash.wav": "/nonexistent/path/sound.wav"
			}
		}`

		err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		// Loading should fail due to missing sound file
		_, err = LoadJSONSoundpack(jsonFile)
		if err == nil {
			t.Error("Expected error when loading soundpack with missing sound files")
		}
	})

	t.Run("handles malformed JSON gracefully", func(t *testing.T) {
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "malformed.json")

		// Create malformed JSON
		jsonContent := `{
			"name": "test",
			"mappings": {
				"success/bash.wav": "/path/sound.wav"
			// Missing closing brace
		}`

		err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		// Loading should fail due to malformed JSON
		_, err = LoadJSONSoundpack(jsonFile)
		if err == nil {
			t.Error("Expected error when loading malformed JSON")
		}
	})

	t.Run("supports 5-level fallback system", func(t *testing.T) {
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "fallback-test.json")

		// Create sound files for different fallback levels
		bashSpecific := filepath.Join(tempDir, "bash-specific.wav")
		bashGeneral := filepath.Join(tempDir, "bash-general.wav")
		successCategory := filepath.Join(tempDir, "success-category.wav")
		defaultSound := filepath.Join(tempDir, "default.wav")

		for _, file := range []string{bashSpecific, bashGeneral, successCategory, defaultSound} {
			err := os.WriteFile(file, []byte("fake wav data"), 0644)
			if err != nil {
				t.Fatalf("Failed to create sound file %s: %v", file, err)
			}
		}

		// Create JSON with 5-level mappings
		jsonContent := fmt.Sprintf(`{
			"name": "fallback-test",
			"mappings": {
				"success/bash-specific.wav": "%s",
				"success/bash.wav": "%s",
				"success/tool-complete.wav": "%s",
				"success/success.wav": "%s",
				"default.wav": "%s"
			}
		}`, bashSpecific, bashGeneral, successCategory, successCategory, defaultSound)

		err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		mapper, err := LoadJSONSoundpack(jsonFile)
		if err != nil {
			t.Fatalf("Failed to load JSON soundpack: %v", err)
		}

		// Test that all fallback levels are mapped correctly
		testCases := []struct {
			path     string
			expected string
		}{
			{"success/bash-specific.wav", bashSpecific},
			{"success/bash.wav", bashGeneral},
			{"success/tool-complete.wav", successCategory},
			{"success/success.wav", successCategory},
			{"default.wav", defaultSound},
		}

		for _, tc := range testCases {
			candidates, err := mapper.MapPath(tc.path)
			if err != nil {
				t.Errorf("MapPath('%s') should not error: %v", tc.path, err)
			}

			if len(candidates) != 1 || candidates[0] != tc.expected {
				t.Errorf("MapPath('%s') = %v, expected [%s]", tc.path, candidates, tc.expected)
			}
		}
	})
}

func TestSoundpackFactory(t *testing.T) {
	// TDD Test: Auto-detect soundpack type and create appropriate mapper

	t.Run("creates directory mapper for directory soundpack", func(t *testing.T) {
		// Create temporary soundpack directory
		tempDir := t.TempDir()
		soundpackDir := filepath.Join(tempDir, "test-soundpack")
		err := os.MkdirAll(soundpackDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack dir: %v", err)
		}

		// Create test sound file to make it a valid directory soundpack
		successDir := filepath.Join(soundpackDir, "success")
		err = os.MkdirAll(successDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create success dir: %v", err)
		}

		soundFile := filepath.Join(successDir, "bash.wav")
		err = os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Factory should detect directory soundpack and create DirectoryMapper
		mapper, err := CreateSoundpackMapper("test-soundpack", soundpackDir)
		if err != nil {
			t.Errorf("Expected to create directory mapper, got error: %v", err)
		}

		if mapper.GetType() != "directory" {
			t.Errorf("Expected directory mapper, got '%s'", mapper.GetType())
		}

		if mapper.GetName() != "test-soundpack" {
			t.Errorf("Expected name 'test-soundpack', got '%s'", mapper.GetName())
		}
	})

	t.Run("creates JSON mapper for JSON file soundpack", func(t *testing.T) {
		// Create temporary JSON soundpack file
		tempDir := t.TempDir()
		jsonFile := filepath.Join(tempDir, "test-soundpack.json")

		// Create actual sound file that the JSON will reference
		soundFile := filepath.Join(tempDir, "test-sound.wav")
		err := os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Create JSON soundpack content
		jsonContent := fmt.Sprintf(`{
			"name": "json-test-soundpack",
			"mappings": {
				"success/bash.wav": "%s"
			}
		}`, soundFile)

		err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		// Factory should detect JSON soundpack and create JSONMapper
		mapper, err := CreateSoundpackMapper("test-soundpack", jsonFile)
		if err != nil {
			t.Errorf("Expected to create JSON mapper, got error: %v", err)
		}

		if mapper.GetType() != "json" {
			t.Errorf("Expected json mapper, got '%s'", mapper.GetType())
		}

		if mapper.GetName() != "json-test-soundpack" {
			t.Errorf("Expected name 'json-test-soundpack', got '%s'", mapper.GetName())
		}
	})

	t.Run("handles non-existent paths gracefully", func(t *testing.T) {
		// Factory should fail gracefully for non-existent paths
		_, err := CreateSoundpackMapper("test", "/nonexistent/path")
		if err == nil {
			t.Error("Expected error for non-existent path")
		}
	})

	t.Run("auto-detects based on file extension", func(t *testing.T) {
		// Create temporary files
		tempDir := t.TempDir()

		// Test .json extension
		jsonFile := filepath.Join(tempDir, "soundpack.json")
		soundFile := filepath.Join(tempDir, "sound.wav")
		err := os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create sound file: %v", err)
		}

		jsonContent := fmt.Sprintf(`{
			"name": "extension-test",
			"mappings": {
				"default.wav": "%s"
			}
		}`, soundFile)

		err = os.WriteFile(jsonFile, []byte(jsonContent), 0644)
		if err != nil {
			t.Fatalf("Failed to create JSON file: %v", err)
		}

		mapper, err := CreateSoundpackMapper("test", jsonFile)
		if err != nil {
			t.Errorf("Expected to create mapper, got error: %v", err)
		}

		if mapper.GetType() != "json" {
			t.Errorf("Expected json type for .json file, got '%s'", mapper.GetType())
		}
	})

	t.Run("fallback to directory base paths when exact path not found", func(t *testing.T) {
		// Test factory behavior when given soundpack name instead of exact path
		// This should create directory mapper with base paths

		// Create temporary soundpack directory structure
		tempDir := t.TempDir()
		soundpackDir := filepath.Join(tempDir, "fallback-test")
		err := os.MkdirAll(soundpackDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create soundpack dir: %v", err)
		}

		// Create test sound file
		soundFile := filepath.Join(soundpackDir, "default.wav")
		err = os.WriteFile(soundFile, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Factory should create directory mapper with base paths when exact path doesn't exist
		mapper, err := CreateSoundpackMapperWithBasePaths("fallback-test", "nonexistent-path", []string{tempDir})
		if err != nil {
			t.Errorf("Expected to create directory mapper with base paths, got error: %v", err)
		}

		if mapper.GetType() != "directory" {
			t.Errorf("Expected directory mapper for fallback, got '%s'", mapper.GetType())
		}

		// Test that it can resolve sounds from the base paths
		resolver := NewSoundpackResolver(mapper)
		resolved, err := resolver.ResolveSound("fallback-test/default.wav")
		if err != nil {
			t.Errorf("Expected to resolve sound through base paths: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "fallback-test", "default.wav")
		if resolved != expectedPath {
			t.Errorf("Expected resolved path %s, got %s", expectedPath, resolved)
		}
	})
}

func TestLoadJSONSoundpackFromBytes(t *testing.T) {
	// TDD Test: Load JSON soundpack from byte data with validation

	t.Run("loads valid JSON soundpack from bytes", func(t *testing.T) {
		// Create temporary sound files that the JSON will reference
		tempDir := t.TempDir()
		soundFile1 := filepath.Join(tempDir, "success-sound.wav")
		soundFile2 := filepath.Join(tempDir, "error-sound.wav")

		err := os.WriteFile(soundFile1, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}
		err = os.WriteFile(soundFile2, []byte("fake wav data"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test sound file: %v", err)
		}

		// Create JSON soundpack content as bytes
		// Use forward slashes to avoid Windows path escaping issues in JSON
		soundFile1JSON := strings.ReplaceAll(soundFile1, "\\", "/")
		soundFile2JSON := strings.ReplaceAll(soundFile2, "\\", "/")
		
		jsonContent := fmt.Sprintf(`{
			"name": "test-soundpack-from-bytes",
			"description": "Test soundpack for LoadJSONSoundpackFromBytes",
			"version": "1.0.0",
			"mappings": {
				"success/bash.wav": "%s",
				"error/bash.wav": "%s"
			}
		}`, soundFile1JSON, soundFile2JSON)

		// Load the JSON soundpack from bytes
		mapper, err := LoadJSONSoundpackFromBytes([]byte(jsonContent))
		if err != nil {
			t.Errorf("Expected to load JSON soundpack from bytes, got error: %v", err)
		}

		// Verify mapper properties
		if mapper == nil {
			t.Fatal("Expected mapper to be created, got nil")
		}

		if mapper.GetName() != "test-soundpack-from-bytes" {
			t.Errorf("Expected name 'test-soundpack-from-bytes', got '%s'", mapper.GetName())
		}

		// Test sound mapping
		candidates, err := mapper.MapPath("success/bash.wav")
		if err != nil {
			t.Errorf("Expected to map sound path, got error: %v", err)
		}

		// The JSON contains forward slashes, but we expect the path to work
		expectedPath := strings.ReplaceAll(soundFile1, "\\", "/")
		if len(candidates) != 1 || candidates[0] != expectedPath {
			t.Errorf("Expected candidates [%s], got %v", expectedPath, candidates)
		}
	})

	t.Run("rejects invalid JSON", func(t *testing.T) {
		invalidJSON := `{
			"name": "test-soundpack",
			"mappings": {
				"invalid": "json" // Missing closing quote
			}
		`

		_, err := LoadJSONSoundpackFromBytes([]byte(invalidJSON))
		if err == nil {
			t.Error("Expected error for invalid JSON, got nil")
		}

		if !strings.Contains(err.Error(), "failed to parse JSON soundpack from bytes") {
			t.Errorf("Expected JSON parse error message, got: %v", err)
		}
	})

	t.Run("rejects soundpack missing name field", func(t *testing.T) {
		jsonContent := `{
			"description": "Missing name field",
			"mappings": {
				"success/test.wav": "/tmp/test.wav"
			}
		}`

		_, err := LoadJSONSoundpackFromBytes([]byte(jsonContent))
		if err == nil {
			t.Error("Expected error for missing name field, got nil")
		}

		if !strings.Contains(err.Error(), "missing required 'name' field") {
			t.Errorf("Expected missing name error message, got: %v", err)
		}
	})

	t.Run("rejects soundpack with empty mappings", func(t *testing.T) {
		jsonContent := `{
			"name": "empty-mappings-test",
			"mappings": {}
		}`

		_, err := LoadJSONSoundpackFromBytes([]byte(jsonContent))
		if err == nil {
			t.Error("Expected error for empty mappings, got nil")
		}

		if !strings.Contains(err.Error(), "missing or empty 'mappings' field") {
			t.Errorf("Expected empty mappings error message, got: %v", err)
		}
	})

	t.Run("rejects soundpack with missing sound files", func(t *testing.T) {
		jsonContent := `{
			"name": "missing-files-test",
			"mappings": {
				"success/test.wav": "/nonexistent/file.wav"
			}
		}`

		_, err := LoadJSONSoundpackFromBytes([]byte(jsonContent))
		if err == nil {
			t.Error("Expected error for missing sound files, got nil")
		}

		if !strings.Contains(err.Error(), "sound file not found") {
			t.Errorf("Expected missing file error message, got: %v", err)
		}
	})

	t.Run("handles empty byte data", func(t *testing.T) {
		_, err := LoadJSONSoundpackFromBytes([]byte(""))
		if err == nil {
			t.Error("Expected error for empty byte data, got nil")
		}

		if !strings.Contains(err.Error(), "failed to parse JSON soundpack from bytes") {
			t.Errorf("Expected JSON parse error for empty data, got: %v", err)
		}
	})
}
