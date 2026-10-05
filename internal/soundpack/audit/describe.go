package audit

import (
	"math"
	"math/cmplx"
	"sort"
)

// Description is what a sound is like, in numbers: enough for a reader who
// cannot hear it to tell a single low thud from three rising beeps, and to
// check that a pack's errors do not sound like its successes.
type Description struct {
	// Onsets counts separate bursts: 1 for a single tone, 3 for a triple beep.
	Onsets int `json:"onsets"`
	// AttackMS is how long the sound takes to reach 90% of its loudest point.
	AttackMS float64 `json:"attack_ms"`
	// DominantHz is the strongest frequency; CentroidHz is the spectrum's
	// centre of mass, which tracks perceived brightness.
	DominantHz float64 `json:"dominant_hz"`
	CentroidHz float64 `json:"centroid_hz"`
	// PitchTrend compares the brightness of the first and second halves:
	// "rising", "falling" or "steady".
	PitchTrend string `json:"pitch_trend"`
	// Tonality is how much of the sound's energy, moment by moment, sits in
	// a few spectral peaks: near 1 for tones and chords, near 0 for noise
	// (filtered or not). Texture names it: "tonal", "mixed" or "noisy".
	Tonality float64 `json:"tonality"`
	Texture  string  `json:"texture"`
	// Flatness is spectral flatness of the whole sound from 0 (pure tone)
	// to 1 (white noise). Unlike Tonality it reads low for any band-limited
	// noise, so it measures how full the spectrum is, not whether there is
	// a pitch.
	Flatness float64 `json:"flatness"`
}

const (
	fftSize        = 2048
	envelopeHopSec = 0.005
)

// Describe characterizes frames, which should already be trimmed to the
// audible span of the sound.
func Describe(frames [][2]float64, rate int) Description {
	if len(frames) == 0 || rate <= 0 {
		return Description{}
	}
	mono := make([]float64, len(frames))
	for i, f := range frames {
		mono[i] = (f[0] + f[1]) / 2
	}

	d := Description{}
	d.Onsets, d.AttackMS = envelopeShape(mono, rate)

	whole, tonality := spectrum(mono, rate)
	d.DominantHz, d.CentroidHz, d.Flatness = spectrumStats(whole, rate)
	d.DominantHz, d.CentroidHz = math.Round(d.DominantHz), math.Round(d.CentroidHz)
	d.Flatness = math.Round(d.Flatness*1000) / 1000
	d.Tonality = math.Round(tonality*1000) / 1000
	// Measured: tones, chords and FM bells read above 0.9, noise through a
	// narrow filter 0.5 to 0.6, white noise about 0.1.
	switch {
	case d.Tonality >= 0.8:
		d.Texture = "tonal"
	case d.Tonality >= 0.65:
		d.Texture = "mixed"
	default:
		d.Texture = "noisy"
	}

	half := len(mono) / 2
	firstHalf, _ := spectrum(mono[:half], rate)
	secondHalf, _ := spectrum(mono[half:], rate)
	_, first, _ := spectrumStats(firstHalf, rate)
	_, second, _ := spectrumStats(secondHalf, rate)
	d.PitchTrend = "steady"
	if first > 0 && second > 0 {
		switch ratio := second / first; {
		case ratio > 1.15:
			d.PitchTrend = "rising"
		case ratio < 1/1.15:
			d.PitchTrend = "falling"
		}
	}
	return d
}

// envelopeShape follows the RMS envelope in 5 ms hops. An onset is counted
// each time the envelope climbs past a quarter of its peak after having
// dropped below a tenth of it, so a decaying tail is one onset and a gap
// between beeps starts a new one.
func envelopeShape(mono []float64, rate int) (onsets int, attackMS float64) {
	hop := max(1, int(envelopeHopSec*float64(rate)))
	var env []float64
	for i := 0; i < len(mono); i += hop {
		chunk := mono[i:min(i+hop, len(mono))]
		var e float64
		for _, x := range chunk {
			e += x * x
		}
		env = append(env, math.Sqrt(e/float64(len(chunk))))
	}
	var peak float64
	for _, e := range env {
		peak = math.Max(peak, e)
	}
	if peak == 0 {
		return 0, 0
	}

	attackMS = -1
	high := false
	for i, e := range env {
		if attackMS < 0 && e >= 0.9*peak {
			attackMS = float64(i*hop) * 1000 / float64(rate)
		}
		switch {
		case !high && e > 0.25*peak:
			high = true
			onsets++
		case high && e < 0.1*peak:
			high = false
		}
	}
	return onsets, math.Max(attackMS, 0)
}

