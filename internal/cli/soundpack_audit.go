package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"claudio.click/internal/soundpack/audit"
	"claudio.click/internal/soundpack/master"
)

// auditFlags are the thresholds shared by `soundpack audit` and
// `soundpack master`, so a pack is always mastered to the standard it is
// audited against.
type auditFlags struct {
	targetLUFS  float64
	tolerance   float64
	peakCeiling float64
	maxDuration map[string]string
}

func addAuditFlags(cmd *cobra.Command) *auditFlags {
	d := audit.DefaultOptions()
	f := &auditFlags{}
	cmd.Flags().Float64Var(&f.targetLUFS, "target-lufs", d.TargetLUFS, "loudness every sound should have, in LUFS")
	cmd.Flags().Float64Var(&f.tolerance, "tolerance", d.ToleranceLU, "allowed distance from the target, in LU")
	cmd.Flags().Float64Var(&f.peakCeiling, "peak-ceiling", d.PeakCeilingDBTP, "highest allowed true peak, in dBTP")
	cmd.Flags().StringToStringVar(&f.maxDuration, "max-duration", nil,
		"override duration limits per category, e.g. loading=1s,system=8s")
	return f
}

func (f *auditFlags) options() (audit.Options, error) {
	o := audit.DefaultOptions()
	o.TargetLUFS, o.ToleranceLU, o.PeakCeilingDBTP = f.targetLUFS, f.tolerance, f.peakCeiling
	for category, value := range f.maxDuration {
		if _, known := o.MaxDuration[category]; !known {
			return o, fmt.Errorf("unknown category %q in --max-duration (known: %s)", category, strings.Join(durationCategories(o), ", "))
		}
		d, err := time.ParseDuration(value)
		if err != nil || d <= 0 {
			return o, fmt.Errorf("invalid duration %q for %s in --max-duration", value, category)
		}
		o.MaxDuration[category] = d
	}
	return o, nil
}

