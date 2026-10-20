package synth

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/gopxl/beep/v2"

	"claudio.click/internal/audio/native"
	"claudio.click/internal/safeio"
)

// LoadSamples reads the recordings the recipe's sample layers use, from
// paths relative to dir, and cuts each layer's slice. It must run before
// Render for a recipe that has sample layers.
func (r *Recipe) LoadSamples(dir string) error {
	decoded := make(map[string]recording)
	for _, name := range r.SoundNames() {
		s := r.Sounds[name]
		for i := range s.Layers {
			l := &s.Layers[i]
			if l.Sample == "" {
				continue
			}
			if err := l.load(dir, decoded); err != nil {
				return fmt.Errorf("%s, layer %d: %w", name, i+1, err)
			}
		}
		if d := s.drySeconds(); d > maxSoundSeconds {
			return fmt.Errorf("%s: %.2f s is longer than the %.0f s limit", name, d, maxSoundSeconds)
		}
	}
	return nil
}

type recording struct {
	frames [][2]float64
	rate   int
}

func (l *Layer) load(dir string, decoded map[string]recording) error {
	rec, ok := decoded[l.Sample]
	if !ok {
		path := filepath.Join(dir, filepath.FromSlash(l.Sample))
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("sample %s: %w", l.Sample, err)
		}
		if info.Size() > safeio.MaxAudioFileBytes {
			return fmt.Errorf("sample %s: larger than %d bytes", l.Sample, int64(safeio.MaxAudioFileBytes))
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("sample %s: %w", l.Sample, err)
		}
		frames, rate, err := native.DecodeFrames(context.Background(), path, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("sample %s: %w", l.Sample, err)
		}
		rec = recording{frames, rate}
		decoded[l.Sample] = rec
	}

	length := float64(len(rec.frames)) / float64(rec.rate)
	if l.Start >= length {
		return fmt.Errorf("start: %.3f s is past the end of %s (%.3f s long)", l.Start, l.Sample, length)
	}
	from := int(l.Start * float64(rec.rate))
	to := len(rec.frames)
	if l.End != 0 {
		to = min(to, int(l.End*float64(rec.rate)))
	}
	slice := append([][2]float64(nil), rec.frames[from:to]...)
	if l.Reverse {
		for i, j := 0, len(slice)-1; i < j; i, j = i+1, j-1 {
			slice[i], slice[j] = slice[j], slice[i]
		}
	}

	speed := l.Speed
	if speed == 0 {
		speed = 1
	}
	// Playing faster is the same as the file having a higher sample rate.
	if from := int(math.Round(float64(rec.rate) * speed)); from != SampleRate {
		slice = resample(slice, from)
	}
	if len(slice) == 0 {
		return errors.New("start, end: the slice is empty")
	}
	l.frames, l.loaded = slice, true

	if l.Dur == 0 {
		// Play the whole slice: the release takes its last moments.
		total := float64(len(slice)) / SampleRate
		release := math.Min(l.release(), total/2)
		l.Release = &release
		l.Dur = total - release
	}
	return nil
}

type sliceStreamer struct {
	frames [][2]float64
	pos    int
}

func (s *sliceStreamer) Stream(dst [][2]float64) (int, bool) {
	n := copy(dst, s.frames[s.pos:])
	s.pos += n
	return n, n > 0
}

func (s *sliceStreamer) Err() error { return nil }

func resample(frames [][2]float64, from int) [][2]float64 {
	stream := beep.Resample(4, beep.SampleRate(from), beep.SampleRate(SampleRate), &sliceStreamer{frames: frames})
	out := make([][2]float64, 0, len(frames)*SampleRate/from+1)
	buf := make([][2]float64, 512)
	for {
		n, ok := stream.Stream(buf)
		out = append(out, buf[:n]...)
		if !ok || n == 0 {
			return out
		}
	}
}

// sampleNote plays the layer's slice through its envelope and pan.
func (l *Layer) sampleNote() [][2]float64 {
	out := make([][2]float64, frameCount(l.Dur+l.release()))
	left, right := math.Min(1, 1-l.Pan), math.Min(1, 1+l.Pan)
	for i := range out {
		if i >= len(l.frames) {
			break
		}
		t := float64(i) / SampleRate
		level := l.envelope(t)
		if l.Tremolo != nil {
			level *= 1 - l.Tremolo.Depth*(0.5+0.5*math.Sin(2*math.Pi*l.Tremolo.Rate*t))
		}
		out[i][0] = l.frames[i][0] * level * left
		out[i][1] = l.frames[i][1] * level * right
	}
	return out
}
