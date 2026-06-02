//go:build ignore

// Command generate produces the default procedural WAV sounds that ship
// embedded in the binary as the native-Linux default soundpack.
//
// These are synthesized tones (no licensing, tiny on disk) rather than
// system sounds, because a bare Linux box has no guaranteed set of WAV
// files the way Windows (C:\Windows\Media) and macOS (/System/Library/
// Sounds) do. Run from this directory to regenerate:
//
//	go run generate.go
//
// Output: default-success.wav, default-error.wav, default-loading.wav,
// default-interactive.wav, default.wav — 16-bit mono PCM at 44.1 kHz.
package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const sampleRate = 44100

// tone is one partial: a frequency held for the whole sound, summed with
// the others and shaped by a shared attack/release envelope.
type tone struct {
	freqs []float64 // summed partials (Hz)
	dur   float64   // seconds
	amp   float64   // peak amplitude before envelope (0..1)
}

// glide is a sound whose single partial sweeps linearly from start to end
// frequency over its duration (used for the rising/falling cues).
type glide struct {
	start, end float64
	dur        float64
	amp        float64
}

func envelope(t, dur float64) float64 {
	// Short raised-cosine attack and release so there are no clicks.
	const edge = 0.012 // seconds
	switch {
	case t < edge:
		return 0.5 * (1 - math.Cos(math.Pi*t/edge))
	case t > dur-edge:
		return 0.5 * (1 - math.Cos(math.Pi*(dur-t)/edge))
	default:
		return 1
	}
}

func renderTone(s tone) []int16 {
	n := int(float64(sampleRate) * s.dur)
	out := make([]int16, n)
	for i := range out {
		t := float64(i) / sampleRate
		var v float64
		for _, f := range s.freqs {
			v += math.Sin(2 * math.Pi * f * t)
		}
		v /= float64(len(s.freqs))
		v *= s.amp * envelope(t, s.dur)
		out[i] = int16(v * math.MaxInt16)
	}
	return out
}

func renderGlide(g glide) []int16 {
	n := int(float64(sampleRate) * g.dur)
	out := make([]int16, n)
	phase := 0.0
	for i := range out {
		t := float64(i) / sampleRate
		frac := t / g.dur
		f := g.start + (g.end-g.start)*frac
		phase += 2 * math.Pi * f / sampleRate
		v := math.Sin(phase) * g.amp * envelope(t, g.dur)
		out[i] = int16(v * math.MaxInt16)
	}
	return out
}

func writeWAV(name string, samples []int16) error {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataLen := len(samples) * 2

	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()

	le := binary.LittleEndian
	w := func(order any) { _ = binary.Write(f, le, order) }

	f.WriteString("RIFF")
	w(uint32(36 + dataLen))
	f.WriteString("WAVE")
	f.WriteString("fmt ")
	w(uint32(16))                 // PCM fmt chunk size
	w(uint16(1))                  // audio format = PCM
	w(uint16(numChannels))        //
	w(uint32(sampleRate))         //
	w(uint32(byteRate))           //
	w(uint16(blockAlign))         //
	w(uint16(bitsPerSample))      //
	f.WriteString("data")
	w(uint32(dataLen))
	for _, s := range samples {
		w(s)
	}
	return nil
}

func main() {
	files := map[string][]int16{
		// Rising two-tone chime — C5 then E5/G5 implied by partials.
		"default-success.wav": renderGlide(glide{start: 523.25, end: 783.99, dur: 0.28, amp: 0.35}),
		// Descending low buzz — A3 + a slightly detuned partial.
		"default-error.wav": renderTone(tone{freqs: []float64{220.0, 233.08}, dur: 0.34, amp: 0.32}),
		// Soft mid pulse for loading/in-progress.
		"default-loading.wav": renderTone(tone{freqs: []float64{440.0}, dur: 0.16, amp: 0.28}),
		// Single clean high tone for interactive prompts.
		"default-interactive.wav": renderTone(tone{freqs: []float64{659.25}, dur: 0.20, amp: 0.32}),
		// Neutral default beep.
		"default.wav": renderTone(tone{freqs: []float64{587.33}, dur: 0.16, amp: 0.30}),
	}
	for name, samples := range files {
		if err := writeWAV(name, samples); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d samples)\n", name, len(samples))
	}
}
