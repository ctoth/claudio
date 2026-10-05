// Package master rewrites the sounds of a pack so they pass the audit:
// silence trimmed from both ends, over-long sounds cut to their category's
// limit with a fade, and every sound brought to the same loudness under a
// true-peak ceiling. Output is 48 kHz 16-bit WAV, the player's own format:
// no resampling at play time, and none of the encoder-delay silence that
// MP3 puts in front of a sound.
package master

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gopxl/beep/v2"

	"claudio.click/internal/audio/native"
	"claudio.click/internal/loudness"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/soundpack/audit"
)

const (
	rate = audit.TargetSampleRate

	fadeIn  = time.Millisecond
	fadeOut = 5 * time.Millisecond

	// Silence left at an edge after a gain change is re-trimmed only past
	// these, well inside the audit's limits, so the edge fades themselves
	// never count as new silence.
	leadSlack = 5 * time.Millisecond
	tailSlack = 50 * time.Millisecond

	maxPasses = 6

	// The most a sound's peaks are held down to reach the target. Beyond
	// this the sound is left quiet instead: it is all spike and no body,
	// and limiting it harder would change what it is.
	maxLimitDB = 9.0
	// Room left under the ceiling for the true peak to exceed the sample
	// peak the limiter works to.
	limiterMarginDB = 0.3
)

// Options are the audit thresholds to master to, plus how a truncated sound
// is faded out.
type Options struct {
	audit.Options
	TruncateFade time.Duration
}

// DefaultOptions masters to the audit's default thresholds.
func DefaultOptions() Options {
	return Options{Options: audit.DefaultOptions(), TruncateFade: 200 * time.Millisecond}
}

// Result is what mastering did to one sound.
type Result struct {
	// Source and Output are slash-separated paths relative to the source
	// and output pack roots. They are empty when Process is called directly.
	Source string   `json:"source,omitempty"`
	Output string   `json:"output,omitempty"`
	Keys   []string `json:"keys,omitempty"`

	GainDB float64 `json:"gain_db"`
	// LimitedDB is how far the sound's peaks were held down so the rest of
	// it could reach the target. Zero for a sound that only needed gain.
	LimitedDB     float64 `json:"limited_db"`
	TrimmedLeadMS float64 `json:"trimmed_lead_ms"`
	TrimmedTailMS float64 `json:"trimmed_tail_ms"`
	// Truncated is set when the sound was longer than its category allows
	// and was cut. A cut is mechanical: it keeps the start of the sound,
	// which may not be the part worth keeping.
	Truncated bool `json:"truncated"`
	// Silent is set when the source has nothing above the silence
	// threshold; no output is produced.
	Silent bool `json:"silent"`

	Before audit.Sound `json:"before"`
	After  audit.Sound `json:"after"`
}

func frameCount(d time.Duration) int {
	return int(d.Seconds() * rate)
}

func msOf(frames int) float64 {
	return float64(frames) * 1000 / rate
}

