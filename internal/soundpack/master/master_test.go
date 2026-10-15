package master

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"claudio.click/internal/audio/native"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/soundpack/audit"
	"claudio.click/internal/testutil/wavfixture"
)

func tone(rate int, freq, peakDB float64, d time.Duration) [][2]float64 {
	amp := math.Pow(10, peakDB/20)
	frames := make([][2]float64, int(d.Seconds()*float64(rate)))
	for i := range frames {
		x := amp * math.Sin(2*math.Pi*freq*float64(i)/float64(rate))
		frames[i] = [2]float64{x, x}
	}
	return frames
}

func gap(rate int, d time.Duration) [][2]float64 {
	return make([][2]float64, int(d.Seconds()*float64(rate)))
}

func join(parts ...[][2]float64) [][2]float64 {
	var out [][2]float64
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// decaying returns a tone with an exponential tail, the shape whose quiet
// end moves in and out of "silence" as gain changes.
func decaying(rate int, freq, peakDB float64, d time.Duration) [][2]float64 {
	frames := tone(rate, freq, peakDB, d)
	for i := range frames {
		g := math.Exp(-8 * float64(i) / float64(len(frames)))
		frames[i][0] *= g
		frames[i][1] *= g
	}
	return frames
}

func noise(rate int, peakDB float64, d time.Duration) [][2]float64 {
	rng := rand.New(rand.NewSource(1))
	amp := math.Pow(10, peakDB/20)
	frames := make([][2]float64, int(d.Seconds()*float64(rate)))
	for i := range frames {
		frames[i] = [2]float64{amp * (rng.Float64()*2 - 1), amp * (rng.Float64()*2 - 1)}
	}
	return frames
}

func writeWAV(t *testing.T, path string, rate int, frames [][2]float64) {
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

// The reason this package exists: whatever shape the input is in, the
// output passes the audit with nothing to fix. If a rule is added to the
// audit that mastering does not satisfy, this fails.
func TestMasteredSoundsPassTheAudit(t *testing.T) {
	const r = 48000
	inputs := map[string][][2]float64{
		"quiet":            tone(r, 1000, -45, 400*time.Millisecond),
		"hot":              tone(r, 1000, 0, 400*time.Millisecond),
		"padded":           join(gap(r, 300*time.Millisecond), tone(r, 800, -20, 300*time.Millisecond), gap(r, time.Second)),
		"long":             tone(r, 500, -12, 20*time.Second),
		"blip":             tone(r, 2000, -30, 40*time.Millisecond),
		"loud decay":       decaying(r, 700, 0, 1200*time.Millisecond),
		"quiet decay":      decaying(r, 700, -40, 1200*time.Millisecond),
		"noise":            noise(r, -25, 700*time.Millisecond),
		"low rate":         tone(22050, 900, -10, 500*time.Millisecond),
		"cd rate decaying": decaying(44100, 1200, -3, 900*time.Millisecond),
	}
	rates := map[string]int{"low rate": 22050, "cd rate decaying": 44100}

	for name, frames := range inputs {
		for _, category := range []string{"loading", "completion"} {
			t.Run(name+"/"+category, func(t *testing.T) {
				rate := rates[name]
				if rate == 0 {
					rate = r
				}
				opts := DefaultOptions()
				out, _ := Process(frames, rate, []string{category}, opts)

				var buf bytes.Buffer
				if err := EncodeWAV(&buf, out); err != nil {
					t.Fatal(err)
				}
				decoded, gotRate, err := native.DecodeFrames(context.Background(), "out.wav", &buf)
				if err != nil {
					t.Fatalf("mastered WAV does not decode: %v", err)
				}
				if gotRate != audit.TargetSampleRate {
					t.Fatalf("rate = %d, want %d", gotRate, audit.TargetSampleRate)
				}
				s := audit.Sound{Path: "out.wav", Categories: []string{category}}
				audit.Analyze(&s, decoded, gotRate, opts.Options)
				for _, f := range s.Findings {
					if f.Severity != audit.SeverityInfo {
						t.Errorf("%s: %s (loudness %v, peak %v, %.0f ms, lead %.0f ms, tail %.0f ms)",
							f.Rule, f.Message, s.LoudnessLUFS, s.PeakDBTP, s.DurationMS, s.LeadingSilenceMS, s.TrailingSilenceMS)
					}
				}
			})
		}
	}
}

func TestProcessReportsWhatItDid(t *testing.T) {
	const r = 48000
	in := join(gap(r, 200*time.Millisecond), tone(r, 1000, -40, 5*time.Second), gap(r, 500*time.Millisecond))
	out, res := Process(in, r, []string{"loading"}, DefaultOptions())

	if !res.Truncated {
		t.Error("a 5 s loading sound should be reported as truncated")
	}
	if got := float64(len(out)) / r; math.Abs(got-1.0) > 0.01 {
		t.Errorf("output is %.2f s, want the 1 s loading limit", got)
	}
	if math.Abs(res.TrimmedLeadMS-200) > 2 {
		t.Errorf("trimmed lead = %.0f ms, want 200", res.TrimmedLeadMS)
	}
	if res.GainDB < 15 {
		t.Errorf("gain = %.1f dB, want a large boost for a -40 dBFS tone", res.GainDB)
	}
	if math.Abs(float64(res.After.LoudnessLUFS)+18) > 1 {
		t.Errorf("mastered loudness = %v, want about -18", res.After.LoudnessLUFS)
	}
}

func TestSilentInputIsLeftAlone(t *testing.T) {
	out, res := Process(gap(44100, time.Second), 44100, []string{"success"}, DefaultOptions())
	if len(out) != 0 || !res.Silent {
		t.Errorf("silent input produced %d frames, silent=%v", len(out), res.Silent)
	}
}

func TestEncodeWAVWritesMonoWhenChannelsMatch(t *testing.T) {
	mono := tone(48000, 1000, -6, 100*time.Millisecond)
	stereo := tone(48000, 1000, -6, 100*time.Millisecond)
	for i := range stereo {
		stereo[i][1] = -stereo[i][1]
	}
	var m, s bytes.Buffer
	if err := EncodeWAV(&m, mono); err != nil {
		t.Fatal(err)
	}
	if err := EncodeWAV(&s, stereo); err != nil {
		t.Fatal(err)
	}
	if s.Len() != 2*m.Len()-44 {
		t.Errorf("mono %d bytes, stereo %d bytes: identical channels should be stored once", m.Len(), s.Len())
	}
	got, _, err := native.DecodeFrames(context.Background(), "s.wav", &s)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got {
		if math.Abs(got[i][0]-stereo[i][0]) > 1e-4 || math.Abs(got[i][1]-stereo[i][1]) > 1e-4 {
			t.Fatalf("frame %d = %v, want %v", i, got[i], stereo[i])
		}
	}
}

func TestPackMastersDirectoryPack(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeWAV(t, filepath.Join(src, "loading", "loading.wav"), 44100, tone(44100, 600, -2, 6*time.Second))
	writeWAV(t, filepath.Join(src, "success", "success.wav"), 48000, tone(48000, 1200, -40, 300*time.Millisecond))
	writeWAV(t, filepath.Join(src, "default.wav"), 48000, tone(48000, 900, -12, 300*time.Millisecond))

	results, err := Pack(src, dst, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("mastered %d files, want 3", len(results))
	}
	report, err := audit.Pack(dst, audit.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Files != 3 || report.Failed(true) {
		t.Errorf("mastered pack audit: %+v", report.Summary)
	}

	if _, err := Pack(src, src, DefaultOptions()); err == nil {
		t.Error("mastering a pack onto itself must be refused")
	}
}

func TestPackMastersJSONPackAndRewritesManifest(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeWAV(t, filepath.Join(src, "raw", "chirp.wav"), 48000, tone(48000, 1500, -3, 5*time.Second))
	writeWAV(t, filepath.Join(src, "raw", "thud.wav"), 48000, tone(48000, 200, -30, 400*time.Millisecond))
	manifest := soundpack.JSONSoundpackFile{
		Name: "demo", Description: "d", Version: "1.2.3",
		Mappings: map[string]string{
			"success/success.wav":       "raw/chirp.wav",
			"completion/completion.wav": "raw/chirp.wav",
			"error/error.wav":           "raw/thud.wav",
			"loading/loading.wav":       "",
		},
	}
	data, _ := json.Marshal(manifest)
	srcManifest := filepath.Join(src, "soundpack.json")
	if err := os.WriteFile(srcManifest, data, 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Pack(srcManifest, dst, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("mastered %d files, want 2 (one per source file, not per key)", len(results))
	}

	v, err := soundpack.ValidateJSONSoundpack(filepath.Join(dst, "soundpack.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Err(); err != nil {
		t.Fatalf("mastered manifest is broken: %v", err)
	}
	if v.File.Name != "demo" || v.File.Version != "1.2.3" || len(v.Resolved) != 3 {
		t.Errorf("manifest = %+v, resolved %d", v.File, len(v.Resolved))
	}
	if got := v.File.Mappings["success/success.wav"]; got != "sounds/raw/chirp.wav" {
		t.Errorf("success mapping = %q", got)
	}

	report, err := audit.Pack(filepath.Join(dst, "soundpack.json"), audit.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	if report.Failed(true) {
		t.Errorf("mastered JSON pack audit: %+v %+v", report.Summary, report.Sounds)
	}
	// chirp answers for success (1.5 s limit) and completion (3 s): the
	// strictest applies.
	for _, s := range report.Sounds {
		if s.Path == "sounds/raw/chirp.wav" && math.Abs(s.DurationMS-1500) > 20 {
			t.Errorf("chirp is %.0f ms, want 1500", s.DurationMS)
		}
	}
}
