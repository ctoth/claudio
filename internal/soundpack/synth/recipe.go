// Package synth renders original sounds from a JSON recipe, so a soundpack
// can be designed as text: layers of oscillators with envelopes, glides,
// FM and filters, followed by effects. Rendering is deterministic; the same
// recipe always produces the same samples.
package synth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SampleRate is the rate sounds are rendered at: the player's own.
const SampleRate = 48000

const (
	maxLayers       = 64
	maxNotes        = 64
	maxVoices       = 16
	maxSoundSeconds = 10.0
	minHz, maxHz    = 20.0, 20000.0
)

// Recipe is a soundpack described as synthesis instructions: named sounds,
// and which sound answers for which key.
type Recipe struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Sounds      map[string]*Sound `json:"sounds"`
	// Mappings maps a sound key (such as "success/success.wav") to the
	// name of a sound in Sounds.
	Mappings map[string]string `json:"mappings"`
}

// Sound is one or more layers mixed together and passed through effects in
// order.
type Sound struct {
	// About says what the sound is meant to be. It is not rendered.
	About   string   `json:"about,omitempty"`
	Layers  []Layer  `json:"layers"`
	Effects []Effect `json:"effects,omitempty"`
}

// Layer is one oscillator (or noise source) with an envelope. Times are in
// seconds, gain in dB.
type Layer struct {
	// Wave is sine (default), triangle, square, saw, pulse or noise.
	Wave string `json:"wave,omitempty"`
	// Freq is the pitch in Hz or as a note name ("A4", "C#5"). FreqEnd, if
	// set, glides to that pitch over Dur.
	Freq    Pitch `json:"freq,omitempty"`
	FreqEnd Pitch `json:"freq_end,omitempty"`
	// Notes plays a sequence instead of one pitch: each note lasts Dur and
	// starts Step after the previous one. A 0 is a rest.
	Notes []Pitch `json:"notes,omitempty"`
	Step  float64 `json:"step,omitempty"`

	// At is when the layer starts. Dur is how long a note is held before
	// its release.
	At  float64 `json:"at,omitempty"`
	Dur float64 `json:"dur"`

	// Attack rises to full level (default 5 ms). Decay then falls to
	// Sustain (0..1, default 1); with sustain 0 that is a pluck. Release
	// fades out after Dur (default 50 ms).
	Attack  *float64 `json:"attack,omitempty"`
	Decay   float64  `json:"decay,omitempty"`
	Sustain *float64 `json:"sustain,omitempty"`
	Release *float64 `json:"release,omitempty"`

	Gain float64 `json:"gain,omitempty"`
	// Pan runs from -1 (left) to 1 (right).
	Pan float64 `json:"pan,omitempty"`

	// Harmonics gives a sine wave overtones: amplitudes of the 1st, 2nd,
	// 3rd... harmonic.
	Harmonics []float64 `json:"harmonics,omitempty"`
	// Width is the pulse wave's duty cycle (default 0.25).
	Width float64 `json:"width,omitempty"`

	FM      *FM  `json:"fm,omitempty"`
	Vibrato *LFO `json:"vibrato,omitempty"`
	Tremolo *LFO `json:"tremolo,omitempty"`

	// Voices stacks copies detuned across +/- Detune cents and spread in
	// stereo.
	Voices int     `json:"voices,omitempty"`
	Detune float64 `json:"detune,omitempty"`

	// Lowpass and Highpass are filter cutoffs in Hz. The _end forms sweep
	// the cutoff over the note. Q is resonance (default 0.707).
	Lowpass     float64 `json:"lowpass,omitempty"`
	LowpassEnd  float64 `json:"lowpass_end,omitempty"`
	Highpass    float64 `json:"highpass,omitempty"`
	HighpassEnd float64 `json:"highpass_end,omitempty"`
	Q           float64 `json:"q,omitempty"`
}

// FM modulates the layer's phase with a sine at Ratio times its pitch.
// Index is the depth; IndexEnd, if set, is the depth at the end of the note
// (a falling index gives a bell or pluck).
type FM struct {
	Ratio    float64  `json:"ratio"`
	Index    float64  `json:"index"`
	IndexEnd *float64 `json:"index_end,omitempty"`
}

// LFO is a slow sine. For vibrato Depth is in semitones; for tremolo it is
// the fraction of level removed at the bottom of each cycle (0..1).
type LFO struct {
	Rate  float64 `json:"rate"`
	Depth float64 `json:"depth"`
}

// Effect is one processing step on the whole sound. Type selects it:
//
//	reverb    size (0..1), damp (0..1, default 0.5), mix (0..1)
//	delay     time (s), feedback (0..0.9), mix (0..1)
//	drive     amount (0..1)
//	bitcrush  bits (1..16), downsample (1..64)
//	lowpass   freq (Hz), q
//	highpass  freq (Hz), q
type Effect struct {
	Type       string  `json:"type"`
	Size       float64 `json:"size,omitempty"`
	Damp       float64 `json:"damp,omitempty"`
	Mix        float64 `json:"mix,omitempty"`
	Time       float64 `json:"time,omitempty"`
	Feedback   float64 `json:"feedback,omitempty"`
	Amount     float64 `json:"amount,omitempty"`
	Bits       int     `json:"bits,omitempty"`
	Downsample int     `json:"downsample,omitempty"`
	Freq       float64 `json:"freq,omitempty"`
	Q          float64 `json:"q,omitempty"`
}

