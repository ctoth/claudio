package synth

import "math"

// biquad is a second-order filter (direct form I, a0 normalized to 1) with
// state for both channels.
type biquad struct {
	b0, b1, b2, a1, a2 float64
	x1, x2, y1, y2     [2]float64
}

func (q *biquad) process(f [2]float64) [2]float64 {
	var out [2]float64
	for ch := range 2 {
		y := q.b0*f[ch] + q.b1*q.x1[ch] + q.b2*q.x2[ch] - q.a1*q.y1[ch] - q.a2*q.y2[ch]
		q.x2[ch], q.x1[ch] = q.x1[ch], f[ch]
		q.y2[ch], q.y1[ch] = q.y1[ch], y
		out[ch] = y
	}
	return out
}

type filterKind int

const (
	lowpass filterKind = iota
	highpass
)

// tune sets the coefficients (RBJ cookbook) without disturbing the state,
// so the cutoff can move while the filter runs.
func (q *biquad) tune(kind filterKind, hz, resonance float64) {
	if resonance <= 0 {
		resonance = math.Sqrt2 / 2
	}
	hz = math.Max(minHz, math.Min(hz, 0.45*SampleRate))
	w := 2 * math.Pi * hz / SampleRate
	cos, alpha := math.Cos(w), math.Sin(w)/(2*resonance)
	a0 := 1 + alpha
	switch kind {
	case lowpass:
		q.b0, q.b1, q.b2 = (1-cos)/2/a0, (1-cos)/a0, (1-cos)/2/a0
	case highpass:
		q.b0, q.b1, q.b2 = (1+cos)/2/a0, -(1+cos)/a0, (1+cos)/2/a0
	}
	q.a1, q.a2 = -2*cos/a0, (1-alpha)/a0
}

// sweepFilter filters frames in place, moving the cutoff exponentially from
// from to to (or holding it when to is 0).
func sweepFilter(frames [][2]float64, kind filterKind, from, to, resonance float64) {
	var q biquad
	for i := range frames {
		if i%16 == 0 {
			hz := from
			if to != 0 {
				hz = from * math.Pow(to/from, float64(i)/float64(len(frames)))
			}
			q.tune(kind, hz, resonance)
		}
		frames[i] = q.process(frames[i])
	}
}

// apply runs the effect over frames and returns the result, which is longer
// than the input when the effect rings on.
func (e *Effect) apply(frames [][2]float64) [][2]float64 {
	switch e.Type {
	case "lowpass":
		sweepFilter(frames, lowpass, e.Freq, 0, e.Q)
	case "highpass":
		sweepFilter(frames, highpass, e.Freq, 0, e.Q)
	case "drive":
		gain := 1 + 20*e.Amount
		norm := math.Tanh(gain)
		for i := range frames {
			frames[i][0] = math.Tanh(frames[i][0]*gain) / norm
			frames[i][1] = math.Tanh(frames[i][1]*gain) / norm
		}
	case "bitcrush":
		levels := math.Pow(2, float64(e.Bits-1))
		hold := max(1, e.Downsample)
		var held [2]float64
		for i := range frames {
			if i%hold == 0 {
				held[0] = math.Round(frames[i][0]*levels) / levels
				held[1] = math.Round(frames[i][1]*levels) / levels
			}
			frames[i] = held
		}
	case "delay":
		return e.delay(frames)
	case "reverb":
		return e.reverb(frames)
	}
	return frames
}

// delay is a feedback echo. The tail is as long as the echoes take to fall
// 80 dB.
func (e *Effect) delay(frames [][2]float64) [][2]float64 {
	step := max(1, frameCount(e.Time))
	echoes := 1.0
	if e.Feedback > 0 {
		echoes = math.Ceil(math.Log(1e-4) / math.Log(e.Feedback))
	}
	tail := min(int(echoes+1)*step, frameCount(6))

	line := make([][2]float64, step)
	out := make([][2]float64, len(frames)+tail)
	for i := range out {
		var in [2]float64
		if i < len(frames) {
			in = frames[i]
		}
		echo := line[i%step]
		for ch := range 2 {
			line[i%step][ch] = in[ch] + echo[ch]*e.Feedback
			out[i][ch] = in[ch]*(1-e.Mix) + echo[ch]*e.Mix
		}
	}
	return out
}

// Freeverb delay-line lengths at 44.1 kHz, scaled to the render rate. The
// right channel's lines are longer by stereoSpread, which is what makes the
// reverb wide.
var (
	combTunings    = []int{1116, 1188, 1277, 1356, 1422, 1491, 1557, 1617}
	allpassTunings = []int{556, 441, 341, 225}
)

const stereoSpread = 23

type delayLine struct {
	buf   []float64
	pos   int
	store float64
}

func newLine(samples int) *delayLine {
	return &delayLine{buf: make([]float64, samples*SampleRate/44100)}
}

func (d *delayLine) comb(in, feedback, damp float64) float64 {
	out := d.buf[d.pos]
	d.store = out*(1-damp) + d.store*damp
	d.buf[d.pos] = in + d.store*feedback
	d.pos = (d.pos + 1) % len(d.buf)
	return out
}

func (d *delayLine) allpass(in float64) float64 {
	out := d.buf[d.pos]
	d.buf[d.pos] = in + out*0.5
	d.pos = (d.pos + 1) % len(d.buf)
	return out - in
}

// reverb is Freeverb: eight damped comb filters in parallel into four
// allpasses in series, per channel. Size sets how long it rings.
func (e *Effect) reverb(frames [][2]float64) [][2]float64 {
	feedback := 0.7 + 0.28*e.Size
	damp := e.Damp
	if damp == 0 {
		damp = 0.5
	}
	damp *= 0.4

	var combs, allpasses [2][]*delayLine
	for ch := range 2 {
		for _, n := range combTunings {
			combs[ch] = append(combs[ch], newLine(n+ch*stereoSpread))
		}
		for _, n := range allpassTunings {
			allpasses[ch] = append(allpasses[ch], newLine(n+ch*stereoSpread))
		}
	}

	out := make([][2]float64, len(frames)+frameCount(0.5+5.5*e.Size))
	for i := range out {
		var in [2]float64
		if i < len(frames) {
			in = frames[i]
		}
		send := (in[0] + in[1]) * 0.015
		for ch := range 2 {
			var wet float64
			for _, c := range combs[ch] {
				wet += c.comb(send, feedback, damp)
			}
			for _, a := range allpasses[ch] {
				wet = a.allpass(wet)
			}
			out[i][ch] = in[ch]*(1-e.Mix) + wet*3*e.Mix
		}
	}
	return out
}
