package audio

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
)

func TestAudioPlayer(t *testing.T) {
	player := NewAudioPlayer()
	
	if player == nil {
		t.Fatal("NewAudioPlayer returned nil")
	}
	
	// Test that player implements expected interface
	var _ Player = player
}

func TestAudioPlayerInitialization(t *testing.T) {
	player := NewAudioPlayer()
	
	// Test initial state
	if player.IsPlaying() {
		t.Error("player should not be playing initially")
	}
	
	if player.GetVolume() != 1.0 {
		t.Errorf("expected default volume 1.0, got %f", player.GetVolume())
	}
}

func TestAudioPlayerVolumeControl(t *testing.T) {
	player := NewAudioPlayer()
	
	testCases := []struct {
		volume   float32
		expected float32
		valid    bool
	}{
		{0.0, 0.0, true},   // Mute
		{0.5, 0.5, true},   // Half volume
		{1.0, 1.0, true},   // Full volume
		{-0.1, 1.0, false}, // Invalid: negative
		{1.1, 1.0, false},  // Invalid: too high
		{0.75, 0.75, true}, // Valid: 75%
	}
	
	for _, tc := range testCases {
		err := player.SetVolume(tc.volume)
		
		if tc.valid && err != nil {
			t.Errorf("SetVolume(%f) should be valid but got error: %v", tc.volume, err)
		}
		
		if !tc.valid && err == nil {
			t.Errorf("SetVolume(%f) should be invalid but no error returned", tc.volume)
		}
		
		if player.GetVolume() != tc.expected {
			t.Errorf("after SetVolume(%f), GetVolume() = %f, expected %f", 
				tc.volume, player.GetVolume(), tc.expected)
		}
	}
}

func TestAudioPlayerPreloadSound(t *testing.T) {
	player := NewAudioPlayer()
	
	// Create test audio data
	testData := &AudioData{
		Samples:    []byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04},
		Channels:   2,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	t.Run("successful preload", func(t *testing.T) {
		soundID := "test-sound"
		err := player.PreloadSound(soundID, testData)
		
		if err != nil {
			t.Fatalf("PreloadSound failed: %v", err)
		}
		
		// Verify sound is preloaded
		if !player.IsSoundLoaded(soundID) {
			t.Error("sound should be reported as loaded after preload")
		}
	})
	
	t.Run("preload with nil data", func(t *testing.T) {
		err := player.PreloadSound("nil-sound", nil)
		
		if err == nil {
			t.Fatal("PreloadSound should fail with nil audio data")
		}
	})
	
	t.Run("preload with empty sound ID", func(t *testing.T) {
		err := player.PreloadSound("", testData)
		
		if err == nil {
			t.Fatal("PreloadSound should fail with empty sound ID")
		}
	})
	
	t.Run("preload overwrites existing", func(t *testing.T) {
		soundID := "overwrite-test"
		
		// First preload
		err := player.PreloadSound(soundID, testData)
		if err != nil {
			t.Fatalf("First preload failed: %v", err)
		}
		
		// Second preload should overwrite
		newData := &AudioData{
			Samples:    []byte{0xFF, 0xFE, 0xFD, 0xFC},
			Channels:   1,
			SampleRate: 22050,
			Format:     malgo.FormatS16,
		}
		
		err = player.PreloadSound(soundID, newData)
		if err != nil {
			t.Fatalf("Overwrite preload failed: %v", err)
		}
		
		if !player.IsSoundLoaded(soundID) {
			t.Error("sound should still be loaded after overwrite")
		}
	})
}

func TestAudioPlayerPlaySound(t *testing.T) {
	player := NewAudioPlayer()
	
	// Create test audio data
	testData := &AudioData{
		Samples:    []byte{0x00, 0x01, 0x00, 0x02, 0x00, 0x03, 0x00, 0x04},
		Channels:   2,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	t.Run("play preloaded sound", func(t *testing.T) {
		soundID := "play-test"
		
		// Preload first
		err := player.PreloadSound(soundID, testData)
		if err != nil {
			t.Fatalf("PreloadSound failed: %v", err)
		}
		
		// Play the sound
		err = player.PlaySound(soundID)
		if err != nil {
			t.Fatalf("PlaySound failed: %v", err)
		}
		
		// Note: We can't easily test actual audio output in unit tests,
		// but we can verify the call succeeded
	})
	
	t.Run("play non-existent sound", func(t *testing.T) {
		err := player.PlaySound("non-existent")
		
		if err == nil {
			t.Fatal("PlaySound should fail for non-existent sound")
		}
	})
	
	t.Run("play with empty sound ID", func(t *testing.T) {
		err := player.PlaySound("")
		
		if err == nil {
			t.Fatal("PlaySound should fail with empty sound ID")
		}
	})
}

func TestAudioPlayerPlaySoundWithTimeout(t *testing.T) {
	player := NewAudioPlayer()
	
	// Create short test audio data (very brief)
	testData := &AudioData{
		Samples:    []byte{0x00, 0x01}, // Very short sample
		Channels:   1,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	soundID := "timeout-test"
	err := player.PreloadSound(soundID, testData)
	if err != nil {
		t.Fatalf("PreloadSound failed: %v", err)
	}
	
	t.Run("play with sufficient timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		err := player.PlaySoundWithContext(ctx, soundID)
		if err != nil {
			t.Fatalf("PlaySoundWithContext failed: %v", err)
		}
	})
	
	t.Run("play with very short timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Microsecond)
		cancel() // Cancel immediately
		
		err := player.PlaySoundWithContext(ctx, soundID)
		if err == nil {
			t.Fatal("PlaySoundWithContext should fail with cancelled context")
		}
	})
}

