package audio

import (
	"bytes"
	"context"
	"log/slog"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
)

// skipIfNoAudioDevice skips the test if the error indicates no audio device
// is available (e.g. CI runners without sound hardware).
func skipIfNoAudioDevice(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		return
	}
	msg := err.Error()
	if strings.Contains(msg, "Failed to open backend device") ||
		strings.Contains(msg, "failed to initialize playback device") {
		t.Skip("no audio device available")
	}
}

func TestAudioPlayer(t *testing.T) {
	player := NewAudioPlayer()

	if player == nil {
		t.Fatal("NewAudioPlayer returned nil")
	}
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
		skipIfNoAudioDevice(t, err)
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
		skipIfNoAudioDevice(t, err)
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
			skipIfNoAudioDevice(t, err)
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

// TestSetVolumeGetVolume_ConcurrentNoRace exercises the lock-free volume
// access pattern that supports the realtime audio callback. The constructor
// is lazy and does NOT initialise a malgo device, so this test runs in any
// CI without audio hardware.
//
// Run with `go test -race -count=3` — must produce zero data races. Before
// the atomic.Uint32 conversion, GetVolume acquired an RLock from inside the
// malgo onSamples callback, which is racy by design (realtime audio callbacks
// must never lock) and likely contributed to the documented crackling.
//
// This is the load-bearing regression guard against future de-atomicisation
// of the volume read path.
func TestSetVolumeGetVolume_ConcurrentNoRace(t *testing.T) {
	player := NewAudioPlayer()
	defer func() {
		if err := player.Close(); err != nil {
			t.Logf("error closing player: %v", err)
		}
	}()

	const iters = 10_000
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			v := float32(i%101) / 100.0 // 0.00..1.00 inclusive
			if err := player.SetVolume(v); err != nil {
				t.Errorf("SetVolume(%v): %v", v, err)
				return
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iters; i++ {
			v := player.GetVolume()
			if v < 0.0 || v > 1.0 {
				t.Errorf("GetVolume out of range: %v", v)
				return
			}
		}
	}()

	wg.Wait()
}

// TestSetVolume_RejectsNonFinite verifies the NaN/Inf input guard added to
// match SystemCommandBackend.SetVolume's input contract.
func TestSetVolume_RejectsNonFinite(t *testing.T) {
	player := NewAudioPlayer()
	defer func() { _ = player.Close() }()

	nonFinite := []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	}
	for _, v := range nonFinite {
		err := player.SetVolume(v)
		if err == nil {
			t.Errorf("SetVolume(%v) should reject non-finite input but returned nil", v)
		}
	}

	// The valid volume should remain whatever it was before the rejected
	// writes — the default 1.0 set by NewAudioPlayer.
	if got := player.GetVolume(); got != 1.0 {
		t.Errorf("after rejected non-finite writes, GetVolume()=%v, want 1.0", got)
	}
}

// TestPlayWithConcurrentSetVolume_NoRace exercises the actual realtime
// callback path under concurrent SetVolume mutation. Requires audio hardware
// and is skipped on CI runners without a device. The device-free hammer test
// above is the load-bearing regression — this one is icing for dev boxes.
func TestPlayWithConcurrentSetVolume_NoRace(t *testing.T) {
	player := NewAudioPlayer()
	defer func() { _ = player.Close() }()

	// Tiny PCM blob — enough to exercise the callback a few times.
	sound := &AudioData{
		Samples:    make([]byte, 16*1024),
		Channels:   2,
		SampleRate: 44100,
		Format:     malgo.FormatS16,
	}
	soundID := "test-concurrent-volume"
	if err := player.PreloadSound(soundID, sound); err != nil {
		t.Fatalf("PreloadSound: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 1000; i++ {
			_ = player.SetVolume(float32(i%101) / 100.0)
		}
	}()

	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		err := player.PlaySoundWithContext(ctx, soundID)
		cancel()
		// First call surfaces "no audio device" if hardware is missing; skip
		// the whole test in that case rather than asserting failure.
		skipIfNoAudioDevice(t, err)
		if err != nil {
			t.Errorf("PlaySoundWithContext: %v", err)
			break
		}
	}
	<-done
}
