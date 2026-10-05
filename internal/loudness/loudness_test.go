package loudness

import (
	"math"
	"testing"
)

// sine returns seconds of a stereo sine at freq Hz with the given peak
// amplitude in dBFS.
func sine(rate int, freq, peakDB, seconds float64) [][2]float64 {
	amp := math.Pow(10, peakDB/20)
	frames := make([][2]float64, int(float64(rate)*seconds))
	for i := range frames {
		x := amp * math.Sin(2*math.Pi*freq*float64(i)/float64(rate))
		frames[i] = [2]float64{x, x}
	}
	return frames
}

func silence(rate int, seconds float64) [][2]float64 {
	return make([][2]float64, int(float64(rate)*seconds))
}

func near(t *testing.T, name string, got, want, tol float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > tol {
		t.Errorf("%s = %.3f, want %.3f ± %.2f", name, got, want, tol)
	}
}

// EBU Tech 3341 test case 1: a stereo 1 kHz sine at -23 dBFS reads -23 LUFS.
func TestIntegratedMatchesEBUReferenceTone(t *testing.T) {
	for _, rate := range []int{44100, 48000, 22050} {
		got := Measure(sine(rate, 1000, -23, 20), rate)
		near(t, "integrated", got.IntegratedLUFS, -23.0, 0.1)
		near(t, "loudness", got.LoudnessLUFS, -23.0, 0.1)
		near(t, "momentary max", got.MomentaryMaxLUFS, -23.0, 0.1)
	}
}

// EBU Tech 3341 test case 3 shape: the relative gate drops quiet stretches,
// so a loud tone between two quiet ones reads as the loud tone.
func TestIntegratedGatesQuietSections(t *testing.T) {
	const rate = 48000
	var frames [][2]float64
	frames = append(frames, sine(rate, 1000, -36, 10)...)
	frames = append(frames, sine(rate, 1000, -23, 60)...)
	frames = append(frames, sine(rate, 1000, -36, 10)...)
	near(t, "integrated", Measure(frames, rate).IntegratedLUFS, -23.0, 0.1)
}

// Gated loudness needs a 400 ms block. Shorter sounds are common in UI
// packs, so they fall back to ungated loudness over the whole sound instead
// of reading as silence.
func TestShortSoundUsesUngatedLoudness(t *testing.T) {
	const rate = 48000
	got := Measure(sine(rate, 1000, -23, 0.150), rate)
	if !math.IsInf(got.IntegratedLUFS, -1) {
		t.Errorf("integrated = %v, want -Inf for a sound under 400 ms", got.IntegratedLUFS)
	}
	near(t, "loudness", got.LoudnessLUFS, -23.0, 0.3)
}

func TestSilenceIsMinusInfinity(t *testing.T) {
	got := Measure(silence(44100, 1), 44100)
	if !math.IsInf(got.LoudnessLUFS, -1) || !math.IsInf(got.PeakDBTP, -1) {
		t.Errorf("silence measured as %+v", got)
	}
}

func TestSamplePeak(t *testing.T) {
	got := Measure(sine(48000, 1000, -6, 1), 48000)
	near(t, "sample peak", got.SamplePeakDBFS, -6.0, 0.05)
	near(t, "true peak", got.PeakDBTP, -6.0, 0.1)
}

// A sine at a quarter of the sample rate, phase-shifted 45 degrees, has
// samples at 0.707 of its real peak. True peak must see the real one.
func TestTruePeakSeesIntersamplePeaks(t *testing.T) {
	const rate = 48000
	frames := make([][2]float64, rate)
	for i := range frames {
		x := math.Sin(2*math.Pi*float64(i)/4 + math.Pi/4)
		frames[i] = [2]float64{x, x}
	}
	got := Measure(frames, rate)
	near(t, "sample peak", got.SamplePeakDBFS, -3.01, 0.05)
	near(t, "true peak", got.PeakDBTP, 0.0, 0.3)
}

func TestSilenceBounds(t *testing.T) {
	const rate = 48000
	var frames [][2]float64
	frames = append(frames, silence(rate, 0.050)...)
	frames = append(frames, sine(rate, 1000, -12, 0.300)...)
	frames = append(frames, silence(rate, 0.200)...)

	start, end := ActiveRange(frames, -60)
	near(t, "start ms", float64(start)*1000/rate, 50, 1)
	near(t, "end ms", float64(end)*1000/rate, 350, 1)

	start, end = ActiveRange(silence(rate, 1), -60)
	if start != end {
		t.Errorf("all-silent range = [%d,%d), want empty", start, end)
	}
}

func TestClippedSamples(t *testing.T) {
	const rate = 48000
	frames := sine(rate, 100, -1, 1)
	if n := ClippedRuns(frames); n != 0 {
		t.Errorf("clean sine has %d clipped runs", n)
	}
	for i := range frames {
		for ch := range frames[i] {
			frames[i][ch] = math.Max(-1, math.Min(1, frames[i][ch]*2))
		}
	}
	if n := ClippedRuns(frames); n < 100 {
		t.Errorf("hard-clipped sine has %d clipped runs, want about 200", n)
	}
}

func TestGainToTarget(t *testing.T) {
	// Plenty of headroom: full correction.
	near(t, "gain", GainDB(Measurement{LoudnessLUFS: -30, PeakDBTP: -20}, -18, -1), 12, 0.001)
	// Peak-limited: only up to the ceiling.
	near(t, "gain", GainDB(Measurement{LoudnessLUFS: -30, PeakDBTP: -5}, -18, -1), 4, 0.001)
	// Too loud: attenuate, and further if the peak still exceeds the ceiling.
	near(t, "gain", GainDB(Measurement{LoudnessLUFS: -10, PeakDBTP: -0.5}, -18, -1), -8, 0.001)
	near(t, "gain", GainDB(Measurement{LoudnessLUFS: -17, PeakDBTP: 0.5}, -18, -1), -1.5, 0.001)
}
