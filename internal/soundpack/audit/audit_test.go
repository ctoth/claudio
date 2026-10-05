package audit

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"claudio.click/internal/testutil/wavfixture"
)

const rate = 48000

// tone returns a stereo sine with 5 ms fades so it has no edge clicks.
func tone(freq, peakDB float64, d time.Duration) [][2]float64 {
	amp := math.Pow(10, peakDB/20)
	n := int(d.Seconds() * rate)
	fade := rate * 5 / 1000
	frames := make([][2]float64, n)
	for i := range frames {
		g := 1.0
		if i < fade {
			g = float64(i) / float64(fade)
		} else if n-i <= fade {
			g = float64(n-i-1) / float64(fade)
		}
		x := g * amp * math.Sin(2*math.Pi*freq*float64(i)/rate)
		frames[i] = [2]float64{x, x}
	}
	return frames
}

func gap(d time.Duration) [][2]float64 {
	return make([][2]float64, int(d.Seconds()*rate))
}

func join(parts ...[][2]float64) [][2]float64 {
	var out [][2]float64
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func rules(s Sound) []string {
	var out []string
	for _, f := range s.Findings {
		out = append(out, f.Rule)
	}
	return out
}

func hasRule(s Sound, rule string) bool {
	for _, f := range s.Findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func analyze(frames [][2]float64, categories ...string) Sound {
	s := Sound{Path: "x.wav", Categories: categories}
	Analyze(&s, frames, rate, DefaultOptions())
	return s
}

func TestCleanSoundHasNoFindings(t *testing.T) {
	s := analyze(tone(1000, -18, 500*time.Millisecond), "success")
	if len(s.Findings) != 0 {
		t.Fatalf("findings = %v, want none (%+v)", rules(s), s)
	}
	if math.Abs(float64(s.LoudnessLUFS)+18) > 0.5 {
		t.Errorf("loudness = %v, want about -18", s.LoudnessLUFS)
	}
	if math.Abs(s.DurationMS-500) > 1 {
		t.Errorf("duration = %v ms, want 500", s.DurationMS)
	}
}

func TestRules(t *testing.T) {
	ok := tone(1000, -18, 500*time.Millisecond)
	for _, tc := range []struct {
		name     string
		frames   [][2]float64
		category string
		want     string
	}{
		{"too loud", tone(1000, -6, 500*time.Millisecond), "success", RuleTooLoud},
		{"too quiet", tone(1000, -40, 500*time.Millisecond), "success", RuleTooQuiet},
		{"leading silence", join(gap(100*time.Millisecond), ok), "success", RuleLeadingSilence},
		{"trailing silence", join(ok, gap(400*time.Millisecond)), "success", RuleTrailingSilence},
		{"too long for loading", tone(1000, -18, 3*time.Second), "loading", RuleTooLong},
		{"peak over ceiling", tone(1000, 0, 500*time.Millisecond), "success", RulePeak},
		{"silent", gap(500 * time.Millisecond), "success", RuleSilent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := analyze(tc.frames, tc.category)
			if !hasRule(s, tc.want) {
				t.Errorf("findings = %v, want %s", rules(s), tc.want)
			}
		})
	}
}

// The same three seconds is fine as a completion sound: limits are per
// category because a sound that fires on every tool call has to be shorter
// than one that fires once per turn.
func TestDurationLimitDependsOnCategory(t *testing.T) {
	frames := tone(1000, -18, 2500*time.Millisecond)
	if s := analyze(frames, "completion"); hasRule(s, RuleTooLong) {
		t.Errorf("2.5 s completion sound flagged: %v", rules(s))
	}
	// A file that answers for several categories gets the strictest limit.
	if s := analyze(frames, "completion", "loading"); !hasRule(s, RuleTooLong) {
		t.Errorf("2.5 s sound shared with loading not flagged: %v", rules(s))
	}
}

// A click has almost no energy for its peak. It cannot reach the target
// without exceeding the ceiling, which is a property of the sound, not a
// mastering mistake.
func TestPeakLimitedSoundIsNotCalledQuiet(t *testing.T) {
	frames := gap(200 * time.Millisecond)
	peak := math.Pow(10, -1.0/20)
	for i := range 48 {
		frames[i] = [2]float64{peak * math.Sin(math.Pi*float64(i)/48), peak * math.Sin(math.Pi*float64(i)/48)}
	}
	s := analyze(frames[:rate/10], "success")
	if hasRule(s, RuleTooQuiet) || !hasRule(s, RulePeakLimited) {
		t.Errorf("findings = %v, want %s and not %s (loudness %v, peak %v)",
			rules(s), RulePeakLimited, RuleTooQuiet, s.LoudnessLUFS, s.PeakDBTP)
	}
}

func TestDescribeCountsOnsets(t *testing.T) {
	beep := tone(1200, -18, 80*time.Millisecond)
	s := analyze(join(beep, gap(60*time.Millisecond), beep, gap(60*time.Millisecond), beep), "success")
	if s.Onsets != 3 {
		t.Errorf("onsets = %d, want 3", s.Onsets)
	}
}

func TestDescribeFindsPitchAndTrend(t *testing.T) {
	steady := analyze(tone(1000, -18, 400*time.Millisecond), "success")
	if math.Abs(steady.DominantHz-1000) > 30 {
		t.Errorf("dominant = %v Hz, want about 1000", steady.DominantHz)
	}
	if steady.PitchTrend != "steady" || steady.Texture != "tonal" {
		t.Errorf("steady tone described as %s / %s", steady.PitchTrend, steady.Texture)
	}

	up := analyze(join(tone(600, -18, 200*time.Millisecond), tone(1800, -18, 200*time.Millisecond)), "success")
	if up.PitchTrend != "rising" {
		t.Errorf("600 -> 1800 Hz described as %s", up.PitchTrend)
	}
	down := analyze(join(tone(1800, -18, 200*time.Millisecond), tone(600, -18, 200*time.Millisecond)), "error")
	if down.PitchTrend != "falling" {
		t.Errorf("1800 -> 600 Hz described as %s", down.PitchTrend)
	}
}

// bandNoise is white noise through a one-pole lowpass: most of its energy is
// in a few hundred hertz, but it has no pitch.
func bandNoise(d time.Duration) [][2]float64 {
	rng := rand.New(rand.NewSource(7))
	frames := make([][2]float64, int(d.Seconds()*rate))
	var y float64
	for i := range frames {
		y += 0.05 * ((rng.Float64()*2 - 1) - y)
		frames[i] = [2]float64{y, y}
	}
	return frames
}

// Texture has to survive filtering: a rumble or a whoosh is noise even
// though its spectrum is far from flat, and a chord is tonal even though it
// is more than one frequency.
func TestTextureSeparatesPitchFromNoise(t *testing.T) {
	chord := tone(440, -24, 400*time.Millisecond)
	for i, f := range join(tone(554.37, -24, 400*time.Millisecond)) {
		chord[i][0] += f[0] + 0.06*math.Sin(2*math.Pi*659.25*float64(i)/rate)
		chord[i][1] = chord[i][0]
	}
	for name, tc := range map[string]struct {
		frames [][2]float64
		want   string
	}{
		"pure tone":      {tone(1000, -18, 400*time.Millisecond), "tonal"},
		"three-note mix": {chord, "tonal"},
		"filtered noise": {bandNoise(400 * time.Millisecond), "noisy"},
	} {
		if got := analyze(tc.frames, "success"); got.Texture != tc.want {
			t.Errorf("%s: texture %s (tonality %v), want %s", name, got.Texture, got.Tonality, tc.want)
		}
	}
}

// ring is a struck note: a fundamental with a bright overtone that dies
// faster than it does.
func ring(freq float64, d time.Duration) [][2]float64 {
	frames := make([][2]float64, int(d.Seconds()*rate))
	for i := range frames {
		t := float64(i) / rate
		pos := t / d.Seconds()
		x := 0.12*math.Exp(-4*pos)*math.Sin(2*math.Pi*freq*t) + 0.08*math.Exp(-25*pos)*math.Sin(2*math.Pi*3*freq*t)
		if i < 96 {
			x *= float64(i) / 96
		}
		frames[i] = [2]float64{x, x}
	}
	return frames
}

// Trend is about melody. Brightness follows loudness and decay, so a
// brightness measure calls every struck note "falling" and hides a rise
// that ends on a long ringing note.
func TestTrendFollowsPitchForPitchedSounds(t *testing.T) {
	for name, tc := range map[string]struct {
		frames [][2]float64
		want   string
	}{
		"one struck note fading":            {ring(800, 600*time.Millisecond), "steady"},
		"short low note, long ringing high": {join(tone(523, -18, 60*time.Millisecond), ring(1047, 600*time.Millisecond)), "rising"},
		"long ringing fall":                 {join(tone(1047, -18, 60*time.Millisecond), ring(523, 600*time.Millisecond)), "falling"},
		"low steady note":                   {ring(294, 300*time.Millisecond), "steady"},
	} {
		got := analyze(tc.frames, "success")
		if got.PitchTrend != tc.want {
			t.Errorf("%s: trend %s (starts %v Hz, ends %v Hz), want %s", name, got.PitchTrend, got.StartHz, got.EndHz, tc.want)
		}
	}
	// Noise has no pitch to follow; its trend is still its brightness.
	sweep := bandNoise(400 * time.Millisecond)
	for i := range sweep {
		g := float64(i) / float64(len(sweep))
		sweep[i][0] *= 1 - g
		sweep[i][1] = sweep[i][0]
	}
	if got := analyze(sweep, "success"); got.PitchTrend == "" {
		t.Error("noise must still get a trend")
	}
}

// The reported pitch is the note, not its loudest overtone.
func TestPitchIsTheFundamental(t *testing.T) {
	frames := make([][2]float64, rate/2)
	for i := range frames {
		t := float64(i) / rate
		x := 0.05*math.Sin(2*math.Pi*185*t) + 0.08*math.Sin(2*math.Pi*370*t)
		frames[i] = [2]float64{x, x}
	}
	got := analyze(frames, "success")
	if math.Abs(got.PitchHz-185) > 6 || math.Abs(got.DominantHz-370) > 25 {
		t.Errorf("pitch %v Hz (want 185), dominant %v Hz (want 370)", got.PitchHz, got.DominantHz)
	}
}

// A weaker component a semitone or so below a note is part of that note's
// skirt (leakage, a beating partner, an attack transient), not a lower
// note. Reading it as the pitch put short beeps 8% flat.
func TestPitchIgnoresAWeakerNeighbourOfThePeak(t *testing.T) {
	frames := make([][2]float64, rate/8)
	for i := range frames {
		t := float64(i) / rate
		x := 0.10*math.Sin(2*math.Pi*1048*t) + 0.06*math.Sin(2*math.Pi*963*t)
		frames[i] = [2]float64{x, x}
	}
	if got := analyze(frames, "success"); math.Abs(got.PitchHz-1048) > 12 {
		t.Errorf("pitch %v Hz, want 1048 (the stronger of two close components)", got.PitchHz)
	}
}

func writeWAV(t *testing.T, path string, frames [][2]float64) {
	t.Helper()
	in := make([][]float64, len(frames))
	for i, f := range frames {
		in[i] = []float64{f[0], f[1]}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, wavfixture.WAV(wavfixture.TagPCM, 16, rate, in), 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(t *testing.T, r Report, path string) Sound {
	t.Helper()
	for _, s := range r.Sounds {
		if s.Path == path {
			return s
		}
	}
	t.Fatalf("no sound %q in report", path)
	return Sound{}
}

func TestDirectoryPack(t *testing.T) {
	dir := t.TempDir()
	good := tone(1000, -18, 500*time.Millisecond)
	writeWAV(t, filepath.Join(dir, "success", "success.wav"), good)
	writeWAV(t, filepath.Join(dir, "success", "bash-success.wav"), good)
	writeWAV(t, filepath.Join(dir, "loading", "loading.wav"), tone(700, -6, 3*time.Second))
	writeWAV(t, filepath.Join(dir, ".git", "objects", "ignored.wav"), good)
	if err := os.MkdirAll(filepath.Join(dir, "error"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "error", "error.wav"), []byte("not audio"), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Pack(dir, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sounds) != 4 {
		t.Fatalf("audited %d sounds, want 4 (.git must be skipped)", len(r.Sounds))
	}
	if s := find(t, r, "error/error.wav"); !hasRule(s, RuleUndecodable) {
		t.Errorf("garbage file findings = %v", rules(s))
	}
	if s := find(t, r, "success/success.wav"); !hasRule(s, RuleDuplicate) || s.DuplicateOf != "success/bash-success.wav" {
		t.Errorf("duplicate not reported: %v (of %q)", rules(s), s.DuplicateOf)
	}
	if s := find(t, r, "loading/loading.wav"); !hasRule(s, RuleTooLong) || !hasRule(s, RuleTooLoud) {
		t.Errorf("loading findings = %v", rules(s))
	}
	if r.Summary.Errors != 1 || r.Summary.Warnings < 2 {
		t.Errorf("summary = %+v", r.Summary)
	}
	if !r.Failed(false) {
		t.Error("a pack with an undecodable file must fail")
	}

	// Silence and undecodable files have no level; the report must still be
	// valid JSON.
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"loudness_lufs":null`) {
		t.Errorf("undecodable file should report a null loudness: %s", data)
	}
}

func TestJSONPackUsesKeyCategories(t *testing.T) {
	dir := t.TempDir()
	writeWAV(t, filepath.Join(dir, "sounds", "chirp.wav"), tone(1000, -18, 3*time.Second))
	writeWAV(t, filepath.Join(dir, "sounds", "spare.wav"), tone(1000, -18, 300*time.Millisecond))
	// Raw material kept beside the pack is not part of it.
	writeWAV(t, filepath.Join(dir, "source", "raw.wav"), tone(1000, -3, 30*time.Second))
	manifest := `{"name":"p","mappings":{
		"completion/completion.wav":"sounds/chirp.wav",
		"loading/loading.wav":"sounds/chirp.wav"}}`
	path := filepath.Join(dir, "soundpack.json")
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Pack(path, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Sounds) != 2 {
		t.Fatalf("audited %d sounds, want the 2 under sounds/", len(r.Sounds))
	}
	chirp := find(t, r, "sounds/chirp.wav")
	if len(chirp.Keys) != 2 || !hasRule(chirp, RuleTooLong) {
		t.Errorf("chirp keys = %v findings = %v", chirp.Keys, rules(chirp))
	}
	if s := find(t, r, "sounds/spare.wav"); !hasRule(s, RuleUnreferenced) {
		t.Errorf("spare findings = %v", rules(s))
	}
}

func TestStrictFailsOnWarnings(t *testing.T) {
	dir := t.TempDir()
	writeWAV(t, filepath.Join(dir, "success", "success.wav"), tone(1000, -6, 500*time.Millisecond))
	r, err := Pack(dir, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Failed(false) || !r.Failed(true) {
		t.Errorf("warnings-only pack: Failed(false)=%v Failed(true)=%v", r.Failed(false), r.Failed(true))
	}
}
