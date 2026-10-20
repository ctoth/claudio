package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"claudio.click/internal/safeio"
	"claudio.click/internal/soundpack/synth"
)

// newSoundpackSynthCommand creates the soundpack synth subcommand
func newSoundpackSynthCommand() *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "synth <recipe.json> --out <dir>",
		Short: "Render original sounds from a JSON recipe",
		Long: `Synthesize every sound a recipe describes and write them, with a JSON
soundpack over them, into --out: <out>/synth/<sound>.wav and
<out>/soundpack.json. Master that to get a pack at a consistent level:

  claudio soundpack synth source/recipe.json --out source
  claudio soundpack master source/soundpack.json --out .
  claudio soundpack audit soundpack.json --strict

A recipe names sounds and says which sound answers for which key:

  {
    "name": "my-pack",
    "sounds": {
      "chirp": {
        "about": "two rising notes",
        "layers": [{"wave": "triangle", "notes": ["C5", "G5"], "step": 0.09,
                    "dur": 0.08, "release": 0.1}],
        "effects": [{"type": "reverb", "size": 0.3, "mix": 0.15}]
      }
    },
    "mappings": {"success/success.wav": "chirp", "default.wav": "chirp"}
  }

A layer is an oscillator or a slice of a recording. To use a recording:
  sample      path to a WAV, MP3 or AIFF file, relative to the recipe
  start, end  the slice to play, in seconds into the file
  speed       2 is an octave up and half as long; 0.5 the opposite
  reverse     true plays the slice backwards
  dur         optional; leave it out to play the whole slice
A sample layer takes gain, pan, tremolo, the envelope and the filters below,
and mixes with oscillator layers in the same sound.

Layer settings (times in seconds, gain in dB):
  wave        sine (default), triangle, square, saw, pulse, noise
  freq        pitch in Hz or a note name ("A4", "C#5"); freq_end glides to it
  notes,step  a sequence instead of one pitch; 0 is a rest
  at, dur     when the layer starts, how long each note is held
  attack, decay, sustain, release
              envelope; decay with sustain 0 is a pluck
  gain, pan   level in dB, position from -1 (left) to 1 (right)
  harmonics   overtone amplitudes for a sine: [1, 0.5, 0.25]
  width       pulse duty cycle
  fm          {"ratio": 3.5, "index": 4, "index_end": 0}  bells, metal
  vibrato     {"rate": 6, "depth": 0.3}   depth in semitones
  tremolo     {"rate": 9, "depth": 0.5}   depth 0..1
  voices, detune
              stacked copies spread over +/- detune cents
  lowpass, highpass, q
              filter cutoffs in Hz; lowpass_end and highpass_end sweep them

Effects, applied in order to the whole sound:
  reverb    size, damp, mix          delay     time, feedback, mix
  drive     amount                   bitcrush  bits, downsample
  lowpass   freq, q                  highpass  freq, q

Rendering is deterministic. Unknown settings are errors, so a misspelling
cannot be silently ignored.`,
		Args: cobra.ExactArgs(1),
	}
	cmd.Flags().StringVar(&outDir, "out", "", "directory to write synth/*.wav and soundpack.json into (required)")
	_ = cmd.MarkFlagRequired("out")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		file, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("cannot read recipe: %w", err)
		}
		data, err := safeio.ReadAllCapped(file, 4<<20, "recipe")
		_ = file.Close()
		if err != nil {
			return err
		}
		recipe, err := synth.Parse(data)
		if err != nil {
			return err
		}
		// Sample paths are relative to the recipe, so a pack builds the
		// same wherever it is checked out.
		if err := recipe.LoadSamples(filepath.Dir(args[0])); err != nil {
			return err
		}
		written, err := recipe.Write(outDir)
		if err != nil {
			return err
		}
		for _, w := range written {
			about := recipe.Sounds[w.Name].About
			cmd.Printf("%-28s %5.2fs  %2d key(s)  %s\n", w.Name, w.Seconds, w.Keys, about)
		}
		cmd.Printf("\nRendered %d sounds for %d keys into %s\n", len(written), len(recipe.Mappings), outDir)
		return nil
	}
	return cmd
}

