package synth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"claudio.click/internal/soundpack"
	"claudio.click/internal/soundpack/master"
)

const (
	// Rendered sounds peak at -3 dBFS. Mastering sets the final level;
	// this only keeps the 16-bit file well above its noise floor.
	outputPeak = 0.708
	// Level the dry mix is brought to before effects, so drive and
	// bitcrush respond the same whatever the layer gains add up to.
	effectsPeak = 0.9
)

func frameCount(seconds float64) int {
	return int(math.Round(seconds * SampleRate))
}

// Render synthesizes the named sound.
func (r *Recipe) Render(name string) ([][2]float64, error) {
	s, ok := r.Sounds[name]
	if !ok {
		return nil, fmt.Errorf("no sound named %q", name)
	}
	seed := fnv.New64a()
	seed.Write([]byte(name))

	dry := frameCount(s.drySeconds())
	out := make([][2]float64, dry)
	for i := range s.Layers {
		l := &s.Layers[i]
		if l.Sample != "" && !l.loaded {
			return nil, fmt.Errorf("%s, layer %d: sample %s is not loaded (call LoadSamples first)", name, i+1, l.Sample)
		}
		rng := rand.New(rand.NewSource(int64(seed.Sum64()) + int64(i)*7919))
		for n := range l.noteCount() {
			freq := float64(l.Freq)
			if len(l.Notes) > 0 {
				if freq = float64(l.Notes[n]); freq == 0 {
					continue // rest
				}
			}
			var note [][2]float64
			if l.Sample != "" {
				note = l.sampleNote()
			} else {
				note = l.renderNote(freq, rng)
			}
			l.filter(note)
			at := frameCount(l.At + float64(n)*l.Step)
			gain := math.Pow(10, l.Gain/20)
			for j, f := range note {
				if at+j >= len(out) {
					break
				}
				out[at+j][0] += f[0] * gain
				out[at+j][1] += f[1] * gain
			}
		}
	}

	if len(s.Effects) > 0 {
		normalize(out, effectsPeak)
		for i := range s.Effects {
			out = s.Effects[i].apply(out)
		}
		out = trimTail(out, dry)
	}
	normalize(out, outputPeak)
	return out, nil
}

// renderNote renders one note of an oscillator layer at freq: every voice,
// panned and summed.
func (l *Layer) renderNote(freq float64, rng *rand.Rand) [][2]float64 {
	total := l.Dur + l.release()
	out := make([][2]float64, frameCount(total))
	voices := max(1, l.Voices)
	if l.Wave == "noise" {
		voices = 1
	}

	for v := range voices {
		spread := 0.0
		if voices > 1 {
			spread = 2*float64(v)/float64(voices-1) - 1
		}
		detune := math.Pow(2, spread*l.Detune/1200)
		pan := math.Max(-1, math.Min(1, l.Pan+0.6*spread))
		left, right := math.Cos((pan+1)*math.Pi/4), math.Sin((pan+1)*math.Pi/4)

		phase, modPhase := 0.0, 0.0
		if v > 0 {
			phase = rng.Float64()
		}
		for i := range out {
			t := float64(i) / SampleRate
			pos := math.Min(t/l.Dur, 1)

			f := freq * detune
			if l.FreqEnd != 0 {
				f *= math.Pow(float64(l.FreqEnd)/freq, pos)
			}
			if l.Vibrato != nil {
				f *= math.Pow(2, l.Vibrato.Depth*math.Sin(2*math.Pi*l.Vibrato.Rate*t)/12)
			}
			dt := f / SampleRate

			offset := 0.0
			if l.FM != nil {
				index := l.FM.Index
				if l.FM.IndexEnd != nil {
					index += (*l.FM.IndexEnd - index) * pos
				}
				offset = index * math.Sin(2*math.Pi*modPhase) / (2 * math.Pi)
				modPhase += dt * l.FM.Ratio
			}

			x := l.wave(phase+offset, dt, rng) * l.envelope(t)
			if l.Tremolo != nil {
				x *= 1 - l.Tremolo.Depth*(0.5+0.5*math.Sin(2*math.Pi*l.Tremolo.Rate*t))
			}
			out[i][0] += x * left
			out[i][1] += x * right
			phase += dt
		}
	}

	scale := 1 / math.Sqrt(float64(voices))
	for i := range out {
		out[i][0] *= scale
		out[i][1] *= scale
	}
	return out
}

// filter applies the layer's lowpass and highpass to one rendered note.
func (l *Layer) filter(note [][2]float64) {
	if l.Lowpass != 0 {
		sweepFilter(note, lowpass, l.Lowpass, l.LowpassEnd, l.Q)
	}
	if l.Highpass != 0 {
		sweepFilter(note, highpass, l.Highpass, l.HighpassEnd, l.Q)
	}
}

