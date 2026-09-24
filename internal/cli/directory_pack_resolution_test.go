package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claudio.click/internal/audio/audiotest"
	"claudio.click/internal/cli/testenv"
	"claudio.click/internal/config"
	"claudio.click/internal/hooks"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/sounds"
	"claudio.click/internal/testutil/wavfixture"
)

const preToolUseGitStatus = `{"session_id":"t","transcript_path":"/t","cwd":"/t","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"git status"}}`

// Regression for #84 (not reproduced): a directory pack installed under
// the XDG data home and selected by name in the config plays its own
// loading/git-start.wav for PreToolUse `git status`, not default.wav.
func TestHookPlaysNamedDirectoryPackInstalledUnderXDGDataHome(t *testing.T) {
	testenv.IsolateXDG(t)
	audiotest.ResetLastFakeBackend()

	packDir := config.UserDataPath("soundpacks", "pack")
	for _, rel := range []string{"loading/git-start.wav", "default.wav"} {
		wavfixture.Write(t, filepath.Join(packDir, rel))
	}
	for _, category := range []string{"success", "error", "interactive", "completion", "system"} {
		if err := os.MkdirAll(filepath.Join(packDir, category), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	configPath := config.UserConfigPath("config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"default_soundpack":"pack"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	stderr := &bytes.Buffer{}
	if code := NewCLI().Run([]string{"claudio"}, strings.NewReader(preToolUseGitStatus), &bytes.Buffer{}, stderr); code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, stderr.String())
	}

	fake := audiotest.LastFakeBackend()
	if fake == nil {
		t.Fatal("fake audio backend was not constructed")
	}
	plays := fake.Plays()
	if len(plays) != 1 {
		t.Fatalf("expected one Play, got %+v", plays)
	}
	want := filepath.Join(packDir, "loading", "git-start.wav")
	if plays[0].SourcePath != want {
		t.Errorf("played %q, want %q", plays[0].SourcePath, want)
	}
}

// Regression for #84's embedded-pack claim: the windows pack maps a key
// in the PreToolUse `git status` chain ahead of default.wav. This checks
// the pack's keys rather than its C:\Windows\Media targets, so it holds
// on every OS.
func TestEmbeddedWindowsPackMapsGitStatusChainBeforeDefault(t *testing.T) {
	t.Parallel()

	event, err := hooks.ParseHookEvent([]byte(preToolUseGitStatus))
	if err != nil {
		t.Fatal(err)
	}
	chain := sounds.NewSoundMapper().MapSound(context.Background(), event.GetContext()).AllPaths

	data, err := config.GetEmbeddedPlatformSoundpackData("windows.json")
	if err != nil {
		t.Fatal(err)
	}
	pack, err := soundpack.PeekJSONSoundpackFromBytes(data)
	if err != nil {
		t.Fatal(err)
	}

	for _, key := range chain {
		if pack.Mappings[key] == "" {
			continue
		}
		if key == "default.wav" {
			t.Fatalf("windows pack maps nothing in chain %v before default.wav", chain)
		}
		t.Logf("windows pack resolves %q for PreToolUse git status", key)
		return
	}
	t.Fatalf("windows pack maps no key in chain %v", chain)
}
