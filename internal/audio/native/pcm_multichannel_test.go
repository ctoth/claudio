package native

import (
	"bytes"
	"context"
	"io"
	"testing"

	"claudio.click/internal/audio"
	"claudio.click/internal/testutil/wavfixture"
)

// The output device is stereo. Sounds with more channels are folded into it
// rather than rejected; malgo played them, so rejecting them is a regression.
func TestBackendPlaysSixChannelAIFF(t *testing.T) {
	b, o := testBackend()
	defer b.Close()
	data := wavfixture.AIFF("AIFF", 16, 44100, wavfixture.SineFrames(100, 6, 0.1))
	src := audio.NewReaderSource(io.NopCloser(bytes.NewReader(data)), "aiff")
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), src) }()
	waitPlayer(t, o).finish(nil)
	if err := waitPlay(t, done); err != nil {
		t.Fatalf("6-channel AIFF: %v", err)
	}
}