// SoundpackTopic is the GitHub topic that marks a repository as a Claudio
// soundpack, and soundpackRepoPrefix the naming convention that goes with
// it. Together they are the whole discovery mechanism: there is no index
// to register with.
const (
	SoundpackTopic      = "claudio-soundpack"
	soundpackRepoPrefix = "claudio-soundpack-"
	defaultSearchURL    = "https://api.github.com/search/repositories"
)

// foundPack is one search result.
type foundPack struct {
	Name        string `json:"name"`
	Repository  string `json:"repository"`
	Description string `json:"description"`
	Stars       int    `json:"stars"`
	Updated     string `json:"updated"`
	Install     string `json:"install"`
}

// newSoundpackSearchCommand creates the soundpack search subcommand
func newSoundpackSearchCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "search [words...]",
		Short: "Find published soundpacks on GitHub",
		Long: `List GitHub repositories tagged with the topic "` + SoundpackTopic + `", most
starred first, with the command that installs each one.

To make your own pack findable: name the repository
` + soundpackRepoPrefix + `<name>, put the pack (soundpack.json or a directory pack) at
its root, and add the topic:

  gh repo edit --add-topic ` + SoundpackTopic + `

This is the only command that contacts a server on its own, and only when
you run it. Set GITHUB_TOKEN to raise GitHub's rate limit.

Examples:
  claudio soundpack search
  claudio soundpack search retro
  claudio soundpack search --json`,
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print results as JSON")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		packs, err := searchSoundpacks(cmd.Context(), args)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(cmd, packs)
		}
		if len(packs) == 0 {
			cmd.Println("No soundpacks found.")
			return nil
		}
		for _, p := range packs {
			cmd.Printf("%s  (%d stars, updated %s)\n", p.Name, p.Stars, p.Updated)
			if p.Description != "" {
				cmd.Printf("  %s\n", p.Description)
			}
			cmd.Printf("  %s\n\n", p.Install)
		}
		return nil
	}
	return cmd
}

func searchSoundpacks(ctx context.Context, words []string) ([]foundPack, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	base := os.Getenv("CLAUDIO_SOUNDPACK_SEARCH_URL")
	if base == "" {
		base = defaultSearchURL
	}
	query := url.Values{
		"q":        {strings.TrimSpace("topic:" + SoundpackTopic + " " + strings.Join(words, " "))},
		"sort":     {"stars"},
		"per_page": {"50"},
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"?"+query.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "claudio/"+Version)
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && base == defaultSearchURL {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("could not read GitHub's response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var failure struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &failure)
		if failure.Message == "" {
			failure.Message = strings.TrimSpace(string(body))
		}
		return nil, fmt.Errorf("GitHub search failed (%s): %s", resp.Status, failure.Message)
	}

	var result struct {
		Items []struct {
			Name        string  `json:"name"`
			FullName    string  `json:"full_name"`
			Description *string `json:"description"`
			Stars       int     `json:"stargazers_count"`
			PushedAt    string  `json:"pushed_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("could not parse GitHub's response: %w", err)
	}

	packs := make([]foundPack, 0, len(result.Items))
	for _, item := range result.Items {
		name := strings.TrimPrefix(item.Name, soundpackRepoPrefix)
		p := foundPack{
			Name:       name,
			Repository: item.FullName,
			Stars:      item.Stars,
			Updated:    item.PushedAt,
			Install:    fmt.Sprintf("claudio soundpack add gh:%s --name %s", item.FullName, name),
		}
		if item.Description != nil {
			p.Description = *item.Description
		}
		if len(p.Updated) >= 10 {
			p.Updated = p.Updated[:10]
		}
		packs = append(packs, p)
	}
	return packs, nil
}
