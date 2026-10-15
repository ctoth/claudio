// Package loudness measures decoded audio the way ITU-R BS.1770 / EBU R 128
// do: K-weighted loudness in LUFS and true peak in dBTP. It also finds the
// non-silent span of a sound and counts clipped runs. Everything works on
// stereo float frames, the form the native decoders produce, so a mono file
// (duplicated to both channels) measures as loud as it plays.
package loudness

import "math"

const (
	blockSeconds = 0.4 // BS.1770 gating block
	stepSeconds  = 0.1 // 75% overlap
	absoluteGate = -70.0
	relativeGate = -10.0
)

// Measurement is the loudness and peak of one sound. A level of silence is
// negative infinity.
type Measurement struct {
	// IntegratedLUFS is BS.1770 gated programme loudness. It is negative
	// infinity for sounds shorter than one 400 ms gating block.
	IntegratedLUFS float64
	// LoudnessLUFS is the level to normalize by: IntegratedLUFS when it is
	// defined, otherwise ungated K-weighted loudness over the whole sound.
	LoudnessLUFS float64
	// MomentaryMaxLUFS is the loudest 400 ms block (the whole sound when it
	// is shorter than that).
	MomentaryMaxLUFS float64
	// PeakDBTP is the true peak, estimated by 4x oversampling.
	PeakDBTP float64
	// SamplePeakDBFS is the largest sample value.
	SamplePeakDBFS float64
}

// Measure computes loudness and peak for frames at the given sample rate.
func Measure(frames [][2]float64, rate int) Measurement {
	m := Measurement{
		IntegratedLUFS:   math.Inf(-1),
		LoudnessLUFS:     math.Inf(-1),
		MomentaryMaxLUFS: math.Inf(-1),
		PeakDBTP:         math.Inf(-1),
		SamplePeakDBFS:   math.Inf(-1),
	}
	if len(frames) == 0 || rate <= 0 {
		return m
	}

	samplePeak := 0.0
	for _, f := range frames {
		samplePeak = math.Max(samplePeak, math.Max(math.Abs(f[0]), math.Abs(f[1])))
	}
	m.SamplePeakDBFS = toDB(samplePeak)
	m.PeakDBTP = toDB(truePeak(frames, samplePeak))

	// Channel-summed K-weighted energy per 100 ms step.
	step := max(1, int(math.Round(stepSeconds*float64(rate))))
	stepEnergy := make([]float64, 0, len(frames)/step+1)
	var filters [2]kWeighting
	for ch := range filters {
		filters[ch] = newKWeighting(float64(rate))
	}
	var total, acc float64
	for i, f := range frames {
		for ch := range filters {
			y := filters[ch].process(f[ch])
			acc += y * y
		}
		if (i+1)%step == 0 {
			stepEnergy = append(stepEnergy, acc)
			total += acc
			acc = 0
		}
	}
	total += acc
	ungated := energyToLUFS(total / float64(len(frames)))

	const stepsPerBlock = int(blockSeconds / stepSeconds)
	if len(stepEnergy) < stepsPerBlock {
		m.LoudnessLUFS = ungated
		m.MomentaryMaxLUFS = ungated
		return m
	}

	blocks := make([]float64, 0, len(stepEnergy)-stepsPerBlock+1)
	blockFrames := float64(step * stepsPerBlock)
	for i := 0; i+stepsPerBlock <= len(stepEnergy); i++ {
		var e float64
		for _, s := range stepEnergy[i : i+stepsPerBlock] {
			e += s
		}
		blocks = append(blocks, e/blockFrames)
	}
	for _, b := range blocks {
		m.MomentaryMaxLUFS = math.Max(m.MomentaryMaxLUFS, energyToLUFS(b))
	}

	gate := gatedMean(blocks, absoluteGate)
	m.IntegratedLUFS = energyToLUFS(gatedMean(blocks, math.Max(absoluteGate, energyToLUFS(gate)+relativeGate)))
	m.LoudnessLUFS = m.IntegratedLUFS
	return m
}

// GainDB returns the gain that brings m to targetLUFS without pushing its
// true peak above ceilingDBTP. A sound with little headroom is raised only
// as far as the ceiling allows, so it can end up below the target.
func GainDB(m Measurement, targetLUFS, ceilingDBTP float64) float64 {
	if math.IsInf(m.LoudnessLUFS, -1) {
		return 0
	}
	return math.Min(targetLUFS-m.LoudnessLUFS, ceilingDBTP-m.PeakDBTP)
}

