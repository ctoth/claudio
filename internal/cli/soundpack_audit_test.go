package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"claudio.click/internal/soundpack/audit"
	"claudio.click/internal/soundpack/master"
	"claudio.click/internal/testutil/wavfixture"
)

// writeTone writes a 48 kHz stereo WAV tone of the given length and
// amplitude (0..1) with silence in front.
func writeTone(t *testing.T, path string, milliseconds, leadMS int, amplitude float64) {
	t.Helper()
	frames := make([][]float64, 48000*leadMS/1000)
	for i := range frames {
		frames[i] = []float64{0, 0}
	}
	frames = append(frames, wavfixture.ToneFrames(48000, 2, milliseconds, amplitude)...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, wavfixture.WAV(wavfixture.TagPCM, 16, 48000, frames), 0o644); err != nil {
		t.Fatal(err)
	}
}

func runCommand(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	err := cmd.Execute()
	return out.String(), err
}

// roughPack is a directory pack with the usual problems: a hot sound, a
// quiet one behind silence, and a loading sound that goes on for seconds.
func roughPack(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "rough")
	writeTone(t, filepath.Join(dir, "success", "success.wav"), 400, 0, 0.9)
	writeTone(t, filepath.Join(dir, "error", "error.wav"), 400, 250, 0.01)
	writeTone(t, filepath.Join(dir, "loading", "loading.wav"), 6000, 0, 0.3)
	return dir
}

func TestSoundpackAuditReportsProblems(t *testing.T) {
	dir := roughPack(t)

	out, err := runCommand(newSoundpackAuditCommand(), dir)
	if err != nil {
		t.Fatalf("warnings alone must not fail without --strict: %v\n%s", err, out)
	}
	for _, want := range []string{"success/success.wav", audit.RuleTooLoud, audit.RuleTooQuiet,
		audit.RuleLeadingSilence, audit.RuleTooLong, "3 files"} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not mention %q:\n%s", want, out)
		}
	}

	if _, err := runCommand(newSoundpackAuditCommand(), dir, "--strict"); err == nil {
		t.Error("--strict must fail a pack with warnings")
	}
}

func TestSoundpackAuditJSON(t *testing.T) {
	out, err := runCommand(newSoundpackAuditCommand(), roughPack(t), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var report audit.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("output is not a JSON report: %v\n%s", err, out)
	}
	if report.Summary.Files != 3 || report.Summary.Warnings == 0 {
		t.Errorf("summary = %+v", report.Summary)
	}
}

func TestSoundpackAuditRejectsInvalidManifestMappings(t *testing.T) {
	for _, tc := range []struct {
		name, value, want string
		withPlayable      bool
	}{
		{"only missing", "missing.wav", "broken reference", false},
		{"missing beside playable", "missing.wav", "broken reference", true},
		{"unsafe path", "../outside.wav", "unsafe path", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mappings := map[string]string{"default.wav": tc.value}
			if tc.withPlayable {
				writeTone(t, filepath.Join(dir, "good.wav"), 400, 0, 0.126)
				mappings["success/success.wav"] = "good.wav"
			}
			data, err := json.Marshal(map[string]any{"name": "incomplete", "mappings": mappings})
			if err != nil {
				t.Fatal(err)
			}
			manifest := filepath.Join(dir, "soundpack.json")
			if err := os.WriteFile(manifest, data, 0o644); err != nil {
				t.Fatal(err)
			}

			for _, flags := range [][]string{nil, {"--strict"}} {
				out, err := runCommand(newSoundpackAuditCommand(), append([]string{manifest}, flags...)...)
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Errorf("flags %v: error = %v, want %q; output:\n%s", flags, err, tc.want, out)
				}
			}
		})
	}
}

