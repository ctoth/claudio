// Package audit measures every sound in a soundpack and reports what a
// listener would notice: sounds that are too long for how often they fire,
// louder or quieter than the rest, clipped, or padded with silence. Each
// sound also gets a short numeric description (onsets, pitch, trend) so a
// reader who cannot hear the pack can still tell the sounds apart.
package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claudio.click/internal/audio/native"
	"claudio.click/internal/loudness"
	"claudio.click/internal/soundpack"
)

// Rule identifiers, stable for scripts and CI.
const (
	RuleUndecodable     = "undecodable"
	RuleSilent          = "silent"
	RuleTooLong         = "too-long"
	RuleTooLoud         = "too-loud"
	RuleTooQuiet        = "too-quiet"
	RulePeak            = "peak-over-ceiling"
	RuleClipping        = "clipping"
	RuleLeadingSilence  = "leading-silence"
	RuleTrailingSilence = "trailing-silence"
	RulePeakLimited     = "peak-limited"
	RuleDuplicate       = "duplicate"
	RuleSampleRate      = "sample-rate"
	RuleUnreferenced    = "unreferenced"
)

// Severities. Errors always fail an audit; warnings fail it under --strict;
// infos never do.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// RootCategory is the category of files outside any category directory,
// such as default.wav.
const RootCategory = "default"

// TargetSampleRate is the rate the player's output runs at. Other rates
// work but are resampled on every play.
const TargetSampleRate = 48000

// Options are the audit thresholds.
type Options struct {
	TargetLUFS         float64
	ToleranceLU        float64
	PeakCeilingDBTP    float64
	SilenceThresholdDB float64
	MaxLeadingSilence  time.Duration
	MaxTrailingSilence time.Duration
	// MaxDuration is the longest allowed sound per category.
	MaxDuration map[string]time.Duration
}

// DefaultOptions returns the house standard. Durations follow how often a
// category fires. A loading sound plays before every tool call and most
// tools finish within a second, so a longer one is still playing when the
// result sound starts. Completion and system sounds fire a few times per
// session and can take their time.
func DefaultOptions() Options {
	return Options{
		TargetLUFS:         -18,
		ToleranceLU:        1,
		PeakCeilingDBTP:    -1,
		SilenceThresholdDB: -60,
		MaxLeadingSilence:  15 * time.Millisecond,
		MaxTrailingSilence: 150 * time.Millisecond,
		MaxDuration: map[string]time.Duration{
			"loading":     time.Second,
			"success":     1500 * time.Millisecond,
			"error":       2 * time.Second,
			"interactive": 2500 * time.Millisecond,
			"completion":  3 * time.Second,
			"system":      6 * time.Second,
			RootCategory:  1500 * time.Millisecond,
		},
	}
}

// MaxDurationFor returns the strictest duration limit among categories, or
// zero when none of them has one.
func (o Options) MaxDurationFor(categories []string) time.Duration {
	var limit time.Duration
	for _, c := range categories {
		if d, ok := o.MaxDuration[c]; ok && (limit == 0 || d < limit) {
			limit = d
		}
	}
	return limit
}

// Level is a level in dB. Silence is negative infinity, which JSON cannot
// carry, so it marshals as null.
type Level float64

func (l Level) MarshalJSON() ([]byte, error) {
	if math.IsInf(float64(l), 0) || math.IsNaN(float64(l)) {
		return []byte("null"), nil
	}
	return json.Marshal(math.Round(float64(l)*100) / 100)
}

func (l Level) String() string {
	if math.IsInf(float64(l), 0) || math.IsNaN(float64(l)) {
		return "-inf"
	}
	return fmt.Sprintf("%.1f", float64(l))
}

