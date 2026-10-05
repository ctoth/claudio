package synth

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/loudness"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/soundpack/audit"
	"claudio.click/internal/soundpack/master"
)

func parse(t *testing.T, recipe string) *Recipe {
	t.Helper()
	r, err := Parse([]byte(recipe))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return r
}

// render parses a one-sound recipe and renders that sound.
func render(t *testing.T, sound string) [][2]float64 {
	t.Helper()
	r := parse(t, `{"name":"t","sounds":{"s":`+sound+`},"mappings":{"default.wav":"s"}}`)
	frames, err := r.Render("s")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return frames
}

func describe(frames [][2]float64) audit.Description {
	start, end := loudness.ActiveRange(frames, -60)
	return audit.Describe(frames[start:end], SampleRate)
}

func seconds(frames [][2]float64) float64 {
	return float64(len(frames)) / SampleRate
}

func TestPitchAcceptsHertzAndNoteNames(t *testing.T) {
	for in, want := range map[string]float64{
		`440`: 440, `"A4"`: 440, `"a4"`: 440, `"C5"`: 523.251, `"C#5"`: 554.365,
		`"Db5"`: 554.365, `"Bb3"`: 233.082, `"A0"`: 27.5, `0`: 0,
	} {
		var p Pitch
		if err := json.Unmarshal([]byte(in), &p); err != nil {
			t.Errorf("%s: %v", in, err)
			continue
		}
		if math.Abs(float64(p)-want) > 0.01 {
			t.Errorf("%s = %.3f Hz, want %.3f", in, float64(p), want)
		}
	}
	for _, bad := range []string{`"H4"`, `"C"`, `"4C"`, `true`, `-5`} {
		var p Pitch
		if err := json.Unmarshal([]byte(bad), &p); err == nil {
			t.Errorf("%s parsed as %v, want an error", bad, p)
		}
	}
}

func TestSineLayerHasItsPitchAndLength(t *testing.T) {
	frames := render(t, `{"layers":[{"freq":1000,"dur":0.3,"release":0.1}]}`)
	if got := seconds(frames); math.Abs(got-0.4) > 0.01 {
		t.Errorf("length = %.3f s, want dur + release = 0.4", got)
	}
	d := describe(frames)
	if math.Abs(d.DominantHz-1000) > 30 || d.Texture != "tonal" || d.Onsets != 1 {
		t.Errorf("described as %+v", d)
	}
}

func TestEveryWaveSoundsAtItsFundamental(t *testing.T) {
	for _, wave := range []string{"sine", "triangle", "square", "saw", "pulse"} {
		frames := render(t, `{"layers":[{"wave":"`+wave+`","freq":440,"dur":0.4}]}`)
		if d := describe(frames); math.Abs(d.DominantHz-440) > 30 {
			t.Errorf("%s: dominant %v Hz, want 440", wave, d.DominantHz)
		}
	}
}

func TestGlideRisesAndFalls(t *testing.T) {
	up := describe(render(t, `{"layers":[{"freq":400,"freq_end":1600,"dur":0.4}]}`))
	down := describe(render(t, `{"layers":[{"freq":1600,"freq_end":400,"dur":0.4}]}`))
	if up.PitchTrend != "rising" || down.PitchTrend != "falling" {
		t.Errorf("glides described as %s and %s", up.PitchTrend, down.PitchTrend)
	}
}

func TestNotesPlayInSequence(t *testing.T) {
	frames := render(t, `{"layers":[{"notes":["C5","E5","G5"],"step":0.15,"dur":0.06,"release":0.03}]}`)
	d := describe(frames)
	if d.Onsets != 3 || d.PitchTrend != "rising" {
		t.Errorf("arpeggio described as %+v", d)
	}
	if got := seconds(frames); math.Abs(got-0.39) > 0.01 {
		t.Errorf("length = %.3f s, want 2 steps + dur + release = 0.39", got)
	}

	// A zero is a rest.
	rest := describe(render(t, `{"layers":[{"notes":["C5",0,"G5"],"step":0.15,"dur":0.06,"release":0.03}]}`))
	if rest.Onsets != 2 {
		t.Errorf("sequence with a rest has %d onsets, want 2", rest.Onsets)
	}
}