func durationCategories(o audit.Options) []string {
	names := make([]string, 0, len(o.MaxDuration))
	for name := range o.MaxDuration {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func durationLimitsHelp() string {
	o := audit.DefaultOptions()
	var b strings.Builder
	for _, name := range durationCategories(o) {
		fmt.Fprintf(&b, "  %-12s %s\n", name, o.MaxDuration[name])
	}
	return b.String()
}

// newSoundpackAuditCommand creates the soundpack audit subcommand
func newSoundpackAuditCommand() *cobra.Command {
	var jsonOutput, strict bool
	cmd := &cobra.Command{
		Use:   "audit <path>",
		Short: "Measure every sound in a soundpack and report what is wrong with it",
		Long: `Decode every sound in a directory or JSON soundpack with the same decoders
playback uses, measure it, and report the problems a listener would notice.

Measured per sound: duration, loudness (LUFS, ITU-R BS.1770), true peak,
silence before and after, clipping, and a short description (number of
onsets, dominant pitch, whether it rises or falls, tonal or noisy) so the
sounds can be told apart without hearing them.

Warnings:
  too-long            longer than its category allows (limits below)
  too-loud/too-quiet  outside the loudness target
  peak-over-ceiling   true peak above the ceiling
  clipping            runs of full-scale samples
  leading-silence     silence before the sound, which makes it feel late
  trailing-silence    silence after the sound
Errors:
  undecodable         claudio cannot play the file
  silent              nothing above -60 dBFS

Default duration limits:
` + durationLimitsHelp() + `
Exit code is non-zero on any error, and with --strict on any warning.
'claudio soundpack master' fixes everything audit warns about except
clipping that is already in the source.

Examples:
  claudio soundpack audit ./my-pack
  claudio soundpack audit ./my-pack/soundpack.json --strict
  claudio soundpack audit ./my-pack --json`,
		Args: cobra.ExactArgs(1),
	}
	flags := addAuditFlags(cmd)
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print the full report as JSON")
	cmd.Flags().BoolVar(&strict, "strict", false, "fail on warnings as well as errors")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		opts, err := flags.options()
		if err != nil {
			return err
		}
		report, err := audit.Pack(args[0], opts)
		if err != nil {
			return err
		}
		if jsonOutput {
			if err := printJSON(cmd, report); err != nil {
				return err
			}
		} else {
			printAuditReport(cmd, report)
		}
		if report.Failed(strict) {
			return fmt.Errorf("audit failed: %d error(s), %d warning(s)", report.Summary.Errors, report.Summary.Warnings)
		}
		return nil
	}
	return cmd
}

func printJSON(cmd *cobra.Command, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	cmd.Println(string(data))
	return nil
}

func printAuditReport(cmd *cobra.Command, report audit.Report) {
	cmd.Printf("Auditing: %s (%s)\n\n", report.Pack, report.Type)

	width := len("SOUND")
	for _, s := range report.Sounds {
		width = max(width, len(s.Path))
	}
	cmd.Printf("%-*s  %7s  %6s  %6s  %3s  %7s  %-7s  %-6s  %s\n", width,
		"SOUND", "LENGTH", "LUFS", "PEAK", "HIT", "PITCH", "TREND", "TEXTURE", "FINDINGS")
	for _, s := range report.Sounds {
		rules := make([]string, 0, len(s.Findings))
		for _, f := range s.Findings {
			rules = append(rules, f.Rule)
		}
		cmd.Printf("%-*s  %6.2fs  %6s  %6s  %3d  %5.0fHz  %-7s  %-6s  %s\n", width,
			s.Path, s.DurationMS/1000, s.LoudnessLUFS, s.PeakDBTP, s.Onsets, s.PitchHz,
			s.PitchTrend, s.Texture, strings.Join(rules, ", "))
	}

	for _, severity := range []string{audit.SeverityError, audit.SeverityWarning} {
		printed := false
		for _, s := range report.Sounds {
			for _, f := range s.Findings {
				if f.Severity != severity {
					continue
				}
				if !printed {
					cmd.Printf("\n%ss:\n", strings.ToUpper(severity[:1])+severity[1:])
					printed = true
				}
				cmd.Printf("  %s: %s: %s\n", s.Path, f.Rule, f.Message)
			}
		}
	}

	sum := report.Summary
	cmd.Printf("\nSummary: %d files, %.1f MB, longest %.2f s\n", sum.Files, float64(sum.Bytes)/(1<<20), sum.LongestMS/1000)
	cmd.Printf("  Loudness: %s to %s LUFS (spread %.1f LU)\n", sum.LoudnessMinLUFS, sum.LoudnessMaxLUFS, sum.LoudnessSpread)
	cmd.Printf("  %d error(s), %d warning(s), %d note(s)\n", sum.Errors, sum.Warnings, sum.Infos)
}

// newSoundpackMasterCommand creates the soundpack master subcommand
func newSoundpackMasterCommand() *cobra.Command {
	var jsonOutput bool
	var outDir string
	cmd := &cobra.Command{
		Use:   "master <path> --out <dir>",
		Short: "Rewrite a soundpack's sounds so they pass the audit",
		Long: `Write a mastered copy of a directory or JSON soundpack into --out. The
source is never modified.

Each sound has the silence trimmed from both ends, is cut (with a fade) to
its category's duration limit, and is brought to the loudness target under
the true-peak ceiling. Output is 48 kHz 16-bit WAV, mono when both channels
are identical.

A directory pack keeps its layout. A JSON pack is written as
<out>/soundpack.json with its sounds under <out>/sounds/, each source file
mastered once however many keys map to it. That makes this layout work for
a pack repository: keep the raw material and a manifest over it in source/,
and build the published pack at the root:

  claudio soundpack master source/soundpack.json --out .
  claudio soundpack audit soundpack.json --strict

Sounds reported as "truncated" were longer than their limit and lost their
end. Check that the part kept is the part you wanted, or cut the source
yourself.

Examples:
  claudio soundpack master ./raw-pack --out ./my-pack
  claudio soundpack master source/soundpack.json --out . --target-lufs -20`,
		Args: cobra.ExactArgs(1),
	}
	flags := addAuditFlags(cmd)
	cmd.Flags().StringVar(&outDir, "out", "", "directory to write the mastered pack into (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print what was done to each sound as JSON")
	_ = cmd.MarkFlagRequired("out")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		auditOpts, err := flags.options()
		if err != nil {
			return err
		}
		opts := master.DefaultOptions()
		opts.Options = auditOpts
		results, err := master.Pack(args[0], outDir, opts)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(cmd, results)
		}
		printMasterReport(cmd, results, outDir)
		return nil
	}
	return cmd
}

func printMasterReport(cmd *cobra.Command, results []master.Result, outDir string) {
	width := len("SOURCE")
	for _, r := range results {
		width = max(width, len(r.Source))
	}
	cmd.Printf("%-*s  %15s  %15s  %7s  %s\n", width, "SOURCE", "LENGTH", "LUFS", "GAIN", "NOTES")
	truncated := 0
	var before, after int64
	for _, r := range results {
		var notes []string
		if r.Truncated {
			notes = append(notes, "truncated")
			truncated++
		}
		if r.LimitedDB > 0 {
			notes = append(notes, fmt.Sprintf("peaks limited %.1f dB", r.LimitedDB))
		}
		if r.TrimmedLeadMS >= 1 {
			notes = append(notes, fmt.Sprintf("-%.0f ms lead", r.TrimmedLeadMS))
		}
		for _, f := range r.After.Findings {
			notes = append(notes, f.Rule)
		}
		before += r.Before.Bytes
		after += r.After.Bytes
		cmd.Printf("%-*s  %6.2fs>%6.2fs  %6s > %6s  %+6.1f  %s\n", width, r.Source,
			r.Before.DurationMS/1000, r.After.DurationMS/1000,
			r.Before.LoudnessLUFS, r.After.LoudnessLUFS, r.GainDB, strings.Join(notes, ", "))
	}
	cmd.Printf("\nMastered %d sounds into %s (%.1f MB -> %.1f MB)\n", len(results), outDir,
		float64(before)/(1<<20), float64(after)/(1<<20))
	if truncated > 0 {
		cmd.Printf("%d sound(s) were truncated to their category limit; check that the kept part is the right part.\n", truncated)
	}
}