// spectrum returns the power spectrum of mono averaged over half-overlapped
// Hann windows (bins 0..fftSize/2). A sound shorter than one window is
// zero-padded.
//
// tonality is the energy-weighted mean, over the windows, of the share of
// each window's power held by its strongest bins. A tone or chord puts
// nearly all of a window's power in a few bins; noise, however it is
// filtered, spreads it over the whole band it occupies.
func spectrum(mono []float64, rate int) (power []float64, tonality float64) {
	power = make([]float64, fftSize/2+1)
	if len(mono) == 0 {
		return power, 0
	}
	lo, hi := analysisBand(rate)
	// A Hann-windowed tone fills 4 bins, so 12 holds a three-note chord.
	const peakBins = 12

	buf := make([]complex128, fftSize)
	frame := make([]float64, 0, fftSize/2+1)
	var weighted, total float64
	for start := 0; ; start += fftSize / 2 {
		n := min(fftSize, len(mono)-start)
		for i := range buf {
			buf[i] = 0
			if i < n {
				w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n))
				buf[i] = complex(mono[start+i]*w, 0)
			}
		}
		fft(buf)
		frame = frame[:0]
		var energy float64
		for i := range power {
			a := cmplx.Abs(buf[i])
			power[i] += a * a
			if i >= lo && i <= hi {
				frame = append(frame, a*a)
				energy += a * a
			}
		}
		if energy > 0 {
			// A window shorter than the transform is zero-padded, which
			// smears each peak over proportionally more bins.
			peaks := min(peakBins*((fftSize+n-1)/n), len(frame))
			sort.Float64s(frame)
			var top float64
			for _, p := range frame[len(frame)-peaks:] {
				top += p
			}
			weighted += top
			total += energy
		}
		if start+fftSize >= len(mono) {
			break
		}
	}
	if total > 0 {
		tonality = weighted / total
	}
	return power, tonality
}

// analysisBand is the range of bins the description looks at, 50 Hz to
// 12 kHz: below is rumble and DC, above carries little that identifies a UI
// sound.
func analysisBand(rate int) (lo, hi int) {
	binHz := float64(rate) / fftSize
	return max(1, int(math.Ceil(50/binHz))), min(fftSize/2, int(12000/binHz))
}

// spectrumStats reads the dominant frequency, centroid and flatness from a
// power spectrum, within the analysis band.
func spectrumStats(power []float64, rate int) (dominant, centroid, flatness float64) {
	binHz := float64(rate) / fftSize
	lo, hi := analysisBand(rate)
	if hi <= lo {
		return 0, 0, 0
	}
	var total, weighted, logSum, best float64
	bestBin := lo
	for i := lo; i <= hi; i++ {
		p := power[i]
		total += p
		weighted += p * float64(i) * binHz
		logSum += math.Log(p + 1e-20)
		if p > best {
			best, bestBin = p, i
		}
	}
	if total == 0 {
		return 0, 0, 0
	}
	n := float64(hi - lo + 1)
	return float64(bestBin) * binHz, weighted / total, math.Exp(logSum/n) / (total / n)
}

// fft is an in-place radix-2 transform; len(a) must be a power of two.
func fft(a []complex128) {
	n := len(a)
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			a[i], a[j] = a[j], a[i]
		}
	}
	for size := 2; size <= n; size <<= 1 {
		step := cmplx.Rect(1, -2*math.Pi/float64(size))
		for start := 0; start < n; start += size {
			w := complex(1, 0)
			for k := range size / 2 {
				u, v := a[start+k], a[start+k+size/2]*w
				a[start+k], a[start+k+size/2] = u+v, u-v
				w *= step
			}
		}
	}
}