// envelope is the layer's level at time t into a note.
func (l *Layer) envelope(t float64) float64 {
	held := func(t float64) float64 {
		attack := l.attack()
		if t < attack {
			return t / attack
		}
		if l.Decay == 0 {
			return l.sustain()
		}
		return l.sustain() + (1-l.sustain())*math.Exp(-5*(t-attack)/l.Decay)
	}
	if t < l.Dur {
		return held(t)
	}
	release := l.release()
	if release == 0 || t >= l.Dur+release {
		return 0
	}
	// Exponential fall, tapered so it reaches exactly zero.
	x := (t - l.Dur) / release
	return held(l.Dur) * math.Exp(-5*x) * (1 - x)
}

func frac(x float64) float64 {
	return x - math.Floor(x)
}

// polyBLEP smooths the step of a saw or pulse wave at phase 0, which is
// what keeps their aliasing down.
func polyBLEP(t, dt float64) float64 {
	switch {
	case dt <= 0:
		return 0
	case t < dt:
		x := t / dt
		return x + x - x*x - 1
	case t > 1-dt:
		x := (t - 1) / dt
		return x*x + x + x + 1
	}
	return 0
}

// wave returns the oscillator's value at phase (in cycles). dt is the phase
// step per sample.
func (l *Layer) wave(phase, dt float64, rng *rand.Rand) float64 {
	p := frac(phase)
	switch l.Wave {
	case "noise":
		return rng.Float64()*2 - 1
	case "triangle":
		return 1 - 4*math.Abs(frac(p+0.25)-0.5)
	case "saw":
		return 2*p - 1 - polyBLEP(p, dt)
	case "square", "pulse":
		width := 0.5
		if l.Wave == "pulse" {
			width = 0.25
		}
		if l.Width > 0 {
			width = l.Width
		}
		x := -1.0
		if p < width {
			x = 1
		}
		x += polyBLEP(p, dt) - polyBLEP(frac(p+1-width), dt)
		return x - (2*width - 1) // remove the DC a lopsided pulse carries
	}
	if len(l.Harmonics) == 0 {
		return math.Sin(2 * math.Pi * p)
	}
	var sum, norm float64
	for k, amp := range l.Harmonics {
		norm += math.Abs(amp)
		if float64(k+1)*dt < 0.5 { // skip harmonics above Nyquist
			sum += amp * math.Sin(2*math.Pi*float64(k+1)*phase)
		}
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
}

func normalize(frames [][2]float64, peak float64) {
	var top float64
	for _, f := range frames {
		top = math.Max(top, math.Max(math.Abs(f[0]), math.Abs(f[1])))
	}
	if top == 0 {
		return
	}
	scale := peak / top
	for i := range frames {
		frames[i][0] *= scale
		frames[i][1] *= scale
	}
}

// trimTail cuts an effect tail where it falls 80 dB below the peak, never
// shorter than the dry sound, and fades the last 20 ms so the cut is clean.
func trimTail(frames [][2]float64, dry int) [][2]float64 {
	var top float64
	for _, f := range frames {
		top = math.Max(top, math.Max(math.Abs(f[0]), math.Abs(f[1])))
	}
	end := len(frames)
	for end > dry && math.Abs(frames[end-1][0]) < top*1e-4 && math.Abs(frames[end-1][1]) < top*1e-4 {
		end--
	}
	frames = frames[:end]
	if end > dry {
		fade := min(frameCount(0.02), end-dry)
		for i := range fade {
			g := float64(fade-i-1) / float64(fade)
			frames[end-fade+i][0] *= g
			frames[end-fade+i][1] *= g
		}
	}
	return frames
}

// Written describes one rendered sound file.
type Written struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Seconds float64 `json:"seconds"`
	Keys    int     `json:"keys"`
}

// Write renders every sound into dir/synth/<name>.wav and writes
// dir/soundpack.json, a JSON soundpack mapping the recipe's keys to those
// files. The result is raw material: master it to get a pack at a
// consistent level.
func (r *Recipe) Write(dir string) ([]Written, error) {
	if err := os.MkdirAll(filepath.Join(dir, "synth"), 0o755); err != nil {
		return nil, err
	}
	keys := make(map[string]int)
	manifest := soundpack.JSONSoundpackFile{
		Name: r.Name, Description: r.Description, Version: r.Version,
		Mappings: make(map[string]string, len(r.Mappings)),
	}
	for key, sound := range r.Mappings {
		manifest.Mappings[key] = "synth/" + sound + ".wav"
		keys[sound]++
	}

	var written []Written
	for _, name := range r.SoundNames() {
		frames, err := r.Render(name)
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		if err := master.EncodeWAV(&buf, frames); err != nil {
			return nil, err
		}
		rel := "synth/" + name + ".wav"
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(rel)), buf.Bytes(), 0o644); err != nil {
			return nil, err
		}
		slog.Debug("rendered sound", "name", name, "frames", len(frames))
		written = append(written, Written{Name: name, Path: rel, Seconds: float64(len(frames)) / SampleRate, Keys: keys[name]})
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "soundpack.json"), append(data, '\n'), 0o644); err != nil {
		return nil, err
	}
	slog.Info("rendered recipe", "name", r.Name, "sounds", len(written), "dir", dir)
	return written, nil
}
