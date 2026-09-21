package native

import (
	"context"
	"io"
	"sync"

	"github.com/ebitengine/oto/v3"
)

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
				ApplicationName: "Claudio",
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
