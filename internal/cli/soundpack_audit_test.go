package cli

import (
	"bytes"
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