// Process masters one decoded sound and returns it at 48 kHz. categories
// selects the duration limit, as in the audit.
func Process(frames [][2]float64, sourceRate int, categories []string, o Options) ([][2]float64, Result) {
	var res Result
	if sourceRate != rate {
		frames = resample(frames, sourceRate)
	}

	start, end := loudness.ActiveRange(frames, o.SilenceThresholdDB)
	if start == end {
		res.Silent = true
		return nil, res
	}
	res.TrimmedLeadMS = msOf(start)
	res.TrimmedTailMS = msOf(len(frames) - end)
	out := append([][2]float64(nil), frames[start:end]...)

	if limit := frameCount(o.MaxDurationFor(categories)); limit > 0 && len(out) > limit {
		out = out[:limit]
		fade(out, len(out)-min(frameCount(o.TruncateFade), len(out)/2), len(out), false)
		res.Truncated = true
	}

	// Gain moves the silence boundary: turning a sound down pushes more of
	// its tail under the threshold, and trimming that changes its loudness.
	// Repeat until neither moves.
	edgesDirty := true
	for range maxPasses {
		if edgesDirty {
			fade(out, 0, min(frameCount(fadeIn), len(out)/2), true)
			fade(out, len(out)-min(frameCount(fadeOut), len(out)/2), len(out), false)
			edgesDirty = false
		}
		gain := loudness.GainDB(loudness.Measure(out, rate), o.TargetLUFS, o.PeakCeilingDBTP)
		scale := math.Pow(10, gain/20)
		for i := range out {
			out[i][0] *= scale
			out[i][1] *= scale
		}
		res.GainDB += gain

		s, e := loudness.ActiveRange(out, o.SilenceThresholdDB)
		if s == e {
			break
		}
		if s > frameCount(leadSlack) {
			res.TrimmedLeadMS += msOf(s)
			out, e, edgesDirty = out[s:], e-s, true
		}
		if len(out)-e > frameCount(tailSlack) {
			res.TrimmedTailMS += msOf(len(out) - e)
			out, edgesDirty = out[:e], true
		}
		if !edgesDirty && math.Abs(gain) < 0.05 {
			break
		}
	}

	// A sound that is still short of the target has peaks in the way.
	// Recordings of impacts are like this: a few spikes far above the body
	// of the sound. Push it up and hold only the spikes down.
	for range 3 {
		short := o.TargetLUFS - loudness.Measure(out, rate).LoudnessLUFS
		if short <= o.ToleranceLU/2 || res.LimitedDB >= maxLimitDB || math.IsInf(short, 0) {
			break
		}
		boost := math.Min(short, maxLimitDB-res.LimitedDB)
		scale := math.Pow(10, boost/20)
		for i := range out {
			out[i][0] *= scale
			out[i][1] *= scale
		}
		limit(out, math.Pow(10, (o.PeakCeilingDBTP-limiterMarginDB)/20))
		res.LimitedDB += boost
		res.GainDB += boost
	}
	if res.LimitedDB > 0 {
		// The limiter works on samples; settle the true peak with gain.
		if trim := loudness.GainDB(loudness.Measure(out, rate), o.TargetLUFS, o.PeakCeilingDBTP); trim < 0 {
			scale := math.Pow(10, trim/20)
			for i := range out {
				out[i][0] *= scale
				out[i][1] *= scale
			}
			res.GainDB += trim
		}
	}
	res.GainDB = math.Round(res.GainDB*100) / 100
	res.LimitedDB = math.Round(res.LimitedDB*100) / 100

	res.After = audit.Sound{Categories: categories}
	audit.Analyze(&res.After, out, rate, o.Options)
	return out, res
}

// limit holds frames under ceiling (a linear sample level) by turning the
// gain down around each peak: the reduction begins 2 ms before the peak, so
// the peak itself is not flattened into a square edge, and recovers over
// about 60 ms. Both channels get the same gain, so the stereo image holds.
func limit(frames [][2]float64, ceiling float64) {
	const lookahead = rate * 2 / 1000
	release := 1 - math.Exp(-1/(0.06*rate))

	// The gain each sample needs, then the least gain needed over the
	// lookahead, so the reduction arrives ahead of the peak.
	need := make([]float64, len(frames)+lookahead)
	for i := range need {
		need[i] = 1
		if i < len(frames) {
			if peak := math.Max(math.Abs(frames[i][0]), math.Abs(frames[i][1])); peak > ceiling {
				need[i] = ceiling / peak
			}
		}
	}
	held := make([]float64, len(frames))
	prev := 1.0
	for i := range held {
		least := 1.0
		for _, g := range need[i : i+lookahead+1] {
			least = math.Min(least, g)
		}
		// Recover gradually, but never above what the lookahead allows.
		prev = math.Min(least, prev+(1-prev)*release)
		held[i] = prev
	}
	// Average over the lookahead to round the corner into each reduction.
	// Every value averaged was already low enough for this sample, so the
	// average is too.
	var sum float64
	for i := range frames {
		sum += held[i]
		if i >= lookahead {
			sum -= held[i-lookahead]
		}
		g := sum / float64(min(i+1, lookahead))
		frames[i][0] *= g
		frames[i][1] *= g
	}
}