// Finding is one thing wrong (or worth knowing) about a sound.
type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Sound is the audit of one audio file.
type Sound struct {
	// Path is slash-separated and relative to the pack root.
	Path string `json:"path"`
	// Keys are the sound keys this file answers for.
	Keys       []string `json:"keys"`
	Categories []string `json:"categories"`
	Bytes      int64    `json:"bytes"`
	SHA256     string   `json:"sha256,omitempty"`

	SampleRate int     `json:"sample_rate"`
	Mono       bool    `json:"mono"`
	DurationMS float64 `json:"duration_ms"`

	LoudnessLUFS     Level `json:"loudness_lufs"`
	IntegratedLUFS   Level `json:"integrated_lufs"`
	MomentaryMaxLUFS Level `json:"momentary_max_lufs"`
	PeakDBTP         Level `json:"peak_dbtp"`

	LeadingSilenceMS  float64 `json:"leading_silence_ms"`
	TrailingSilenceMS float64 `json:"trailing_silence_ms"`
	ClippedRuns       int     `json:"clipped_runs"`

	Description

	DuplicateOf string    `json:"duplicate_of,omitempty"`
	Findings    []Finding `json:"findings"`
}

func (s *Sound) add(rule, severity, format string, args ...any) {
	s.Findings = append(s.Findings, Finding{Rule: rule, Severity: severity, Message: fmt.Sprintf(format, args...)})
}

// Summary totals a report.
type Summary struct {
	Files    int   `json:"files"`
	Bytes    int64 `json:"bytes"`
	Errors   int   `json:"errors"`
	Warnings int   `json:"warnings"`
	Infos    int   `json:"infos"`
	// LoudnessMin/Max cover the sounds that have a level.
	LoudnessMinLUFS Level   `json:"loudness_min_lufs"`
	LoudnessMaxLUFS Level   `json:"loudness_max_lufs"`
	LoudnessSpread  float64 `json:"loudness_spread_lu"`
	LongestMS       float64 `json:"longest_ms"`
}

// Report is the audit of a whole pack.
type Report struct {
	Pack    string  `json:"pack"`
	Type    string  `json:"type"` // "directory" or "json"
	Summary Summary `json:"summary"`
	Sounds  []Sound `json:"sounds"`
}

// Failed reports whether the audit should fail: on any error, and under
// strict on any warning too.
func (r Report) Failed(strict bool) bool {
	return r.Summary.Errors > 0 || (strict && r.Summary.Warnings > 0)
}

func ms(frames, rate int) float64 {
	return float64(frames) * 1000 / float64(rate)
}

