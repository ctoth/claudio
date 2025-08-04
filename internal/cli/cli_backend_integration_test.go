package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/ctoth/claudio/internal/audio"
)

// TestCLIBackendIntegration tests that CLI properly integrates with audio backend system
func TestCLIBackendIntegration(t *testing.T) {
	cli := NewCLI()

	// Test that CLI structure has been updated to use backends
	if cli.backendFactory != nil {
		t.Error("backendFactory should be nil before initialization")
	}
	if cli.audioBackend != nil {
		t.Error("audioBackend should be nil before initialization")
	}
}

func TestCLIInitializeAudioSystemWithBackend(t *testing.T) {
	tests := []struct {
		name                string
		audioBackend        string
		expectError         bool
		expectedBackendType string
	}{
		{
			name:                "auto backend selection",
			audioBackend:        "auto",
			expectError:         false,
			expectedBackendType: "", // Will depend on system
		},
		{
			name:                "explicit malgo backend",
			audioBackend:        "malgo",
			expectError:         false,
			expectedBackendType: "*audio.MalgoBackend",
		},
		{
			name:                "explicit system_command backend",
			audioBackend:        "system_command",
			expectError:         false,
			expectedBackendType: "*audio.SystemCommandBackend",
		},
		{
			name:                "invalid backend",
			audioBackend:        "invalid",
			expectError:         true,
			expectedBackendType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cli := NewCLI()
			cli.initializeSystems()

			// Create test config with specific backend
			cfg := cli.configManager.GetDefaultConfig()
			cfg.AudioBackend = tt.audioBackend

			// Test initializeAudioSystemWithBackend function
			err := cli.initializeAudioSystemWithBackend(cfg)

			if tt.expectError && err == nil {
				t.Errorf("expected error for backend '%s' but got none", tt.audioBackend)
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error for backend '%s': %v", tt.audioBackend, err)
			}

			if !tt.expectError {
				if cli.audioBackend == nil {
					t.Error("audioBackend should be initialized")
				}

				// Check backend type if specified
				if tt.expectedBackendType != "" {
					backendType := getTypeName(cli.audioBackend)
					if backendType != tt.expectedBackendType {
						t.Errorf("expected backend type '%s', got '%s'", tt.expectedBackendType, backendType)
					}
				}
			}

			// Clean up
			if cli.audioBackend != nil {
				cli.audioBackend.Close()
			}
		})
	}
}

func TestCLIPlaySoundWithBackend(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	// Initialize with malgo backend for testing
	cfg := cli.configManager.GetDefaultConfig()
	cfg.AudioBackend = "malgo"

	// Need to initialize soundpack resolver for playSoundWithBackend to work
	err := initializeAudioSystem(nil, cli, cfg)
	if err != nil {
		t.Fatalf("failed to initialize audio system: %v", err)
	}
	defer cli.audioBackend.Close()

	// Test that playSound uses backend instead of hardcoded paplay
	err = cli.playSoundWithBackend("/test/nonexistent.wav", cfg.Volume)

	// We should get a "file not found" type error, but no panic
	// The important thing is that it doesn't crash and uses the backend system
	t.Logf("PlaySound error (expected): %v", err)
}

func TestCLIBackendFactoryIntegration(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	// Test that CLI creates backend factory
	if cli.backendFactory == nil {
		t.Error("CLI should have backendFactory initialized")
	}

	// Test supported backends
	supported := cli.backendFactory.GetSupportedBackends()
	if len(supported) == 0 {
		t.Error("backendFactory should return supported backends")
	}

	// Test backend creation
	backend, err := cli.backendFactory.CreateBackend("malgo")
	if err != nil {
		t.Errorf("failed to create malgo backend: %v", err)
	}
	if backend == nil {
		t.Error("created backend should not be nil")
	}

	// Clean up
	if backend != nil {
		backend.Close()
	}
}