// fade applies a raised-cosine fade to frames[from:to], rising when in is
// true and falling otherwise.
func fade(frames [][2]float64, from, to int, in bool) {
	n := to - from
	for i := range n {
		g := 0.5 - 0.5*math.Cos(math.Pi*float64(i+1)/float64(n+1))
		if !in {
			g = 1 - g
		}
		frames[from+i][0] *= g
		frames[from+i][1] *= g
	}
}

type sliceStreamer struct {
	frames [][2]float64
	pos    int
}

func (s *sliceStreamer) Stream(dst [][2]float64) (int, bool) {
	n := copy(dst, s.frames[s.pos:])
	s.pos += n
	return n, n > 0
}

func (s *sliceStreamer) Err() error { return nil }

// resample converts frames to 48 kHz with the same resampler and quality
// the player uses.
func resample(frames [][2]float64, from int) [][2]float64 {
	stream := beep.Resample(4, beep.SampleRate(from), beep.SampleRate(rate), &sliceStreamer{frames: frames})
	out := make([][2]float64, 0, len(frames)*rate/from+1)
	buf := make([][2]float64, 512)
	for {
		n, ok := stream.Stream(buf)
		out = append(out, buf[:n]...)
		if !ok || n == 0 {
			return out
		}
	}
}

// EncodeWAV writes frames as a 48 kHz 16-bit PCM WAV. Identical channels
// are stored once, as mono; the player duplicates them back.
func EncodeWAV(w io.Writer, frames [][2]float64) error {
	channels := 1
	for _, f := range frames {
		if f[0] != f[1] {
			channels = 2
			break
		}
	}
	const width = 2
	dataBytes := len(frames) * channels * width

	var buf bytes.Buffer
	buf.Grow(44 + dataBytes)
	buf.WriteString("RIFF")
	put := func(v any) { _ = binary.Write(&buf, binary.LittleEndian, v) }
	put(uint32(36 + dataBytes))
	buf.WriteString("WAVEfmt ")
	put(uint32(16))
	put(uint16(1)) // PCM
	put(uint16(channels))
	put(uint32(rate))
	put(uint32(rate * channels * width))
	put(uint16(channels * width))
	put(uint16(8 * width))
	buf.WriteString("data")
	put(uint32(dataBytes))
	for _, f := range frames {
		for ch := range channels {
			x := math.Round(f[ch] * 32767)
			put(int16(math.Max(-32768, math.Min(32767, x))))
		}
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// job is one source file to master.
type job struct {
	source string // absolute
	rel    string // slash-separated, relative to the source root
	output string // slash-separated, relative to dst
	keys   []string
}

// Pack masters the soundpack at src (a directory pack, or a JSON pack's
// manifest) into the directory dst. A directory pack keeps its layout with
// every file rewritten as .wav. A JSON pack becomes dst/soundpack.json with
// its sounds under dst/sounds/, each source file mastered once however many
// keys use it. Source files are never modified.
func Pack(src, dst string, o Options) ([]Result, error) {
	info, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("cannot access path: %w", err)
	}
	srcRoot := src
	if !info.IsDir() {
		srcRoot = filepath.Dir(src)
	}
	if same, err := sameDir(srcRoot, dst); err != nil {
		return nil, err
	} else if same {
		return nil, fmt.Errorf("output directory %s is the source pack; master into a different directory", dst)
	}

	var jobs []job
	var manifest *soundpack.JSONSoundpackFile
	if info.IsDir() {
		jobs, err = directoryJobs(srcRoot)
	} else {
		jobs, manifest, err = manifestJobs(src, srcRoot)
	}
	if err != nil {
		return nil, err
	}
	if len(jobs) == 0 {
		return nil, fmt.Errorf("no audio files to master in %s", src)
	}

	outputs := make(map[string]string)
	for _, j := range jobs {
		if other, clash := outputs[strings.ToLower(j.output)]; clash {
			return nil, fmt.Errorf("%s and %s would both be written to %s", other, j.rel, j.output)
		}
		outputs[strings.ToLower(j.output)] = j.rel
	}

	results := make([]Result, 0, len(jobs))
	for _, j := range jobs {
		res, err := masterFile(j, dst, o)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", j.rel, err)
		}
		results = append(results, res)
	}

	if manifest != nil {
		data, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dst, "soundpack.json"), append(data, '\n'), 0o644); err != nil {
			return nil, fmt.Errorf("failed to write manifest: %w", err)
		}
	}
	slog.Info("mastered soundpack", "source", src, "output", dst, "files", len(results))
	return results, nil
}

