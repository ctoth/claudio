//go:build cgo

package malgo

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/gen2brain/malgo"
)

type startupObserver struct {
	slog.Handler
	ready chan struct{}
	once  sync.Once
}

type playbackStartedObserver struct {
	slog.Handler
	started chan struct{}
}

type playbackOutcome struct {
	err        error
	panicValue any
}

func (h *startupObserver) Enabled(context.Context, slog.Level) bool { return true }
func (h *startupObserver) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "device configuration" {
		h.once.Do(func() { close(h.ready) })
	}
	return nil
}

func (h *playbackStartedObserver) Enabled(context.Context, slog.Level) bool { return true }
func (h *playbackStartedObserver) Handle(ctx context.Context, record slog.Record) error {
	if record.Message == "sound playback started successfully" {
		select {
		case h.started <- struct{}{}:
		default:
		}
	}
	return nil
}

func TestCloseWaitsForAdmittedPlayback(t *testing.T) {
	player := NewAudioPlayer()
	data := &AudioData{Samples: make([]byte, 800), Channels: 1, SampleRate: 8000, Format: malgo.FormatS16}
	if err := player.PreloadSound("close-race", data); err != nil {
		t.Fatal(err)
	}
	observer := &startupObserver{Handler: slog.Default().Handler(), ready: make(chan struct{})}
	previous := slog.Default()
	slog.SetDefault(slog.New(observer))
	defer slog.SetDefault(previous)
	player.deviceInitMutex.Lock()
	playDone := make(chan playbackOutcome, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				playDone <- playbackOutcome{panicValue: recovered}
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		playDone <- playbackOutcome{err: player.PlaySoundWithContext(ctx, "close-race")}
	}()
	select {
	case <-observer.ready:
	case result := <-playDone:
		player.deviceInitMutex.Unlock()
		_ = player.Close()
		if result.panicValue != nil {
			t.Fatalf("playback panicked: %v", result.panicValue)
		}
		skipIfNoAudioDevice(t, result.err)
		t.Fatalf("playback stopped before device startup: %v", result.err)
	case <-time.After(5 * time.Second):
		player.deviceInitMutex.Unlock()
		t.Fatal("playback never reached device startup")
	}
	closed := make(chan struct{})
	go func() { _ = player.Close(); close(closed) }()
	select {
	case <-closed:
		t.Error("Close returned while admitted playback still needed the audio context")
	case <-time.After(50 * time.Millisecond):
	}
	player.deviceInitMutex.Unlock()
	select {
	case result := <-playDone:
		if result.panicValue != nil {
			t.Errorf("playback panicked: %v", result.panicValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("playback did not finish")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not finish")
	}
	if player.IsPlaying() {
		t.Error("playback remained active after Close")
	}
}

func TestPreloadRejectsClosedPlayer(t *testing.T) {
	player := NewAudioPlayer()
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	if err := player.PreloadSound("late", &AudioData{}); err == nil {
		t.Fatal("closed player accepted a new sound")
	}
}

func TestStopAllStopsConcurrentPlaybackWithSameSoundID(t *testing.T) {
	skipIfWSLMalgoPlayback(t)

	player := NewAudioPlayer()
	data := &AudioData{
		Samples:    make([]byte, 30*8000*2),
		Channels:   1,
		SampleRate: 8000,
		Format:     malgo.FormatS16,
	}
	if err := player.PreloadSound("shared-id", data); err != nil {
		t.Fatal(err)
	}

	observer := &playbackStartedObserver{
		Handler: slog.Default().Handler(),
		started: make(chan struct{}, 2),
	}
	previous := slog.Default()
	slog.SetDefault(slog.New(observer))
	defer slog.SetDefault(previous)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	results := make(chan error, 2)
	for range 2 {
		go func() { results <- player.PlaySoundWithContext(ctx, "shared-id") }()
	}

	join := func(got []error, timeout time.Duration) ([]error, bool) {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		for len(got) < 2 {
			select {
			case err := <-results:
				got = append(got, err)
			case <-timer.C:
				return got, false
			}
		}
		return got, true
	}
	cleanup := func(got []error) ([]error, bool) {
		cancel()
		_ = player.StopAll()
		got, joined := join(got, 5*time.Second)
		_ = player.Close()
		return got, joined
	}

	started := 0
	var earlyResults []error
	startupDeadline := time.NewTimer(10 * time.Second)
	for started < 2 && len(earlyResults) == 0 {
		select {
		case <-observer.started:
			started++
		case err := <-results:
			earlyResults = append(earlyResults, err)
		case <-startupDeadline.C:
			got, joined := cleanup(earlyResults)
			if !joined {
				t.Fatal("playback goroutines did not finish during timeout cleanup")
			}
			t.Fatalf("only %d playback calls started before timeout; results=%v", started, got)
		}
	}
	if !startupDeadline.Stop() {
		select {
		case <-startupDeadline.C:
		default:
		}
	}

	if len(earlyResults) != 0 {
		got, joined := cleanup(earlyResults)
		if !joined {
			t.Fatal("playback goroutines did not finish after startup failure")
		}
		allNoAudio := len(got) == 2
		for _, err := range got {
			allNoAudio = allNoAudio && isNoAudioDeviceError(err)
		}
		if allNoAudio {
			t.Skip("no audio device available")
		}
		t.Fatalf("playback returned before both instances started: started=%d results=%v", started, got)
	}

	if err := player.StopAll(); err != nil {
		got, _ := cleanup(nil)
		t.Fatalf("StopAll: %v; playback results=%v", err, got)
	}
	got, joinedPromptly := join(nil, 3*time.Second)
	if !joinedPromptly {
		got, cleaned := cleanup(got)
		if !cleaned {
			t.Fatal("playback goroutines did not finish during cleanup")
		}
		t.Fatalf("StopAll left same-ID playback running; results=%v", got)
	}
	cancel()
	if err := player.Close(); err != nil {
		t.Fatal(err)
	}
	for _, err := range got {
		if err != nil {
			t.Fatalf("playback returned an error after StopAll: %v", err)
		}
	}
}