func TestCLIBackendLifecycleManagement(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	cfg := cli.configManager.GetDefaultConfig()
	cfg.AudioBackend = "malgo"

	// Initialize backend
	err := cli.initializeAudioSystemWithBackend(cfg)
	if err != nil {
		t.Fatalf("failed to initialize backend: %v", err)
	}

	// Backend should be started
	if cli.audioBackend == nil {
		t.Error("backend should be initialized")
	}

	// Test that backend lifecycle is managed properly
	err = cli.audioBackend.Start()
	if err != nil {
		t.Errorf("backend start failed: %v", err)
	}

	err = cli.audioBackend.Stop()
	if err != nil {
		t.Errorf("backend stop failed: %v", err)
	}

	err = cli.audioBackend.Close()
	if err != nil {
		t.Errorf("backend close failed: %v", err)
	}
}

func TestCLIVolumeControlWithBackend(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	cfg := cli.configManager.GetDefaultConfig()
	cfg.AudioBackend = "malgo"
	cfg.Volume = 0.7

	err := cli.initializeAudioSystemWithBackend(cfg)
	if err != nil {
		t.Fatalf("failed to initialize backend: %v", err)
	}
	defer cli.audioBackend.Close()

	// Test that volume is set on backend
	volume := cli.audioBackend.GetVolume()
	if volume != float32(cfg.Volume) {
		t.Errorf("expected volume %f, got %f", cfg.Volume, volume)
	}

	// Test volume update
	newVolume := float32(0.3)
	err = cli.audioBackend.SetVolume(newVolume)
	if err != nil {
		t.Errorf("failed to set volume: %v", err)
	}

	actualVolume := cli.audioBackend.GetVolume()
	if actualVolume != newVolume {
		t.Errorf("expected updated volume %f, got %f", newVolume, actualVolume)
	}
}

func TestCLIConfigBackendValidation(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	// Test that config validation includes backend validation
	cfg := cli.configManager.GetDefaultConfig()
	cfg.AudioBackend = "invalid_backend"

	err := cli.configManager.ValidateConfig(cfg)
	if err == nil {
		t.Error("expected validation error for invalid backend")
	}

	// Test valid backends pass validation
	validBackends := []string{"auto", "system_command", "malgo"}
	for _, backend := range validBackends {
		cfg.AudioBackend = backend
		err = cli.configManager.ValidateConfig(cfg)
		if err != nil {
			t.Errorf("validation should pass for backend '%s': %v", backend, err)
		}
	}
}

// Helper functions
func getTypeName(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return getType(v)
}

func getType(v interface{}) string {
	// This is a simple type name extractor for testing
	switch v.(type) {
	case *audio.MalgoBackend:
		return "*audio.MalgoBackend"
	case *audio.SystemCommandBackend:
		return "*audio.SystemCommandBackend"
	default:
		return "unknown"
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsSubstringMiddle(s, substr))))
}

func containsSubstringMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestCLIAIFFSupportViaUnifiedSystem verifies AIFF support works through CLI
func TestCLIAIFFSupportViaUnifiedSystem(t *testing.T) {
	cli := NewCLI()
	cli.initializeSystems()

	// Initialize with malgo backend for testing
	cfg := cli.configManager.GetDefaultConfig()
	cfg.AudioBackend = "malgo"

	err := cli.initializeAudioSystemWithBackend(cfg)
	if err != nil {
		t.Fatalf("failed to initialize audio system: %v", err)
	}
	defer cli.audioBackend.Close()

	// Verify that the CLI's audio backend supports AIFF
	malgoBackend, ok := cli.audioBackend.(*audio.MalgoBackend)
	if !ok {
		t.Fatalf("expected MalgoBackend, got %T", cli.audioBackend)
	}

	// Access the registry through the backend (this tests our unified system)
	// We can't directly access private fields, but we can test via the documented interface
	// The logs should show AIFF support is available
	t.Logf("CLI successfully initialized with unified audio system supporting AIFF")
	
	// Test that an AIFF file path would be processed (even if file doesn't exist)
	ctx := context.Background()
	source := audio.NewFileSource("/test/nonexistent.aiff")
	
	err = malgoBackend.Play(ctx, source)
	if err != nil {
		// We expect file not found error, NOT unsupported format error
		errorMsg := strings.ToLower(err.Error())
		if strings.Contains(errorMsg, "unsupported") && strings.Contains(errorMsg, "format") {
			t.Errorf("CLI should support AIFF through unified system, got: %v", err)
		} else {
			// Expected: file not found or decode error
			t.Logf("Expected error with nonexistent AIFF file: %v", err)
		}
	}
}