func sameDir(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(filepath.Clean(absA), filepath.Clean(absB)), nil
}

func wavName(rel string) string {
	return strings.TrimSuffix(rel, path.Ext(rel)) + ".wav"
}

func directoryJobs(root string) ([]job, error) {
	var jobs []job
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if p != root && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !soundpack.IsAudioExt(filepath.Ext(p)) || !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		jobs = append(jobs, job{source: p, rel: rel, output: wavName(rel), keys: []string{wavName(rel)}})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan soundpack: %w", err)
	}
	return jobs, nil
}

// manifestJobs returns one job per source file the manifest references, and
// the manifest to write: the same pack with every mapping pointing at its
// mastered file.
func manifestJobs(manifestPath, root string) ([]job, *soundpack.JSONSoundpackFile, error) {
	v, err := soundpack.ValidateJSONSoundpack(manifestPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load JSON soundpack: %w", err)
	}
	if err := v.Err(); err != nil {
		return nil, nil, err
	}

	byRel := make(map[string]*job)
	out := v.File
	out.Mappings = make(map[string]string, len(v.Resolved))
	for key, resolved := range v.Resolved {
		rel, err := filepath.Rel(root, resolved)
		if err != nil {
			return nil, nil, err
		}
		rel = filepath.ToSlash(rel)
		j, ok := byRel[rel]
		if !ok {
			j = &job{source: resolved, rel: rel, output: "sounds/" + wavName(rel)}
			byRel[rel] = j
		}
		j.keys = append(j.keys, key)
		out.Mappings[key] = j.output
	}

	jobs := make([]job, 0, len(byRel))
	for _, j := range byRel {
		sort.Strings(j.keys)
		jobs = append(jobs, *j)
	}
	sort.Slice(jobs, func(a, b int) bool { return jobs[a].rel < jobs[b].rel })
	return jobs, &out, nil
}

func categoriesOf(keys []string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, k := range keys {
		c := audit.KeyCategory(k)
		if _, ok := seen[c]; !ok {
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func masterFile(j job, dst string, o Options) (Result, error) {
	data, err := os.ReadFile(j.source)
	if err != nil {
		return Result{}, err
	}
	frames, sourceRate, err := native.DecodeFrames(context.Background(), j.source, bytes.NewReader(data))
	if err != nil {
		return Result{}, err
	}
	categories := categoriesOf(j.keys)

	before := audit.Sound{Path: j.rel, Keys: j.keys, Categories: categories, Bytes: int64(len(data))}
	audit.Analyze(&before, frames, sourceRate, o.Options)

	out, res := Process(frames, sourceRate, categories, o)
	if res.Silent {
		return Result{}, errors.New("the file is silent; remove it from the pack")
	}
	res.Source, res.Output, res.Keys, res.Before = j.rel, j.output, j.keys, before

	var buf bytes.Buffer
	if err := EncodeWAV(&buf, out); err != nil {
		return Result{}, err
	}
	target := filepath.Join(dst, filepath.FromSlash(j.output))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(target, buf.Bytes(), 0o644); err != nil {
		return Result{}, err
	}
	res.After.Path, res.After.Keys, res.After.Bytes = j.output, j.keys, int64(buf.Len())
	slog.Debug("mastered sound", "source", j.rel, "output", j.output, "gain_db", res.GainDB, "truncated", res.Truncated)
	return res, nil
}