func TestNoiseIsNoisyAndReproducible(t *testing.T) {
	const sound = `{"layers":[{"wave":"noise","dur":0.3}]}`
	a, b := render(t, sound), render(t, sound)
	if d := describe(a); d.Texture != "noisy" {
		t.Errorf("noise described as %s (flatness %v)", d.Texture, d.Flatness)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("frame %d differs between two renders of the same recipe", i)
		}
	}
}

func TestLowpassDarkensNoise(t *testing.T) {
	bright := describe(render(t, `{"layers":[{"wave":"noise","dur":0.3}]}`))
	dark := describe(render(t, `{"layers":[{"wave":"noise","dur":0.3,"lowpass":500}]}`))
	if dark.CentroidHz > bright.CentroidHz/3 {
		t.Errorf("lowpass at 500 Hz: centroid %v Hz vs unfiltered %v Hz", dark.CentroidHz, bright.CentroidHz)
	}
	sweep := describe(render(t, `{"layers":[{"wave":"noise","dur":0.4,"lowpass":300,"lowpass_end":8000}]}`))
	if sweep.PitchTrend != "rising" {
		t.Errorf("opening filter sweep described as %s", sweep.PitchTrend)
	}
}

func TestDecayMakesAPluck(t *testing.T) {
	held := render(t, `{"layers":[{"freq":800,"dur":0.5}]}`)
	pluck := render(t, `{"layers":[{"freq":800,"dur":0.5,"decay":0.1,"sustain":0}]}`)
	energy := func(frames [][2]float64) (e float64) {
		for _, f := range frames[len(frames)/2:] {
			e += f[0] * f[0]
		}
		return e
	}
	if energy(pluck) > energy(held)/100 {
		t.Error("a 100 ms decay to zero should leave the second half nearly silent")
	}
}

func TestFMAddsSidebands(t *testing.T) {
	plain := describe(render(t, `{"layers":[{"freq":400,"dur":0.4}]}`))
	bell := describe(render(t, `{"layers":[{"freq":400,"dur":0.4,"fm":{"ratio":3.5,"index":4}}]}`))
	if bell.CentroidHz < plain.CentroidHz*1.5 {
		t.Errorf("FM centroid %v Hz is not brighter than the plain sine's %v Hz", bell.CentroidHz, plain.CentroidHz)
	}
}

func TestReverbAndDelayAddATail(t *testing.T) {
	const layer = `"layers":[{"freq":900,"dur":0.1,"release":0.02}]`
	dry := seconds(render(t, `{`+layer+`}`))
	wet := seconds(render(t, `{`+layer+`,"effects":[{"type":"reverb","size":0.8,"mix":0.4}]}`))
	echo := seconds(render(t, `{`+layer+`,"effects":[{"type":"delay","time":0.2,"feedback":0.4,"mix":0.5}]}`))
	if wet < dry+0.3 || echo < dry+0.3 {
		t.Errorf("dry %.2f s, reverb %.2f s, delay %.2f s: effects should ring on", dry, wet, echo)
	}
}

func TestOutputIsFiniteAndBelowFullScale(t *testing.T) {
	frames := render(t, `{"layers":[
		{"wave":"saw","freq":110,"dur":0.5,"voices":5,"detune":25,"gain":12,"lowpass":4000,"lowpass_end":200,"q":8},
		{"wave":"noise","dur":0.5,"highpass":3000,"gain":6},
		{"wave":"square","freq":"A5","dur":0.5,"vibrato":{"rate":6,"depth":0.5},"tremolo":{"rate":9,"depth":0.8},"pan":-1}],
		"effects":[{"type":"drive","amount":0.8},{"type":"bitcrush","bits":6,"downsample":4},
			{"type":"lowpass","freq":6000},{"type":"highpass","freq":80},
			{"type":"delay","time":0.1,"feedback":0.5,"mix":0.3},{"type":"reverb","size":0.6,"mix":0.3}]}`)
	peak := 0.0
	for i, f := range frames {
		for _, x := range f {
			if math.IsNaN(x) || math.IsInf(x, 0) {
				t.Fatalf("frame %d is not finite", i)
			}
			peak = math.Max(peak, math.Abs(x))
		}
	}
	if peak > 0.75 || peak < 0.65 {
		t.Errorf("peak = %.3f, want normalized to about -3 dBFS (0.708)", peak)
	}
}

