package audio

import (
	"context"
	"errors"
	"testing"
)

func TestFakeBackend_RegisteredAsFake(t *testing.T) {
	ResetLastFakeBackend()
	be, err := NewBackend("fake")
	if err != nil {
		t.Fatalf("NewBackend(\"fake\"): unexpected error: %v", err)
	}
	if be == nil {
		t.Fatal("NewBackend(\"fake\") returned nil backend")
	}
	if _, ok := be.(*FakeBackend); !ok {
		t.Errorf("expected *FakeBackend, got %T", be)
	}
}

func TestFakeBackend_RecordsPlay(t *testing.T) {
	ResetLastFakeBackend()
	be, err := NewBackend("fake")
	if err != nil {
		t.Fatalf("NewBackend(\"fake\"): %v", err)
	}

	src := NewFileSource("/some/test/sound.wav")
	if err := be.Play(context.Background(), src); err != nil {
		t.Fatalf("Play: %v", err)
	}

	fake := LastFakeBackend()
	if fake == nil {
		t.Fatal("LastFakeBackend returned nil after Play")
	}

	plays := fake.Plays()
	if len(plays) != 1 {
		t.Fatalf("expected 1 play recorded, got %d", len(plays))
	}
	if plays[0].SourcePath != "/some/test/sound.wav" {
		t.Errorf("expected SourcePath=/some/test/sound.wav, got %q", plays[0].SourcePath)
	}
	// Default volume is 1.0.
	if plays[0].Volume != 1.0 {
		t.Errorf("expected Volume=1.0, got %v", plays[0].Volume)
	}
}

func TestFakeBackend_VolumeAffectsRecordedPlays(t *testing.T) {
	ResetLastFakeBackend()
	be, err := NewBackend("fake")
	if err != nil {
		t.Fatalf("NewBackend(\"fake\"): %v", err)
	}
	if err := be.SetVolume(0.5); err != nil {
		t.Fatalf("SetVolume: %v", err)
	}
	if got := be.GetVolume(); got != 0.5 {
		t.Errorf("GetVolume=%v, want 0.5", got)
	}

	src := NewFileSource("/x.wav")
	if err := be.Play(context.Background(), src); err != nil {
		t.Fatalf("Play: %v", err)
	}
	plays := LastFakeBackend().Plays()
	if len(plays) != 1 || plays[0].Volume != 0.5 {
		t.Errorf("expected one play at volume 0.5, got %+v", plays)
	}
}

func TestFakeBackend_VolumeOutOfRangeRejected(t *testing.T) {
	be := NewFakeBackend()
	if err := be.SetVolume(-0.1); err == nil {
		t.Error("expected error for volume=-0.1")
	}
	if err := be.SetVolume(1.1); err == nil {
		t.Error("expected error for volume=1.1")
	}
}

func TestFakeBackend_CloseRejectsPlay(t *testing.T) {
	be := NewFakeBackend()
	if err := be.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !be.Closed() {
		t.Error("Closed() should return true after Close")
	}
	err := be.Play(context.Background(), NewFileSource("/x.wav"))
	if !errors.Is(err, ErrBackendClosed) {
		t.Errorf("expected ErrBackendClosed after Close, got %v", err)
	}
}

func TestLastFakeBackend_TracksMostRecent(t *testing.T) {
	ResetLastFakeBackend()
	first, err := NewBackend("fake")
	if err != nil {
		t.Fatalf("first NewBackend(\"fake\"): %v", err)
	}
	if LastFakeBackend() != first.(*FakeBackend) {
		t.Error("LastFakeBackend did not return the first construction")
	}
	second, err := NewBackend("fake")
	if err != nil {
		t.Fatalf("second NewBackend(\"fake\"): %v", err)
	}
	if LastFakeBackend() == first.(*FakeBackend) {
		t.Error("LastFakeBackend should track the most recent backend")
	}
	if LastFakeBackend() != second.(*FakeBackend) {
		t.Error("LastFakeBackend did not return the second construction")
	}
}

func TestFakeBackend_StopFlipsIsPlaying(t *testing.T) {
	be := NewFakeBackend()
	src := NewFileSource("/x.wav")
	if err := be.Play(context.Background(), src); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if !be.IsPlaying() {
		t.Error("expected IsPlaying after Play")
	}
	if err := be.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if be.IsPlaying() {
		t.Error("expected !IsPlaying after Stop")
	}
}

func TestFakeBackend_IsValidBackendType(t *testing.T) {
	if !IsValidBackendType("fake") {
		t.Error("IsValidBackendType(\"fake\") should return true")
	}
}