// Analyze fills in the measurements, description and findings of s from its
// decoded frames. s.Categories must already be set; it selects the duration
// limit.
func Analyze(s *Sound, frames [][2]float64, rate int, o Options) {
	s.SampleRate = rate
	s.DurationMS = ms(len(frames), rate)
	s.Mono = true
	for _, f := range frames {
		if f[0] != f[1] {
			s.Mono = false
			break
		}
	}

	m := loudness.Measure(frames, rate)
	s.LoudnessLUFS = Level(m.LoudnessLUFS)
	s.IntegratedLUFS = Level(m.IntegratedLUFS)
	s.MomentaryMaxLUFS = Level(m.MomentaryMaxLUFS)
	s.PeakDBTP = Level(m.PeakDBTP)
	s.ClippedRuns = loudness.ClippedRuns(frames)

	start, end := loudness.ActiveRange(frames, o.SilenceThresholdDB)
	if start == end {
		s.add(RuleSilent, SeverityError, "no sample is above %.0f dBFS", o.SilenceThresholdDB)
		return
	}
	s.LeadingSilenceMS = ms(start, rate)
	s.TrailingSilenceMS = ms(len(frames)-end, rate)
	s.Description = Describe(frames[start:end], rate)

	if limit := o.MaxDurationFor(s.Categories); limit > 0 && s.DurationMS > float64(limit.Milliseconds()) {
		s.add(RuleTooLong, SeverityWarning, "%.2f s is longer than the %.1f s limit for %s",
			s.DurationMS/1000, limit.Seconds(), strings.Join(s.Categories, "+"))
	}

	off := m.LoudnessLUFS - o.TargetLUFS
	headroom := o.PeakCeilingDBTP - m.PeakDBTP
	switch {
	case off > o.ToleranceLU:
		s.add(RuleTooLoud, SeverityWarning, "%.1f LUFS is %.1f LU above the %.0f LUFS target", m.LoudnessLUFS, off, o.TargetLUFS)
	case off < -o.ToleranceLU && headroom > 0.5:
		s.add(RuleTooQuiet, SeverityWarning, "%.1f LUFS is %.1f LU below the %.0f LUFS target with %.1f dB of headroom",
			m.LoudnessLUFS, -off, o.TargetLUFS, headroom)
	case off < -o.ToleranceLU:
		s.add(RulePeakLimited, SeverityInfo, "%.1f LUFS is %.1f LU below target, but the peak is already at %.1f dBTP",
			m.LoudnessLUFS, -off, m.PeakDBTP)
	}
	if m.PeakDBTP > o.PeakCeilingDBTP+0.1 {
		s.add(RulePeak, SeverityWarning, "true peak %.1f dBTP is above the %.1f dBTP ceiling", m.PeakDBTP, o.PeakCeilingDBTP)
	}
	if s.ClippedRuns > 0 {
		s.add(RuleClipping, SeverityWarning, "%d runs of full-scale samples (clipped source)", s.ClippedRuns)
	}
	if s.LeadingSilenceMS > float64(o.MaxLeadingSilence.Milliseconds()) {
		s.add(RuleLeadingSilence, SeverityWarning, "%.0f ms of silence before the sound starts (it will feel late)", s.LeadingSilenceMS)
	}
	if s.TrailingSilenceMS > float64(o.MaxTrailingSilence.Milliseconds()) {
		s.add(RuleTrailingSilence, SeverityWarning, "%.0f ms of silence after the sound ends", s.TrailingSilenceMS)
	}
	if rate != TargetSampleRate {
		s.add(RuleSampleRate, SeverityInfo, "%d Hz is resampled to %d Hz on every play", rate, TargetSampleRate)
	}
}

// KeyCategory returns the category of a sound key or pack-relative path:
// its first directory, or RootCategory for a file at the root.
func KeyCategory(key string) string {
	if i := strings.IndexByte(key, '/'); i > 0 {
		return key[:i]
	}
	return RootCategory
}

// Pack audits the soundpack at packPath: a directory pack, or the JSON
// manifest of a JSON pack. Every audio file under the pack root is
// measured. The returned error covers only a pack that cannot be read at
// all; problems with individual sounds are findings.
func Pack(packPath string, o Options) (Report, error) {
	info, err := os.Stat(packPath)
	if err != nil {
		return Report{}, fmt.Errorf("cannot access path: %w", err)
	}

	root := packPath
	report := Report{Pack: filepath.Base(packPath), Type: "directory"}
	// keys maps a pack-relative file path to the sound keys it answers for.
	// It stays nil for a directory pack, where the path is the key.
	var keys map[string][]string
	if !info.IsDir() {
		v, err := soundpack.ValidateJSONSoundpack(packPath)
		if err != nil {
			return Report{}, fmt.Errorf("failed to load JSON soundpack: %w", err)
		}
		root = filepath.Dir(packPath)
		report.Pack, report.Type = v.File.Name, "json"
		keys = make(map[string][]string)
		for key, resolved := range v.Resolved {
			rel, err := filepath.Rel(root, resolved)
			if err != nil {
				continue
			}
			rel = filepath.ToSlash(rel)
			keys[rel] = append(keys[rel], key)
		}
	}

	var files []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if p != root && errors.Is(walkErr, fs.ErrNotExist) {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			if p != root && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if soundpack.IsAudioExt(filepath.Ext(p)) && d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return Report{}, fmt.Errorf("failed to scan soundpack: %w", err)
	}
	sort.Strings(files)
	if keys != nil {
		files = manifestFiles(root, files, keys)
	}

	firstByHash := make(map[string]string)
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return Report{}, err
		}
		s := Sound{Path: filepath.ToSlash(rel)}
		if keys == nil {
			// Keys always end in .wav, whatever the file's extension.
			s.Keys = []string{strings.TrimSuffix(s.Path, path.Ext(s.Path)) + ".wav"}
		} else {
			s.Keys = keys[s.Path]
			sort.Strings(s.Keys)
		}
		s.Categories = categoriesOf(s.Keys)
		auditFile(&s, file, o)

		if keys != nil && len(s.Keys) == 0 {
			s.add(RuleUnreferenced, SeverityInfo, "no mapping in the manifest uses this file")
		}
		if s.SHA256 != "" {
			if first, seen := firstByHash[s.SHA256]; seen {
				s.DuplicateOf = first
				s.add(RuleDuplicate, SeverityInfo, "same bytes as %s", first)
			} else {
				firstByHash[s.SHA256] = s.Path
			}
		}
		if s.Findings == nil {
			s.Findings = []Finding{}
		}
		report.Sounds = append(report.Sounds, s)
	}

	report.Summary = summarize(report.Sounds)
	slog.Info("audited soundpack", "pack", report.Pack, "files", report.Summary.Files,
		"errors", report.Summary.Errors, "warnings", report.Summary.Warnings)
	return report, nil
}

