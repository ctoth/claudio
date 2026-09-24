package native

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"claudio.click/internal/audio"
	"claudio.click/internal/volume"
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
// Close cancels the current set and rejects future admissions. It does not
// own the process-wide Oto context, so closing one backend cannot stop another.
type Backend struct {
	mu         sync.Mutex
	closed     bool
	volume     float32
	plays      map[*playback]struct{}
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
		volume: 1, plays: make(map[*playback]struct{}), openOutput: openOutput,
		startTimeout: defaultStartTimeout, stallGrace: defaultStallGrace,
	}
}

func (b *Backend) Close() error {
	b.mu.Lock()
	b.closed = true
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

func (b *Backend) SetVolume(v float32) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return audio.ErrBackendClosed
	}
	if err := volume.Validate(float64(v)); err != nil {
		return err
	}
	b.volume = v
	for p := range b.plays {
		if p.player != nil {
			p.player.SetVolume(float64(v))
		}
	}
	return nil
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
		} // Close is a successful stop.
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
	snd, err := decodeSound(playCtx, filename, reader)
	if err != nil {
		return fmt.Errorf("decode audio: %w", err)
	}
	pcm := newPCMReader(playCtx, snd)
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

	// Admission and start share the close lock. A cancelled admission never
	// starts a player after Close has taken its snapshot.
	b.mu.Lock()
	if err = playCtx.Err(); err != nil {
		b.mu.Unlock()
		return err
	}
	p.player = output.NewPlayer(io.MultiReader(pcm, &silenceReader{ctx: playCtx, n: drainSize}))
	p.player.SetVolume(float64(b.volume))
	p.player.Play()
	b.mu.Unlock()
	slog.Debug("Oto playback started", "sample_rate", int(snd.rate), "frames", snd.frames)

	limit := b.playbackDeadline(snd)
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
func (b *Backend) playbackDeadline(s sound) time.Duration {
	return s.duration() + 2*outputBufferSize + b.stallGrace
}