func TestParseRejectsMistakesWithTheirLocation(t *testing.T) {
	wrap := func(sound string) string {
		return `{"name":"t","sounds":{"zap":` + sound + `},"mappings":{"default.wav":"zap"}}`
	}
	for name, tc := range map[string]struct{ recipe, want string }{
		"misspelled field":  {wrap(`{"layers":[{"freq":440,"dur":0.1,"atack":0.1}]}`), "atack"},
		"unknown wave":      {wrap(`{"layers":[{"wave":"sqaure","freq":440,"dur":0.1}]}`), `zap, layer 1: unknown wave "sqaure"`},
		"missing duration":  {wrap(`{"layers":[{"freq":440}]}`), "zap, layer 1: dur"},
		"missing pitch":     {wrap(`{"layers":[{"dur":0.2}]}`), "zap, layer 1: freq"},
		"above nyquist":     {wrap(`{"layers":[{"freq":30000,"dur":0.2}]}`), "zap, layer 1: freq"},
		"no layers":         {wrap(`{"layers":[]}`), "zap: no layers"},
		"unknown effect":    {wrap(`{"layers":[{"freq":440,"dur":0.1}],"effects":[{"type":"flange"}]}`), `zap, effect 1: unknown type "flange"`},
		"too long":          {wrap(`{"layers":[{"freq":440,"dur":60}]}`), "zap"},
		"unmapped sound":    {`{"name":"t","sounds":{"a":{"layers":[{"freq":440,"dur":0.1}]},"b":{"layers":[{"freq":440,"dur":0.1}]}},"mappings":{"default.wav":"a"}}`, `sound "b" is not used by any mapping`},
		"undefined mapping": {`{"name":"t","sounds":{"a":{"layers":[{"freq":440,"dur":0.1}]}},"mappings":{"default.wav":"nope"}}`, `default.wav: no sound named "nope"`},
		"no name":           {`{"sounds":{"a":{"layers":[{"freq":440,"dur":0.1}]}},"mappings":{"default.wav":"a"}}`, "name"},
		"bad sound name":    {`{"name":"t","sounds":{"../a":{"layers":[{"freq":440,"dur":0.1}]}},"mappings":{"default.wav":"../a"}}`, "../a"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse([]byte(tc.recipe))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// The whole chain an author runs: recipe -> raw pack -> mastered pack that
// passes the strict audit.
func TestWrittenPackMastersAndAuditsClean(t *testing.T) {
	r := parse(t, `{
		"name": "demo", "description": "two sounds", "version": "0.1.0",
		"sounds": {
			"up":   {"layers":[{"notes":["C5","G5"],"step":0.09,"dur":0.08,"release":0.1,"wave":"triangle"}]},
			"thud": {"layers":[{"freq":180,"freq_end":60,"dur":0.2,"decay":0.15,"sustain":0},
			                   {"wave":"noise","dur":0.05,"lowpass":900,"gain":-6}],
			         "effects":[{"type":"reverb","size":0.3,"mix":0.15}]}
		},
		"mappings": {
			"default.wav": "up", "success/success.wav": "up",
			"error/error.wav": "thud", "loading/loading.wav": "thud"
		}}`)

	source, built := t.TempDir(), t.TempDir()
	written, err := r.Write(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 2 {
		t.Fatalf("wrote %d sounds, want 2", len(written))
	}
	if _, err := os.Stat(filepath.Join(source, "synth", "thud.wav")); err != nil {
		t.Fatal(err)
	}

	v, err := soundpack.ValidateJSONSoundpack(filepath.Join(source, "soundpack.json"))
	if err != nil || v.Err() != nil {
		t.Fatalf("written manifest: %v / %v", err, v.Err())
	}
	if v.File.Name != "demo" || v.File.Version != "0.1.0" || len(v.Resolved) != 4 {
		t.Errorf("manifest = %+v with %d resolved", v.File, len(v.Resolved))
	}

	if _, err := master.Pack(filepath.Join(source, "soundpack.json"), built, master.DefaultOptions()); err != nil {
		t.Fatal(err)
	}
	report, err := audit.Pack(filepath.Join(built, "soundpack.json"), audit.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Files != 2 || report.Failed(true) {
		t.Errorf("built pack: %+v %+v", report.Summary, report.Sounds)
	}
}
