package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileResolver_ResolveWithExtensions(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := os.MkdirTemp("", "file_resolver_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test with priority ordering - .wav should be found before .mp3
	t.Run("finds wav before mp3 when both exist", func(t *testing.T) {
		basePath := filepath.Join(tempDir, "success", "bash-success")
		
		// Create directory structure
		successDir := filepath.Join(tempDir, "success")
		err := os.MkdirAll(successDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create success dir: %v", err)
		}

		// Create both .wav and .mp3 files
		wavFile := basePath + ".wav"
		mp3File := basePath + ".mp3"
		
		if err := os.WriteFile(wavFile, []byte("fake wav"), 0644); err != nil {
			t.Fatalf("Failed to create wav file: %v", err)
		}
		if err := os.WriteFile(mp3File, []byte("fake mp3"), 0644); err != nil {
			t.Fatalf("Failed to create mp3 file: %v", err)
		}

		resolver := NewFileResolver([]string{"wav", "mp3", "ogg"})
		result, err := resolver.ResolveWithExtensions(basePath)
		
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != wavFile {
			t.Errorf("Expected wav file %s, got %s", wavFile, result)
		}
	})

	t.Run("finds mp3 when wav doesn't exist", func(t *testing.T) {
		basePath := filepath.Join(tempDir, "error", "bash-error")
		
		// Create directory structure
		errorDir := filepath.Join(tempDir, "error")
		err := os.MkdirAll(errorDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create error dir: %v", err)
		}

		// Create only .mp3 file
		mp3File := basePath + ".mp3"
		if err := os.WriteFile(mp3File, []byte("fake mp3"), 0644); err != nil {
			t.Fatalf("Failed to create mp3 file: %v", err)
		}

		resolver := NewFileResolver([]string{"wav", "mp3", "ogg"})
		result, err := resolver.ResolveWithExtensions(basePath)
		
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != mp3File {
			t.Errorf("Expected mp3 file %s, got %s", mp3File, result)
		}
	})

	t.Run("returns error when no files found", func(t *testing.T) {
		basePath := filepath.Join(tempDir, "nonexistent", "sound")
		
		resolver := NewFileResolver([]string{"wav", "mp3", "ogg"})
		result, err := resolver.ResolveWithExtensions(basePath)
		
		if err == nil {
			t.Error("Expected error for nonexistent files, got nil")
		}
		if result != "" {
			t.Errorf("Expected empty result for nonexistent files, got %s", result)
		}
	})

	t.Run("handles empty base path", func(t *testing.T) {
		resolver := NewFileResolver([]string{"wav", "mp3"})
		result, err := resolver.ResolveWithExtensions("")
		
		if err == nil {
			t.Error("Expected error for empty base path, got nil")
		}
		if result != "" {
			t.Errorf("Expected empty result for empty base path, got %s", result)
		}
	})

	t.Run("handles custom extension order", func(t *testing.T) {
		basePath := filepath.Join(tempDir, "custom", "sound")
		
		// Create directory structure
		customDir := filepath.Join(tempDir, "custom")
		err := os.MkdirAll(customDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create custom dir: %v", err)
		}

		// Create both files
		wavFile := basePath + ".wav"
		oggFile := basePath + ".ogg"
		
		if err := os.WriteFile(wavFile, []byte("fake wav"), 0644); err != nil {
			t.Fatalf("Failed to create wav file: %v", err)
		}
		if err := os.WriteFile(oggFile, []byte("fake ogg"), 0644); err != nil {
			t.Fatalf("Failed to create ogg file: %v", err)
		}

		// Custom order: ogg before wav
		resolver := NewFileResolver([]string{"ogg", "wav", "mp3"})
		result, err := resolver.ResolveWithExtensions(basePath)
		
		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if result != oggFile {
			t.Errorf("Expected ogg file %s, got %s", oggFile, result)
		}
	})
}

func TestFileResolver_GetSupportedExtensions(t *testing.T) {
	t.Run("returns configured extensions", func(t *testing.T) {
		expected := []string{"wav", "mp3", "ogg"}
		resolver := NewFileResolver(expected)
		
		result := resolver.GetSupportedExtensions()
		
		if len(result) != len(expected) {
			t.Errorf("Expected %d extensions, got %d", len(expected), len(result))
		}
		
		for i, ext := range expected {
			if result[i] != ext {
				t.Errorf("Expected extension %s at index %d, got %s", ext, i, result[i])
			}
		}
	})
}

func TestNewFileResolver(t *testing.T) {
	t.Run("creates resolver with extensions", func(t *testing.T) {
		extensions := []string{"wav", "mp3"}
		resolver := NewFileResolver(extensions)
		
		if resolver == nil {
			t.Error("Expected non-nil resolver")
		}
		
		result := resolver.GetSupportedExtensions()
		if len(result) != 2 {
			t.Errorf("Expected 2 extensions, got %d", len(result))
		}
	})

	t.Run("handles empty extensions list", func(t *testing.T) {
		resolver := NewFileResolver([]string{})
		
		if resolver == nil {
			t.Error("Expected non-nil resolver even with empty extensions")
		}
		
		result := resolver.GetSupportedExtensions()
		if len(result) != 0 {
			t.Errorf("Expected 0 extensions, got %d", len(result))
		}
	})
}