package native

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"claudio.click/internal/audio"
)

// The output device is stereo. Sounds with more channels are folded into it
// rather than rejected; malgo played them, so rejecting them is a regression.
func TestPCMReaderDownmixesMultichannelToStereo(t *testing.T) {
	frame := make([]byte, 0, 8)
	for _, v := range []int16{16384, 8192, -8192, 16384} { // 0.5, 0.25, -0.25, 0.5
		frame = binary.LittleEndian.AppendUint16(frame, uint16(v))
	}
	r, err := newPCMReader(context.Background(), &AudioData{Samples: frame, Channels: 4, SampleRate: 48000, Format: FormatS16})
	if err != nil {
		t.Fatalf("4-channel PCM rejected: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 8 {
		t.Fatalf("output bytes=%d, want one stereo frame", len(out))
	}
	for ch := range 2 {
		if got := math.Float32frombits(binary.LittleEndian.Uint32(out[ch*4:])); got != 0.25 {
			t.Errorf("channel %d=%v, want the 0.25 average", ch, got)
		}
	}
}

func TestBackendPlaysSixChannelAIFF(t *testing.T) {
	b, o := testBackend()
	defer b.Close()
	src := audio.NewReaderSource(io.NopCloser(bytes.NewReader(createMinimalAiffFile(44100, 6, 16, 100))), "aiff")
	done := make(chan error, 1)
	go func() { done <- b.Play(context.Background(), src) }()
	waitPlayer(t, o).finish(nil)
	if err := waitPlay(t, done); err != nil {
		t.Fatalf("6-channel AIFF: %v", err)
	}
}
