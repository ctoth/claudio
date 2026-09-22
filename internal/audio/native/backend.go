package native

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"sync"
	"time"

	"claudio.click/internal/audio"
)

func init() {
	audio.RegisterBackend("oto", func() (audio.AudioBackend, error) { return NewBackend(), nil })
}

// ErrPlaybackStalled reports a device that stopped consuming audio, or never
// started, without reporting an error. Oto keeps such a player "playing"
// forever, and the hook caller has no deadline of its own.
var ErrPlaybackStalled = errors.New("audio device stalled")

const (
	defaultStartTimeout = 5 * time.Second
	defaultStallGrace   = 2 * time.Second
)

// Backend owns each admitted playback from decode through player shutdown.
// Stop cancels the current set; Close also rejects future admissions. Neither
// owns the process-wide Oto context, so closing one backend cannot stop another.
type Backend struct {
	mu         sync.Mutex
	closed     bool
	volume     float32
	plays      map[*playback]struct{}
	registry   *DecoderRegistry
	openOutput func(context.Context) (outputContext, error)

	// startTimeout bounds device initialization; stallGrace is how long
	// playback may overrun the sound plus its drain before it is abandoned.
	startTimeout, stallGrace time.Duration
}

type playback struct {
	cancel context.CancelFunc
	done   chan struct{}
	player outputPlayer
}

func NewBackend() *Backend {
	return &Backend{
		volume: 1, plays: make(map[*playback]struct{}), registry: NewDefaultRegistry(), openOutput: openOutput,
		startTimeout: defaultStartTimeout, stallGrace: defaultStallGrace,
	}
}

func (b *Backend) Stop() error  { return b.stop(false) }
func (b *Backend) Close() error { return b.stop(true) }

func (b *Backend) stop(closeBackend bool) error {
	b.mu.Lock()
	if b.closed && !closeBackend {
		b.mu.Unlock()
		return audio.ErrBackendClosed
	}
	if closeBackend {
		b.closed = true
	}
	active := make([]*playback, 0, len(b.plays))
	for p := range b.plays {
		p.cancel()
		active = append(active, p)
	}
	b.mu.Unlock()
	for _, p := range active {
		<-p.done
	}
	return nil
}

func (b *Backend) IsPlaying() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for p := range b.plays {
		if p.player != nil && p.player.IsPlaying() {
			return true
		}
	}
	return false
}

func (b *Backend) SetVolume(volume float32) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return audio.ErrBackendClosed
	}
	if math.IsNaN(float64(volume)) || math.IsInf(float64(volume), 0) || volume < 0 || volume > 1 {
		return fmt.Errorf("invalid volume level: %v (must be finite and between 0 and 1)", volume)
	}
	b.volume = volume
	for p := range b.plays {
		if p.player != nil {
			p.player.SetVolume(float64(volume))
		}
	}
	return nil
}

func (b *Backend) GetVolume() float32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return 0
	}
	return b.volume
}

func (b *Backend) Play(ctx context.Context, source audio.AudioSource) (err error) {
	if err = ctx.Err(); err != nil {
		return err
	}
	playCtx, cancel := context.WithCancel(ctx)
	p := &playback{cancel: cancel, done: make(chan struct{})}
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		cancel()
		return audio.ErrBackendClosed
	}
	b.plays[p] = struct{}{}
	b.mu.Unlock()
	defer func() {
		// Pause alone can leave a read in flight; Close on Oto v3 is a no-op.
		if p.player != nil {
			p.player.PauseAndStopReading()
		}
		if ctx.Err() != nil {
			err = ctx.Err()
		} else if playCtx.Err() != nil {
			err = nil
		} // Stop/Close are successful stops.
		cancel()
		b.mu.Lock()
		delete(b.plays, p)
		close(p.done)
		b.mu.Unlock()
	}()

	reader, format, err := source.Reader()
	if err != nil {
		return fmt.Errorf("get audio source: %w", err)
	}
	defer reader.Close()
	filename := "stream." + format
	if file, ok := source.(audio.FilePather); ok {
		if path, pathErr := file.FilePath(); pathErr == nil {
			filename = path
		}
	}
	data, err := b.registry.DecodeFile(playCtx, filename, reader)
	if err != nil {
		return fmt.Errorf("decode audio: %w", err)
	}
	pcm, err := newPCMReader(playCtx, data)
	if err != nil {
		return err
	}
	startCtx, cancelStart := context.WithTimeout(playCtx, b.startTimeout)
	output, err := b.openOutput(startCtx)
	cancelStart()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && playCtx.Err() == nil {
			slog.Warn("Oto output did not start", "timeout", b.startTimeout)
			return fmt.Errorf("initialize Oto output: %w after %v", ErrPlaybackStalled, b.startTimeout)
		}
		return fmt.Errorf("initialize Oto output: %w", err)
	}
	if err = output.Err(); err != nil {
		return fmt.Errorf("oto output: %w", err)
	}

	// Admission and start share the stop lock. A cancelled admission never
	// starts a player after Stop has taken its snapshot.
	b.mu.Lock()
	if err = playCtx.Err(); err != nil {
		b.mu.Unlock()
		return err
	}
	p.player = output.NewPlayer(io.MultiReader(pcm, &silenceReader{ctx: playCtx, n: drainSize}))
	p.player.SetVolume(float64(b.volume))
	p.player.Play()
	b.mu.Unlock()
	slog.Debug("Oto playback started", "sample_rate", data.SampleRate, "channels", data.Channels)

	limit := b.playbackDeadline(data)
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		// Observe completion before checking errors: a reader can fail while
		// transitioning to stopped, and that failure must not become success.
		playing := p.player.IsPlaying()
		if err = p.player.Err(); err != nil {
			return fmt.Errorf("oto player: %w", err)
		}
		if err = output.Err(); err != nil {
			return fmt.Errorf("oto output: %w", err)
		}
		if !playing {
			return nil
		}
		select {
		case <-playCtx.Done():
			return playCtx.Err()
		case <-deadline.C:
			slog.Warn("Oto playback stalled; abandoning player", "limit", limit)
			return fmt.Errorf("oto playback: %w after %v", ErrPlaybackStalled, limit)
		case <-ticker.C:
		}
	}
}

// playbackDeadline is the longest a healthy device needs: the sound, the
// trailing silence drain, and a grace period for scheduling jitter.
// data has already been validated by newPCMReader.
func (b *Backend) playbackDeadline(data *AudioData) time.Duration {
	width, _ := getBytesPerSample(data.Format)
	frames := len(data.Samples) / (width * int(data.Channels))
	sound := time.Duration(frames) * time.Second / time.Duration(data.SampleRate)
	return sound + 2*outputBufferSize + b.stallGrace
}