func TestAudioPlayerConcurrentPlayback(t *testing.T) {
	player := NewAudioPlayer()
	
	// Create test audio data
	testData := &AudioData{
		Samples:    []byte{0x00, 0x01, 0x00, 0x02},
		Channels:   1,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	// Preload multiple sounds
	sounds := []string{"sound1", "sound2", "sound3"}
	for _, soundID := range sounds {
		err := player.PreloadSound(soundID, testData)
		if err != nil {
			t.Fatalf("PreloadSound failed for %s: %v", soundID, err)
		}
	}
	
	t.Run("concurrent playback", func(t *testing.T) {
		// Play all sounds concurrently
		errChan := make(chan error, len(sounds))
		
		for _, soundID := range sounds {
			go func(id string) {
				errChan <- player.PlaySound(id)
			}(soundID)
		}
		
		// Collect results
		for i := 0; i < len(sounds); i++ {
			err := <-errChan
			if err != nil {
				t.Errorf("Concurrent playback failed: %v", err)
			}
		}
	})
}

func TestAudioPlayerStop(t *testing.T) {
	player := NewAudioPlayer()
	
	t.Run("stop when not playing", func(t *testing.T) {
		err := player.Stop()
		if err != nil {
			t.Errorf("Stop should not fail when not playing: %v", err)
		}
	})
	
	t.Run("stop all sounds", func(t *testing.T) {
		err := player.StopAll()
		if err != nil {
			t.Errorf("StopAll should not fail: %v", err)
		}
	})
}

func TestAudioPlayerCleanup(t *testing.T) {
	player := NewAudioPlayer()
	
	// Create and preload test sound
	testData := &AudioData{
		Samples:    []byte{0x00, 0x01, 0x00, 0x02},
		Channels:   1,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	
	soundID := "cleanup-test"
	err := player.PreloadSound(soundID, testData)
	if err != nil {
		t.Fatalf("PreloadSound failed: %v", err)
	}
	
	t.Run("unload specific sound", func(t *testing.T) {
		if !player.IsSoundLoaded(soundID) {
			t.Fatal("sound should be loaded before unload test")
		}
		
		err := player.UnloadSound(soundID)
		if err != nil {
			t.Fatalf("UnloadSound failed: %v", err)
		}
		
		if player.IsSoundLoaded(soundID) {
			t.Error("sound should not be loaded after unload")
		}
	})
	
	t.Run("close player", func(t *testing.T) {
		err := player.Close()
		if err != nil {
			t.Fatalf("Close failed: %v", err)
		}
		
		// After close, operations should fail
		err = player.PlaySound("any-sound")
		if err == nil {
			t.Error("PlaySound should fail after Close")
		}
	})
}

func TestAudioPlayerInterface(t *testing.T) {
	// Verify that our player implements the expected interface
	var player Player = NewAudioPlayer()
	
	// Test interface methods exist (compilation check)
	_ = player.IsPlaying()
	_ = player.GetVolume()
	_ = player.SetVolume(1.0)
	_ = player.IsSoundLoaded("test")
	_ = player.PreloadSound("test", nil)
	_ = player.PlaySound("test")
	_ = player.PlaySoundWithContext(context.Background(), "test")
	_ = player.Stop()
	_ = player.StopAll()
	_ = player.UnloadSound("test")
	_ = player.Close()
}

func TestAudioLoggingLevels(t *testing.T) {
	// TDD RED: This test should FAIL because routine audio operations currently use INFO logging
	// We expect routine audio operations to use DEBUG level, not INFO level
	
	// Capture log output to verify log levels
	var logBuffer bytes.Buffer
	originalHandler := slog.Default().Handler()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuffer, &slog.HandlerOptions{
		Level: slog.LevelDebug, // Capture all logs
	})))
	defer slog.SetDefault(slog.New(originalHandler))
	
	// Test audio player creation - should be DEBUG level
	player := NewAudioPlayer()
	defer func() {
		if err := player.Close(); err != nil {
			t.Logf("Error closing player: %v", err)
		}
	}()
	
	// Test volume change - should be DEBUG level
	err := player.SetVolume(0.8)
	if err != nil {
		t.Fatalf("SetVolume should not error: %v", err)
	}
	
	logOutput := logBuffer.String()
	
	// CRITICAL: Routine operations should use DEBUG level, not INFO
	problematicInfoLogs := []string{
		"audio player created successfully",
		"volume changed",
		"sound playback started successfully", 
		"audio player closed successfully",
		"all sound playback stopped",
	}
	
	for _, logMsg := range problematicInfoLogs {
		if strings.Contains(logOutput, logMsg) {
			// Check if it appears with INFO level (bad) vs DEBUG level (good)
			if strings.Contains(logOutput, "level=INFO") && strings.Contains(logOutput, logMsg) {
				t.Errorf("Routine operation '%s' should use DEBUG level, not INFO level", logMsg)
				t.Logf("Full log output: %s", logOutput)
			}
		}
	}
	
	// Verify that DEBUG logs are working properly
	if !strings.Contains(logOutput, "level=DEBUG") {
		t.Error("Expected some DEBUG level logs but found none")
		t.Logf("Full log output: %s", logOutput)
	}
}