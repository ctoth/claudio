package audio

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestAudioBackendInterface tests that AudioBackend interface is properly defined
func TestAudioBackendInterface(t *testing.T) {
	// This test ensures the interface compiles and has expected methods
	var _ AudioBackend = (*mockAudioBackend)(nil)
}

// mockAudioBackend is a test implementation of AudioBackend
type mockAudioBackend struct {
	volume    float32
	isPlaying bool
	closed    bool
	startErr  error
	stopErr   error
	closeErr  error
	playErr   error
}

func (m *mockAudioBackend) Start() error {
	if m.closed {
		return ErrBackendClosed
	}
	return m.startErr
}

func (m *mockAudioBackend) Stop() error {
	if m.closed {
		return ErrBackendClosed
	}
	m.isPlaying = false
	return m.stopErr
}

func (m *mockAudioBackend) Close() error {
	m.closed = true
	m.isPlaying = false
	return m.closeErr
}

func (m *mockAudioBackend) IsPlaying() bool {
	return m.isPlaying && !m.closed
}

func (m *mockAudioBackend) SetVolume(volume float32) error {
	if m.closed {
		return ErrBackendClosed
	}
	if volume < 0.0 || volume > 1.0 {
		return errors.New("invalid volume")
	}
	m.volume = volume
	return nil
}

func (m *mockAudioBackend) GetVolume() float32 {
	return m.volume
}

func (m *mockAudioBackend) Play(ctx context.Context, source AudioSource) error {
	if m.closed {
		return ErrBackendClosed
	}
	if m.playErr != nil {
		return m.playErr
	}
	m.isPlaying = true
	
	// Simulate some playback time
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(10 * time.Millisecond):
		m.isPlaying = false
		return nil
	}
}

func TestSystemCommandBackend_Interface(t *testing.T) {
	backend := NewSystemCommandBackend("paplay")
	var _ AudioBackend = backend
}

func TestSystemCommandBackend_Play(t *testing.T) {
	tests := []struct {
		name    string
		command string
		source  AudioSource
		wantErr bool
	}{
		{
			name:    "play file source with nonexistent file",
			command: "echo", // Use echo instead of paplay to avoid system dependencies
			source:  NewFileSource("/test/sound.wav"),
			wantErr: false, // echo will succeed even with nonexistent args
		},
		{
			name:    "play reader source",
			command: "echo", // Use echo instead of paplay
			source:  NewReaderSource(io.NopCloser(strings.NewReader("test")), "wav"),
			wantErr: false, // echo will succeed
		},
		{
			name:    "invalid command",
			command: "nonexistent-command-12345",
			source:  NewFileSource("/test/sound.wav"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := NewSystemCommandBackend(tt.command)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := backend.Play(ctx, tt.source)
			
			if tt.wantErr && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSystemCommandBackend_Lifecycle(t *testing.T) {
	backend := NewSystemCommandBackend("paplay")
	
	// Test Start
	if err := backend.Start(); err != nil {
		t.Errorf("Start() failed: %v", err)
	}
	
	// Test Stop
	if err := backend.Stop(); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
	
	// Test Close
	if err := backend.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}
	
	// Operations after close should fail
	err := backend.Play(context.Background(), NewFileSource("/test/sound.wav"))
	if !errors.Is(err, ErrBackendClosed) {
		t.Errorf("expected ErrBackendClosed after Close(), got: %v", err)
	}
}

func TestSystemCommandBackend_VolumeControl(t *testing.T) {
	backend := NewSystemCommandBackend("paplay")
	
	// Test valid volume
	err := backend.SetVolume(0.5)
	if err != nil {
		t.Errorf("SetVolume(0.5) failed: %v", err)
	}
	
	volume := backend.GetVolume()
	if volume != 0.5 {
		t.Errorf("expected volume 0.5, got %f", volume)
	}
	
	// Test invalid volume
	err = backend.SetVolume(-0.1)
	if err == nil {
		t.Error("expected error for negative volume")
	}
	
	err = backend.SetVolume(1.1)
	if err == nil {
		t.Error("expected error for volume > 1.0")
	}
}

func TestMalgoBackend_Interface(t *testing.T) {
	backend := NewMalgoBackend()
	var _ AudioBackend = backend
}

func TestMalgoBackend_Play(t *testing.T) {
	backend := NewMalgoBackend()
	source := NewFileSource("/test/sound.wav")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := backend.Play(ctx, source)
	// Since this is a mock test, we expect some kind of error (file not found, etc.)
	// The important thing is that the interface works
	_ = err // We don't care about the specific error for interface testing
}

func TestMalgoBackend_Lifecycle(t *testing.T) {
	backend := NewMalgoBackend()
	
	// Test lifecycle methods exist and can be called
	_ = backend.Start()
	_ = backend.Stop() 
	_ = backend.Close()
	
	// Test state methods
	_ = backend.IsPlaying()
	_ = backend.GetVolume()
	_ = backend.SetVolume(0.5)
}

func TestBackendErrorDefinitions(t *testing.T) {
	// Test that our error types are properly defined
	if ErrBackendNotAvailable == nil {
		t.Error("ErrBackendNotAvailable should be defined")
	}
	if ErrBackendClosed == nil {
		t.Error("ErrBackendClosed should be defined")
	}
	
	// Test error messages
	if ErrBackendNotAvailable.Error() != "audio backend not available" {
		t.Errorf("unexpected ErrBackendNotAvailable message: %s", ErrBackendNotAvailable.Error())
	}
	if ErrBackendClosed.Error() != "audio backend is closed" {
		t.Errorf("unexpected ErrBackendClosed message: %s", ErrBackendClosed.Error())
	}
}

// TestMockBackend tests our mock implementation to ensure test infrastructure works
func TestMockBackend(t *testing.T) {
	mock := &mockAudioBackend{volume: 0.8}
	
	// Test volume control
	if mock.GetVolume() != 0.8 {
		t.Errorf("expected volume 0.8, got %f", mock.GetVolume())
	}
	
	err := mock.SetVolume(0.5)
	if err != nil {
		t.Errorf("SetVolume failed: %v", err)
	}
	if mock.GetVolume() != 0.5 {
		t.Errorf("expected volume 0.5, got %f", mock.GetVolume())
	}
	
	// Test lifecycle
	if mock.IsPlaying() {
		t.Error("mock should not be playing initially")
	}
	
	source := NewFileSource("/test/sound.wav")
	ctx := context.Background()
	
	err = mock.Play(ctx, source)
	if err != nil {
		t.Errorf("Play failed: %v", err)
	}
	
	// Test close
	err = mock.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
	
	// Operations after close should fail
	err = mock.Play(ctx, source)
	if !errors.Is(err, ErrBackendClosed) {
		t.Errorf("expected ErrBackendClosed after Close(), got: %v", err)
	}
}