// Pitch is a frequency in Hz. In JSON it is a number, or a note name such
// as "A4", "C#5" or "Bb3" (A4 = 440 Hz).
type Pitch float64

var noteName = regexp.MustCompile(`^([A-Ga-g])([#b]?)(-?\d)$`)

func (p *Pitch) UnmarshalJSON(data []byte) error {
	var hz float64
	if err := json.Unmarshal(data, &hz); err == nil {
		if hz < 0 || math.IsNaN(hz) || math.IsInf(hz, 0) {
			return fmt.Errorf("pitch %v is not a frequency", hz)
		}
		*p = Pitch(hz)
		return nil
	}
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return fmt.Errorf("pitch %s must be a number of Hz or a note name like \"A4\"", data)
	}
	m := noteName.FindStringSubmatch(name)
	if m == nil {
		return fmt.Errorf("pitch %q is not a note name like \"A4\", \"C#5\" or \"Bb3\"", name)
	}
	semitone := map[byte]int{'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}[strings.ToUpper(m[1])[0]]
	switch m[2] {
	case "#":
		semitone++
	case "b":
		semitone--
	}
	octave, _ := strconv.Atoi(m[3])
	midi := 12*(octave+1) + semitone
	*p = Pitch(440 * math.Pow(2, float64(midi-69)/12))
	return nil
}

var soundName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// Parse reads and checks a recipe. Unknown fields are errors, so a
// misspelled setting fails instead of being silently ignored.
func Parse(data []byte) (*Recipe, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var r Recipe
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid recipe: %w", err)
	}
	if err := r.validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// SoundNames returns the recipe's sound names in order.
func (r *Recipe) SoundNames() []string {
	names := make([]string, 0, len(r.Sounds))
	for name := range r.Sounds {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Recipe) validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return errors.New("recipe has no name")
	}
	if len(r.Sounds) == 0 {
		return errors.New("recipe has no sounds")
	}
	if len(r.Mappings) == 0 {
		return errors.New("recipe has no mappings")
	}
	for _, name := range r.SoundNames() {
		if !soundName.MatchString(name) {
			return fmt.Errorf("sound name %q must be lowercase letters, digits, - and _ (it becomes a file name)", name)
		}
		s := r.Sounds[name]
		if s == nil {
			return fmt.Errorf("%s: no layers", name)
		}
		if err := s.validate(name); err != nil {
			return err
		}
	}

	used := make(map[string]bool)
	keys := make([]string, 0, len(r.Mappings))
	for key := range r.Mappings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		target := r.Mappings[key]
		if _, ok := r.Sounds[target]; !ok {
			return fmt.Errorf("%s: no sound named %q", key, target)
		}
		used[target] = true
	}
	for _, name := range r.SoundNames() {
		if !used[name] {
			return fmt.Errorf("sound %q is not used by any mapping", name)
		}
	}
	return nil
}

func (s *Sound) validate(name string) error {
	if len(s.Layers) == 0 {
		return fmt.Errorf("%s: no layers", name)
	}
	if len(s.Layers) > maxLayers {
		return fmt.Errorf("%s: %d layers is more than the limit of %d", name, len(s.Layers), maxLayers)
	}
	for i := range s.Layers {
		if err := s.Layers[i].validate(); err != nil {
			return fmt.Errorf("%s, layer %d: %w", name, i+1, err)
		}
	}
	for i := range s.Effects {
		if err := s.Effects[i].validate(); err != nil {
			return fmt.Errorf("%s, effect %d: %w", name, i+1, err)
		}
	}
	if d := s.drySeconds(); d > maxSoundSeconds {
		return fmt.Errorf("%s: %.2f s is longer than the %.0f s limit", name, d, maxSoundSeconds)
	}
	return nil
}

// drySeconds is the length of the sound before any effect tail.
func (s *Sound) drySeconds() float64 {
	var end float64
	for i := range s.Layers {
		end = math.Max(end, s.Layers[i].end())
	}
	return end
}

func (l *Layer) attack() float64 {
	if l.Attack != nil {
		return *l.Attack
	}
	return 0.005
}

func (l *Layer) sustain() float64 {
	if l.Sustain != nil {
		return *l.Sustain
	}
	return 1
}

func (l *Layer) release() float64 {
	if l.Release != nil {
		return *l.Release
	}
	return 0.05
}

// noteCount is how many notes the layer plays.
func (l *Layer) noteCount() int {
	if len(l.Notes) > 0 {
		return len(l.Notes)
	}
	return 1
}

// end is when the layer's last note has finished releasing.
func (l *Layer) end() float64 {
	return l.At + float64(l.noteCount()-1)*l.Step + l.Dur + l.release()
}