func TestSoundpackAuditRejectsMappedOgg(t *testing.T) {
	// A valid 100 ms, 48 kHz mono Vorbis tone, independently decoded by ffmpeg.
	// Generated with: ffmpeg -f lavfi -i sine=frequency=440:sample_rate=48000:duration=0.1
	// -c:a libvorbis -f ogg pipe:1, then base64-encoded for the fixture.
	encoded, err := os.ReadFile("testdata/tone.ogg.base64")
	if err != nil {
		t.Fatal(err)
	}
	ogg, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		mixed bool
	}{{"Ogg only", false}, {"Ogg and WAV", true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "sounds"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "sounds", "tone.ogg"), ogg, 0o644); err != nil {
				t.Fatal(err)
			}
			mappings := map[string]string{"default.wav": "sounds/tone.ogg", "error/error.wav": "sounds/tone.ogg"}
			wantFiles := 1
			if tc.mixed {
				writeTone(t, filepath.Join(dir, "sounds", "good.wav"), 400, 0, 0.126)
				mappings["success/success.wav"] = "sounds/good.wav"
				wantFiles++
			}
			manifest := filepath.Join(dir, "soundpack.json")
			data, err := json.Marshal(map[string]any{"name": "mixed formats", "mappings": mappings})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, data, 0o644); err != nil {
				t.Fatal(err)
			}
			for _, flags := range [][]string{{"--json"}, {"--json", "--strict"}} {
				out, err := runCommand(newSoundpackAuditCommand(), append([]string{manifest}, flags...)...)
				var report audit.Report
				if jsonErr := json.Unmarshal([]byte(out), &report); jsonErr != nil {
					t.Fatalf("invalid audit report: %v\n%s", jsonErr, out)
				}
				if err == nil || report.Summary.Errors != 1 || report.Summary.Files != wantFiles {
					t.Errorf("flags %v: error = %v, summary = %+v; want %d files and one decode error", flags, err, report.Summary, wantFiles)
				}
				found := false
				for _, sound := range report.Sounds {
					if sound.Path != "sounds/tone.ogg" {
						continue
					}
					found = true
					if len(sound.Keys) != 2 || sound.SampleRate != 0 || sound.DurationMS != 0 {
						t.Errorf("unsupported sound must retain both keys without invented measurements: %+v", sound)
					}
					if len(sound.Findings) != 1 || sound.Findings[0].Rule != audit.RuleUndecodable ||
						sound.Findings[0].Severity != audit.SeverityError ||
						!strings.Contains(sound.Findings[0].Message, "unsupported audio format") {
						t.Errorf("want precise unsupported-format finding: %+v", sound.Findings)
					}
				}
				if !found {
					t.Error("mapped Ogg is absent from the report")
				}
			}
			out, err := runCommand(newSoundpackAuditCommand(), manifest, "--strict")
			if err == nil || !strings.Contains(out, "sounds/tone.ogg: undecodable:") || !strings.Contains(out, "unsupported audio format") {
				t.Errorf("text report must explain why the mapped Ogg fails: %v\n%s", err, out)
			}
		})
	}
}

func TestSoundpackAuditDecodesMappedContentByMagic(t *testing.T) {
	dir := t.TempDir()
	writeTone(t, filepath.Join(dir, "tone.bin"), 400, 0, 0.126)
	manifest := filepath.Join(dir, "soundpack.json")
	if err := os.WriteFile(manifest, []byte(`{"name":"mislabelled WAV","mappings":{"default.wav":"tone.bin"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := runCommand(newSoundpackAuditCommand(), manifest, "--strict", "--json")
	if err != nil {
		t.Fatalf("playable content must be decoded regardless of extension: %v\n%s", err, out)
	}
	var report audit.Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Files != 1 || len(report.Sounds) != 1 {
		t.Fatalf("mapped WAV was skipped: %+v", report)
	}
	if sound := report.Sounds[0]; sound.SampleRate != 48000 || sound.DurationMS != 400 {
		t.Errorf("mapped WAV was not measured: %+v", sound)
	}
}

func TestSoundpackAuditThresholdFlags(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "p")
	writeTone(t, filepath.Join(dir, "loading", "loading.wav"), 3000, 0, 0.126) // about -18 dBFS

	if _, err := runCommand(newSoundpackAuditCommand(), dir, "--strict"); err == nil {
		t.Error("a 3 s loading sound should fail the default limits")
	}
	if out, err := runCommand(newSoundpackAuditCommand(), dir, "--strict", "--max-duration", "loading=5s"); err != nil {
		t.Errorf("raising the loading limit should pass: %v\n%s", err, out)
	}
	if _, err := runCommand(newSoundpackAuditCommand(), dir, "--max-duration", "loading=soon"); err == nil {
		t.Error("an unparseable duration must be rejected")
	}
	if _, err := runCommand(newSoundpackAuditCommand(), dir, "--max-duration", "bogus=1s"); err == nil {
		t.Error("an unknown category must be rejected")
	}
}

func TestSoundpackMasterProducesAPackThatPassesStrictAudit(t *testing.T) {
	dir := roughPack(t)
	out := filepath.Join(t.TempDir(), "mastered")

	report, err := runCommand(newSoundpackMasterCommand(), dir, "--out", out)
	if err != nil {
		t.Fatalf("master failed: %v\n%s", err, report)
	}
	if !strings.Contains(report, "truncated") || !strings.Contains(report, "loading/loading.wav") {
		t.Errorf("master report should name the truncated loading sound:\n%s", report)
	}
	if audited, err := runCommand(newSoundpackAuditCommand(), out, "--strict"); err != nil {
		t.Errorf("mastered pack fails strict audit: %v\n%s", err, audited)
	}

	jsonOut, err := runCommand(newSoundpackMasterCommand(), dir, "--out", out, "--json")
	if err != nil {
		t.Fatal(err)
	}
	var results []master.Result
	if err := json.Unmarshal([]byte(jsonOut), &results); err != nil || len(results) != 3 {
		t.Errorf("--json output: %v (%d results)\n%s", err, len(results), jsonOut)
	}
}

func TestSoundpackMasterRequiresOut(t *testing.T) {
	if _, err := runCommand(newSoundpackMasterCommand(), roughPack(t)); err == nil {
		t.Error("master without --out must fail")
	}
}
