package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const demoRecipe = `{
  "name": "demo",
  "sounds": {
    "chirp": {"about": "two rising notes", "layers": [{"notes": ["C5", "G5"], "step": 0.09, "dur": 0.08, "release": 0.1}]},
    "thud":  {"layers": [{"freq": 180, "freq_end": 60, "dur": 0.2, "decay": 0.15, "sustain": 0}]}
  },
  "mappings": {"default.wav": "chirp", "success/success.wav": "chirp", "error/error.wav": "thud"}
}`

func TestSoundpackSynthRendersARecipeThatMastersClean(t *testing.T) {
	dir := t.TempDir()
	recipe := filepath.Join(dir, "source", "recipe.json")
	if err := os.MkdirAll(filepath.Dir(recipe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recipe, []byte(demoRecipe), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runCommand(newSoundpackSynthCommand(), recipe, "--out", filepath.Join(dir, "source"))
	if err != nil {
		t.Fatalf("synth failed: %v\n%s", err, out)
	}
	for _, want := range []string{"chirp", "thud", "2 sounds"} {
		if !strings.Contains(out, want) {
			t.Errorf("report does not mention %q:\n%s", want, out)
		}
	}

	built := filepath.Join(dir, "built")
	if out, err := runCommand(newSoundpackMasterCommand(), filepath.Join(dir, "source", "soundpack.json"), "--out", built); err != nil {
		t.Fatalf("master failed: %v\n%s", err, out)
	}
	if out, err := runCommand(newSoundpackAuditCommand(), filepath.Join(built, "soundpack.json"), "--strict"); err != nil {
		t.Errorf("synthesized pack fails strict audit: %v\n%s", err, out)
	}
}

func TestSoundpackSynthReportsRecipeMistakes(t *testing.T) {
	recipe := filepath.Join(t.TempDir(), "recipe.json")
	bad := strings.Replace(demoRecipe, `"freq": 180`, `"freq": 180, "wave": "sqaure"`, 1)
	if err := os.WriteFile(recipe, []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runCommand(newSoundpackSynthCommand(), recipe, "--out", t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "thud, layer 1") {
		t.Errorf("error = %v, want it to locate the bad layer", err)
	}
}

func TestSoundpackSearchListsPacksByTopic(t *testing.T) {
	var query, agent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query, agent = r.URL.Query().Get("q"), r.Header.Get("User-Agent")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"total_count": 2,
			"items": []map[string]any{
				{"name": "claudio-soundpack-startrek-bridge", "full_name": "ctoth/claudio-soundpack-startrek-bridge",
					"description": "Star Trek bridge console sounds", "stargazers_count": 12, "pushed_at": "2026-10-05T00:49:00Z"},
				{"name": "my-sounds", "full_name": "someone/my-sounds", "description": nil, "stargazers_count": 0, "pushed_at": "2026-01-02T00:00:00Z"},
			},
		})
	}))
	defer server.Close()
	t.Setenv("CLAUDIO_SOUNDPACK_SEARCH_URL", server.URL)

	out, err := runCommand(newSoundpackSearchCommand(), "trek")
	if err != nil {
		t.Fatalf("search failed: %v\n%s", err, out)
	}
	if !strings.Contains(query, "topic:claudio-soundpack") || !strings.Contains(query, "trek") {
		t.Errorf("query = %q, want the topic and the search term", query)
	}
	if !strings.Contains(agent, "claudio") {
		t.Errorf("User-Agent = %q; GitHub rejects requests without one", agent)
	}
	for _, want := range []string{
		"startrek-bridge", "Star Trek bridge console sounds",
		"claudio soundpack add gh:ctoth/claudio-soundpack-startrek-bridge --name startrek-bridge",
		"claudio soundpack add gh:someone/my-sounds --name my-sounds",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not contain %q:\n%s", want, out)
		}
	}

	jsonOut, err := runCommand(newSoundpackSearchCommand(), "--json")
	if err != nil {
		t.Fatal(err)
	}
	var packs []foundPack
	if err := json.Unmarshal([]byte(jsonOut), &packs); err != nil || len(packs) != 2 || packs[0].Name != "startrek-bridge" {
		t.Errorf("--json: %v %+v\n%s", err, packs, jsonOut)
	}
}

func TestSoundpackSearchReportsFailures(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"API rate limit exceeded"}`, http.StatusForbidden)
	}))
	defer server.Close()
	t.Setenv("CLAUDIO_SOUNDPACK_SEARCH_URL", server.URL)

	_, err := runCommand(newSoundpackSearchCommand())
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error = %v, want GitHub's message passed on", err)
	}
}

func TestSoundpackSearchWithNoResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"total_count":0,"items":[]}`))
	}))
	defer server.Close()
	t.Setenv("CLAUDIO_SOUNDPACK_SEARCH_URL", server.URL)

	out, err := runCommand(newSoundpackSearchCommand(), "nothing")
	if err != nil || !strings.Contains(out, "No soundpacks found") {
		t.Errorf("out = %q err = %v", out, err)
	}
}