func inRange(name string, v, lo, hi float64) error {
	if v < lo || v > hi || math.IsNaN(v) {
		return fmt.Errorf("%s: %v is outside %v..%v", name, v, lo, hi)
	}
	return nil
}

func hzOrUnset(name string, hz float64) error {
	if hz == 0 {
		return nil
	}
	return inRange(name, hz, minHz, maxHz)
}

func (l *Layer) validate() error {
	switch l.Wave {
	case "", "sine", "triangle", "square", "saw", "pulse", "noise":
	default:
		return fmt.Errorf("unknown wave %q (use sine, triangle, square, saw, pulse or noise)", l.Wave)
	}
	if l.Dur <= 0 {
		return errors.New("dur: must be greater than 0")
	}

	switch {
	case len(l.Notes) > 0:
		if l.Freq != 0 || l.FreqEnd != 0 {
			return errors.New("notes: cannot be combined with freq or freq_end")
		}
		if len(l.Notes) > maxNotes {
			return fmt.Errorf("notes: %d is more than the limit of %d", len(l.Notes), maxNotes)
		}
		if len(l.Notes) > 1 && l.Step <= 0 {
			return errors.New("step: must be greater than 0 when there are several notes")
		}
		for i, n := range l.Notes {
			if err := hzOrUnset(fmt.Sprintf("notes[%d]", i), float64(n)); err != nil {
				return err
			}
		}
	case l.Wave == "noise":
		if l.Freq != 0 || l.FreqEnd != 0 {
			return errors.New("freq: noise has no pitch; shape it with lowpass and highpass")
		}
	default:
		if l.Freq == 0 {
			return errors.New("freq: required (or notes) unless wave is noise")
		}
		if err := inRange("freq", float64(l.Freq), minHz, maxHz); err != nil {
			return err
		}
		if err := hzOrUnset("freq_end", float64(l.FreqEnd)); err != nil {
			return err
		}
	}

	checks := []error{
		inRange("at", l.At, 0, maxSoundSeconds),
		inRange("step", l.Step, 0, maxSoundSeconds),
		inRange("attack", l.attack(), 0, maxSoundSeconds),
		inRange("decay", l.Decay, 0, maxSoundSeconds),
		inRange("sustain", l.sustain(), 0, 1),
		inRange("release", l.release(), 0, maxSoundSeconds),
		inRange("gain", l.Gain, -60, 24),
		inRange("pan", l.Pan, -1, 1),
		inRange("width", l.Width, 0, 0.95),
		inRange("voices", float64(l.Voices), 0, maxVoices),
		inRange("detune", l.Detune, 0, 100),
		hzOrUnset("lowpass", l.Lowpass),
		hzOrUnset("lowpass_end", l.LowpassEnd),
		hzOrUnset("highpass", l.Highpass),
		hzOrUnset("highpass_end", l.HighpassEnd),
		inRange("q", l.Q, 0, 30),
	}
	if l.LowpassEnd != 0 && l.Lowpass == 0 {
		checks = append(checks, errors.New("lowpass_end: needs lowpass as the starting cutoff"))
	}
	if l.HighpassEnd != 0 && l.Highpass == 0 {
		checks = append(checks, errors.New("highpass_end: needs highpass as the starting cutoff"))
	}
	if len(l.Harmonics) > 32 {
		checks = append(checks, errors.New("harmonics: at most 32"))
	}
	if l.FM != nil {
		checks = append(checks, inRange("fm.ratio", l.FM.Ratio, 0.01, 32), inRange("fm.index", l.FM.Index, 0, 50))
		if l.FM.IndexEnd != nil {
			checks = append(checks, inRange("fm.index_end", *l.FM.IndexEnd, 0, 50))
		}
	}
	if l.Vibrato != nil {
		checks = append(checks, inRange("vibrato.rate", l.Vibrato.Rate, 0, 100), inRange("vibrato.depth", l.Vibrato.Depth, 0, 24))
	}
	if l.Tremolo != nil {
		checks = append(checks, inRange("tremolo.rate", l.Tremolo.Rate, 0, 100), inRange("tremolo.depth", l.Tremolo.Depth, 0, 1))
	}
	return errors.Join(checks...)
}

func (e *Effect) validate() error {
	switch e.Type {
	case "reverb":
		return errors.Join(inRange("size", e.Size, 0, 1), inRange("damp", e.Damp, 0, 1), inRange("mix", e.Mix, 0, 1))
	case "delay":
		return errors.Join(inRange("time", e.Time, 0.001, 2), inRange("feedback", e.Feedback, 0, 0.9), inRange("mix", e.Mix, 0, 1))
	case "drive":
		return inRange("amount", e.Amount, 0, 1)
	case "bitcrush":
		return errors.Join(inRange("bits", float64(e.Bits), 1, 16), inRange("downsample", float64(e.Downsample), 0, 64))
	case "lowpass", "highpass":
		return errors.Join(inRange("freq", e.Freq, minHz, maxHz), inRange("q", e.Q, 0, 30))
	}
	return fmt.Errorf("unknown type %q (use reverb, delay, drive, bitcrush, lowpass or highpass)", e.Type)
}