// manifestFiles narrows the audio files under a JSON pack's root to the ones
// the manifest references, plus unreferenced files sitting in the same
// directories (dead weight the pack ships). Audio elsewhere, such as the
// raw material a pack was mastered from, is not part of the pack.
func manifestFiles(root string, files []string, keys map[string][]string) []string {
	shipped := make(map[string]struct{})
	for rel := range keys {
		shipped[path.Dir(rel)] = struct{}{}
	}
	var out []string
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			continue
		}
		if _, ok := shipped[path.Dir(filepath.ToSlash(rel))]; ok {
			out = append(out, file)
		}
	}
	return out
}

func categoriesOf(keys []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, k := range keys {
		c := KeyCategory(k)
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

// auditFile reads, hashes, decodes and analyzes one file.
func auditFile(s *Sound, file string, o Options) {
	s.LoudnessLUFS, s.IntegratedLUFS = Level(math.Inf(-1)), Level(math.Inf(-1))
	s.MomentaryMaxLUFS, s.PeakDBTP = Level(math.Inf(-1)), Level(math.Inf(-1))

	data, err := os.ReadFile(file)
	if err != nil {
		s.add(RuleUndecodable, SeverityError, "cannot read: %v", err)
		return
	}
	s.Bytes = int64(len(data))
	sum := sha256.Sum256(data)
	s.SHA256 = hex.EncodeToString(sum[:])

	frames, rate, err := native.DecodeFrames(context.Background(), file, bytes.NewReader(data))
	if err != nil {
		slog.Debug("audit could not decode sound", "path", file, "error", err)
		s.add(RuleUndecodable, SeverityError, "claudio cannot play this file: %v", err)
		return
	}
	Analyze(s, frames, rate, o)
}

func summarize(sounds []Sound) Summary {
	sum := Summary{
		Files:           len(sounds),
		LoudnessMinLUFS: Level(math.Inf(-1)),
		LoudnessMaxLUFS: Level(math.Inf(-1)),
	}
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range sounds {
		sum.Bytes += s.Bytes
		sum.LongestMS = math.Max(sum.LongestMS, s.DurationMS)
		for _, f := range s.Findings {
			switch f.Severity {
			case SeverityError:
				sum.Errors++
			case SeverityWarning:
				sum.Warnings++
			default:
				sum.Infos++
			}
		}
		if l := float64(s.LoudnessLUFS); !math.IsInf(l, 0) {
			lo, hi = math.Min(lo, l), math.Max(hi, l)
		}
	}
	if lo <= hi {
		sum.LoudnessMinLUFS, sum.LoudnessMaxLUFS = Level(lo), Level(hi)
		sum.LoudnessSpread = math.Round((hi-lo)*100) / 100
	}
	return sum
}
