package native

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

// outputBufferSize is set explicitly because Oto's per-driver defaults differ
// (100ms PulseAudio, 50ms WASAPI) and Play must know how much to drain.
const outputBufferSize = 100 * time.Millisecond

// drainSize is trailing silence played after each sound. A player stops once
// its source is exhausted, but the device still holds up to twice its target
// buffer (PulseAudio maxlength); the hook process exits right after Play, so
// without the drain the end of every sound is cut off with an audible pop.
const drainSize = int(2*outputBufferSize/time.Millisecond) * outputSampleRate / 1000 * 2 * 4

// silenceReader supplies n bytes of Float32 silence, stopping on cancellation.
type silenceReader struct {
	ctx context.Context
	n   int
}

func (s *silenceReader) Read(dst []byte) (int, error) {
	if err := s.ctx.Err(); err != nil {
		return 0, err
	}
	if s.n == 0 {
		return 0, io.EOF
	}
	n := min(len(dst), s.n)
	clear(dst[:n])
	s.n -= n
	return n, nil
}

type outputPlayer interface {
	Play()
	PauseAndStopReading()
	IsPlaying() bool
	Err() error
	SetVolume(float64)
}

type outputContext interface {
	NewPlayer(io.Reader) outputPlayer
	Err() error
}

type otoOutput struct{ *oto.Context }

func (o otoOutput) NewPlayer(r io.Reader) outputPlayer { return o.Context.NewPlayer(r) }

// Oto permits one context per process, has no context Close, and caches fatal
// driver errors. Backend.Close owns players; the shared device lives until exit.
// Initialization may outlive a cancelled caller, but only one initializer exists.
var device struct {
	once   sync.Once
	done   chan struct{}
	output outputContext
	err    error
}

func openOutput(ctx context.Context) (outputContext, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	device.once.Do(func() {
		device.done = make(chan struct{})
		go func() {
			defer close(device.done)
			c, ready, err := oto.NewContext(&oto.NewContextOptions{
				SampleRate: outputSampleRate, ChannelCount: 2, Format: oto.FormatFloat32LE,
				BufferSize: outputBufferSize, ApplicationName: "Claudio",
			})
			if err != nil {
				device.err = err
				return
			}
			<-ready
			if err = c.Err(); err != nil {
				device.err = err
				return
			}
			device.output = otoOutput{c}
		}()
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-device.done:
		return device.output, device.err
	}
}
