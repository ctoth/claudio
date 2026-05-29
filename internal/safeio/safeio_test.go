package safeio

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestReadAllCapped_HappyPath(t *testing.T) {
	payload := []byte("hello world")
	r := bytes.NewReader(payload)

	got, err := ReadAllCapped(r, 1024, "hook payload")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestReadAllCapped_ExactlyAtCap(t *testing.T) {
	const cap int64 = 32
	payload := bytes.Repeat([]byte("a"), int(cap))
	r := bytes.NewReader(payload)

	got, err := ReadAllCapped(r, cap, "test data")
	if err != nil {
		t.Fatalf("input exactly at cap should succeed, got error: %v", err)
	}
	if int64(len(got)) != cap {
		t.Fatalf("got %d bytes, want %d", len(got), cap)
	}
}

func TestReadAllCapped_OneByteOverCap(t *testing.T) {
	const cap int64 = 32
	payload := bytes.Repeat([]byte("a"), int(cap)+1)
	r := bytes.NewReader(payload)

	_, err := ReadAllCapped(r, cap, "soundpack JSON")
	if err == nil {
		t.Fatalf("input one byte over cap should error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "soundpack JSON") {
		t.Errorf("error %q should name the kind 'soundpack JSON'", msg)
	}
	wantCap := fmt.Sprintf("%d", cap)
	if !strings.Contains(msg, wantCap) {
		t.Errorf("error %q should mention cap value %q", msg, wantCap)
	}
}

// errReader implements io.Reader and always returns a specified error.
type errReader struct {
	err error
}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, e.err
}

func TestReadAllCapped_ReaderError(t *testing.T) {
	sentinel := errors.New("disk on fire")
	r := &errReader{err: sentinel}

	_, err := ReadAllCapped(r, 1024, "audio file")
	if err == nil {
		t.Fatalf("expected error from underlying reader, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("want wrapped sentinel error, got %v", err)
	}
	if !strings.Contains(err.Error(), "audio file") {
		t.Errorf("error %q should name the kind 'audio file'", err.Error())
	}
}

func TestReadAllCapped_ConstantsExposed(t *testing.T) {
	// Sanity check the exported cap constants are reasonable.
	if MaxHookPayloadBytes <= 0 {
		t.Error("MaxHookPayloadBytes must be positive")
	}
	if MaxSoundpackJSONBytes <= 0 {
		t.Error("MaxSoundpackJSONBytes must be positive")
	}
	if MaxAudioFileBytes <= 0 {
		t.Error("MaxAudioFileBytes must be positive")
	}
	if MaxAudioFileBytes <= MaxSoundpackJSONBytes {
		t.Error("audio file cap should exceed soundpack JSON cap (audio data is larger)")
	}
}

// Confirm we still accept io.Reader implementations beyond *bytes.Reader.
var _ io.Reader = (*errReader)(nil)