// ActiveRange returns the half-open frame range [start, end) from the first
// to the last sample louder than thresholdDB (dBFS). start == end when the
// whole sound is below the threshold.
func ActiveRange(frames [][2]float64, thresholdDB float64) (start, end int) {
	threshold := math.Pow(10, thresholdDB/20)
	loud := func(f [2]float64) bool {
		return math.Abs(f[0]) > threshold || math.Abs(f[1]) > threshold
	}
	for start < len(frames) && !loud(frames[start]) {
		start++
	}
	end = len(frames)
	for end > start && !loud(frames[end-1]) {
		end--
	}
	return start, end
}

// ClippedRuns counts runs of three or more consecutive full-scale samples,
// per channel. A clean recording has none; a hard-clipped one has a run on
// every flattened peak.
func ClippedRuns(frames [][2]float64) int {
	const fullScale = 0.999
	runs := 0
	for ch := range 2 {
		length := 0
		for _, f := range frames {
			if math.Abs(f[ch]) >= fullScale {
				length++
				if length == 3 {
					runs++
				}
			} else {
				length = 0
			}
		}
	}
	return runs
}

func toDB(amplitude float64) float64 {
	if amplitude <= 0 {
		return math.Inf(-1)
	}
	return 20 * math.Log10(amplitude)
}

func energyToLUFS(meanSquare float64) float64 {
	if meanSquare <= 0 {
		return math.Inf(-1)
	}
	return -0.691 + 10*math.Log10(meanSquare)
}

// gatedMean averages the blocks louder than gateLUFS.
func gatedMean(blocks []float64, gateLUFS float64) float64 {
	var sum float64
	n := 0
	for _, b := range blocks {
		if energyToLUFS(b) > gateLUFS {
			sum += b
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// biquad is a direct form I second-order filter with a0 normalized to 1.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     float64
}

func (q *biquad) process(x float64) float64 {
	y := q.b0*x + q.b1*q.x1 + q.b2*q.x2 - q.a1*q.y1 - q.a2*q.y2
	q.x2, q.x1 = q.x1, x
	q.y2, q.y1 = q.y1, y
	return y
}

// kWeighting is the BS.1770 pre-filter: a high shelf modelling the head,
// then a high-pass. The coefficients are derived from the analog prototypes
// so they hold at any sample rate, not only the 48 kHz the standard tabulates.
type kWeighting struct{ shelf, highpass biquad }

func newKWeighting(rate float64) kWeighting {
	var k kWeighting

	f0, gain, q := 1681.974450955533, 3.999843853973347, 0.7071752369554196
	w := math.Tan(math.Pi * f0 / rate)
	vh := math.Pow(10, gain/20)
	vb := math.Pow(vh, 0.4996667741545416)
	a0 := 1 + w/q + w*w
	k.shelf = biquad{
		b0: (vh + vb*w/q + w*w) / a0,
		b1: 2 * (w*w - vh) / a0,
		b2: (vh - vb*w/q + w*w) / a0,
		a1: 2 * (w*w - 1) / a0,
		a2: (1 - w/q + w*w) / a0,
	}

	f0, q = 38.13547087602444, 0.5003270373238773
	w = math.Tan(math.Pi * f0 / rate)
	a0 = 1 + w/q + w*w
	k.highpass = biquad{
		b0: 1, b1: -2, b2: 1,
		a1: 2 * (w*w - 1) / a0,
		a2: (1 - w/q + w*w) / a0,
	}
	return k
}

func (k *kWeighting) process(x float64) float64 {
	return k.highpass.process(k.shelf.process(x))
}

// truePeak estimates the inter-sample peak by evaluating a windowed-sinc
// interpolator at the three quarter-sample points after each sample. Only
// samples within 6 dB of the sample peak are interpolated: an inter-sample
// peak above the sample peak has to sit next to a large sample.
func truePeak(frames [][2]float64, samplePeak float64) float64 {
	const taps = 12 // samples on each side
	var kernel [3][2 * taps]float64
	for p := range kernel {
		frac := float64(p+1) / 4
		for k := range kernel[p] {
			// Distance from the interpolation point to sample (k - taps + 1).
			t := float64(k-taps+1) - frac
			kernel[p][k] = sinc(t) * sinc(t/taps)
		}
	}

	peak := samplePeak
	floor := samplePeak / 2
	for ch := range 2 {
		for n := range frames {
			if math.Abs(frames[n][ch]) < floor {
				continue
			}
			for p := range kernel {
				var y float64
				for k, h := range kernel[p] {
					if i := n + k - taps + 1; i >= 0 && i < len(frames) {
						y += frames[i][ch] * h
					}
				}
				peak = math.Max(peak, math.Abs(y))
			}
		}
	}
	return peak
}

func sinc(x float64) float64 {
	if x == 0 {
		return 1
	}
	return math.Sin(math.Pi*x) / (math.Pi * x)
}
