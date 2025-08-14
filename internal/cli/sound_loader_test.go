package cli

import (
	"os"
	"path/filepath"
	"testing"

	"claudio.click/internal/audio"
)

func TestSoundLoader(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()

	// Create a simple soundpack directory structure
	successDir := filepath.Join(tempDir, "test-pack", "success")
	err := os.MkdirAll(successDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test soundpack dir: %v", err)
	}

	// Create a simple WAV file for testing (minimal valid WAV)
	wavFile := filepath.Join(successDir, "bash-success.wav")
	wavData := createMinimalWAV()
	err = os.WriteFile(wavFile, wavData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test WAV file: %v", err)
	}

	loader := NewSoundLoader([]string{filepath.Join(tempDir, "test-pack")})

	t.Run("load existing sound file", func(t *testing.T) {
		audioData, err := loader.LoadSound("success/bash-success.wav")
		if err != nil {
			t.Errorf("Expected to load sound, got error: %v", err)
		}

		if audioData == nil {
			t.Error("Expected audio data, got nil")
		}

		if audioData != nil {
			if audioData.Channels == 0 {
				t.Error("Expected non-zero channels")
			}
			if audioData.SampleRate == 0 {
				t.Error("Expected non-zero sample rate")
			}
			if len(audioData.Samples) == 0 {
				t.Error("Expected non-empty sample data")
			}
		}
	})

	t.Run("file not found error", func(t *testing.T) {
		_, err := loader.LoadSound("success/nonexistent.wav")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}

		if !IsFileNotFoundError(err) {
			t.Errorf("Expected file not found error, got: %v", err)
		}
	})

	t.Run("invalid file format", func(t *testing.T) {
		// Create an invalid audio file
		invalidFile := filepath.Join(successDir, "invalid.wav")
		err := os.WriteFile(invalidFile, []byte("not a wav file"), 0644)
		if err != nil {
			t.Fatalf("Failed to create invalid file: %v", err)
		}

		_, err = loader.LoadSound("success/invalid.wav")
		if err == nil {
			t.Error("Expected error for invalid file format")
		}
	})

	t.Run("multiple soundpack paths fallback", func(t *testing.T) {
		// Create a second soundpack with the same file
		secondPackDir := filepath.Join(tempDir, "second-pack", "success")
		err := os.MkdirAll(secondPackDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create second soundpack dir: %v", err)
		}

		secondWavFile := filepath.Join(secondPackDir, "bash-success.wav")
		err = os.WriteFile(secondWavFile, wavData, 0644)
		if err != nil {
			t.Fatalf("Failed to create second WAV file: %v", err)
		}

		// Create loader with both paths (first path should be checked first)
		multiLoader := NewSoundLoader([]string{
			filepath.Join(tempDir, "empty-pack"),  // This one doesn't exist
			filepath.Join(tempDir, "second-pack"), // This one has the file
		})

		audioData, err := multiLoader.LoadSound("success/bash-success.wav")
		if err != nil {
			t.Errorf("Expected to load sound from fallback path, got error: %v", err)
		}

		if audioData == nil {
			t.Error("Expected audio data from fallback path")
		}
	})
}

func TestSoundLoaderIntegration(t *testing.T) {
	// Test integration with real AudioPlayer
	tempDir := t.TempDir()

	// Create soundpack structure
	successDir := filepath.Join(tempDir, "integration-pack", "success")
	err := os.MkdirAll(successDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test soundpack dir: %v", err)
	}

	// Create test WAV file
	wavFile := filepath.Join(successDir, "test.wav")
	wavData := createMinimalWAV()
	err = os.WriteFile(wavFile, wavData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test WAV file: %v", err)
	}

	loader := NewSoundLoader([]string{filepath.Join(tempDir, "integration-pack")})
	player := audio.NewAudioPlayer()
	defer player.Close()

	t.Run("load and preload sound", func(t *testing.T) {
		// Load sound data
		audioData, err := loader.LoadSound("success/test.wav")
		if err != nil {
			t.Fatalf("Failed to load sound: %v", err)
		}

		// Preload into player
		err = player.PreloadSound("test-sound", audioData)
		if err != nil {
			t.Fatalf("Failed to preload sound: %v", err)
		}

		// Verify it's loaded
		if !player.IsSoundLoaded("test-sound") {
			t.Error("Sound should be loaded in player")
		}

		// Test playback (won't actually play in test environment)
		err = player.PlaySound("test-sound")
		if err != nil {
			// This might fail in test environment due to no audio devices
			t.Logf("Playback failed (expected in test env): %v", err)
		}
	})
}

// Helper functions for testing

func createMinimalWAV() []byte {
	// Create a minimal valid WAV file (44 bytes header + 8 bytes of audio data)
	wav := []byte{
		// RIFF header
		'R', 'I', 'F', 'F',
		44, 0, 0, 0, // File size - 8 (44 - 8 = 36 + 8 data = 44)
		'W', 'A', 'V', 'E',

		// fmt chunk
		'f', 'm', 't', ' ',
		16, 0, 0, 0, // fmt chunk size
		1, 0, // PCM format
		1, 0, // mono
		0x44, 0xAC, 0, 0, // 44100 Hz sample rate
		0x88, 0x58, 0x01, 0, // byte rate
		2, 0, // block align
		16, 0, // 16 bits per sample

		// data chunk
		'd', 'a', 't', 'a',
		8, 0, 0, 0, // data size
		0, 0, 0x7F, 0x7F, 0, 0, 0x7F, 0x7F, // 4 samples of audio data
	}
	return wav
}

func TestSoundLoader_ResolveSoundPath(t *testing.T) {
	// Create temporary test directory
	tempDir := t.TempDir()

	// Create a simple soundpack directory structure
	successDir := filepath.Join(tempDir, "test-pack", "success")
	err := os.MkdirAll(successDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test soundpack dir: %v", err)
	}

	// Create a simple WAV file for testing
	wavFile := filepath.Join(successDir, "bash-success.wav")
	wavData := createMinimalWAV()
	err = os.WriteFile(wavFile, wavData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test WAV file: %v", err)
	}

	loader := NewSoundLoader([]string{filepath.Join(tempDir, "test-pack")})

	t.Run("resolve existing sound path", func(t *testing.T) {
		fullPath, err := loader.ResolveSoundPath("success/bash-success.wav")
		if err != nil {
			t.Errorf("Expected to resolve sound path, got error: %v", err)
		}

		expectedPath := filepath.Join(tempDir, "test-pack", "success", "bash-success.wav")
		if fullPath != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, fullPath)
		}
	})

	t.Run("path not found error", func(t *testing.T) {
		_, err := loader.ResolveSoundPath("success/nonexistent.wav")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}

		if !IsFileNotFoundError(err) {
			t.Errorf("Expected file not found error, got: %v", err)
		}
	})

	t.Run("empty path error", func(t *testing.T) {
		_, err := loader.ResolveSoundPath("")
		if err == nil {
			t.Error("Expected error for empty path")
		}
	})
}
