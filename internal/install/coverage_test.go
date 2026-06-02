package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/afero"
)

func TestNormalizeMSYSPathCoverage(t *testing.T) {
	// normalizeMSYSPath uses filepath.FromSlash, which differs by OS, so build
	// expectations portably rather than hardcoding backslashes.
	cases := map[string]string{
		"/c/Users/testuser": "C:" + filepath.FromSlash("/Users/testuser"),
		"/d/work":           "D:" + filepath.FromSlash("/work"),
		"already/plain":     "already/plain",
		"":                  "",
		"/notdrive":         "/notdrive",
	}
	for in, want := range cases {
		if got := normalizeMSYSPath(in); got != want {
			t.Errorf("normalizeMSYSPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetHomeDirectoryWindowsBranches(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only home resolution branches")
	}
	// MSYS-style HOME when USERPROFILE absent
	t.Setenv("USERPROFILE", "")
	t.Setenv("HOME", "/c/Users/testuser")
	if got := getHomeDirectory(); got != `C:\Users\testuser` {
		t.Errorf("MSYS HOME normalization: got %q", got)
	}
	// HOMEDRIVE + HOMEPATH fallback
	t.Setenv("HOME", "")
	t.Setenv("HOMEDRIVE", "D:")
	t.Setenv("HOMEPATH", `\Users\testuser`)
	if got := getHomeDirectory(); got != `D:\Users\testuser` {
		t.Errorf("HOMEDRIVE+HOMEPATH: got %q", got)
	}
	// Nothing set -> empty
	t.Setenv("HOMEDRIVE", "")
	t.Setenv("HOMEPATH", "")
	if got := getHomeDirectory(); got != "" {
		t.Errorf("expected empty home when no env set, got %q", got)
	}
}

func TestGetHomeDirectoryUnixBranches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix-only home resolution branch")
	}
	t.Setenv("HOME", "/home/test")
	if got := getHomeDirectory(); got != "/home/test" {
		t.Errorf("expected /home/test, got %q", got)
	}
	t.Setenv("HOME", "")
	if got := getHomeDirectory(); got != "" {
		t.Errorf("expected empty home when HOME unset, got %q", got)
	}
}

func TestWriteSettingsFileReadOnlyFails(t *testing.T) {
	ro := afero.NewReadOnlyFs(afero.NewMemMapFs())
	if err := WriteSettingsFile(ro, "/x/settings.json", &SettingsMap{"a": "b"}); err == nil {
		t.Error("expected error writing to read-only filesystem")
	}
}

func TestGetJSONTypeCoverage(t *testing.T) {
	cases := map[string]string{
		"":          "empty",
		"[1,2]":     "array",
		`"hi"`:      "string",
		"null":      "null",
		"true":      "boolean",
		"false":     "boolean",
		"42":        "non-object value",
		"{\"a\":1}": "object",
	}
	for in, want := range cases {
		if got := getJSONType([]byte(in)); got != want {
			t.Errorf("getJSONType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReadSettingsFileBranches(t *testing.T) {
	fsys := afero.NewMemMapFs()

	// Missing file -> empty settings, no error
	s, err := ReadSettingsFile(fsys, "/nope/settings.json")
	if err != nil || s == nil {
		t.Fatalf("expected empty settings for missing file, got %v err %v", s, err)
	}

	// "null" content -> empty settings
	_ = afero.WriteFile(fsys, "/a.json", []byte("null"), 0644)
	if s, err := ReadSettingsFile(fsys, "/a.json"); err != nil || s == nil {
		t.Fatalf("expected empty settings for null content, err %v", err)
	}

	// whitespace-only -> empty settings
	_ = afero.WriteFile(fsys, "/ws.json", []byte("   \n"), 0644)
	if _, err := ReadSettingsFile(fsys, "/ws.json"); err != nil {
		t.Fatalf("expected nil err for whitespace content, got %v", err)
	}

	// JSON array (not object) -> error
	_ = afero.WriteFile(fsys, "/arr.json", []byte("[1,2,3]"), 0644)
	if _, err := ReadSettingsFile(fsys, "/arr.json"); err == nil {
		t.Error("expected error for JSON array settings")
	}

	// malformed JSON -> error
	_ = afero.WriteFile(fsys, "/bad.json", []byte("{bad"), 0644)
	if _, err := ReadSettingsFile(fsys, "/bad.json"); err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestWriteSettingsFileRoundTrip(t *testing.T) {
	fsys := afero.NewMemMapFs()
	path := "/deep/nested/dir/settings.json"
	in := &SettingsMap{"version": "1.0", "hooks": map[string]interface{}{}}
	if err := WriteSettingsFile(fsys, path, in); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	out, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if (*out)["version"] != "1.0" {
		t.Errorf("roundtrip lost version, got %v", (*out)["version"])
	}
}

func TestWriteSettingsFileMarshalError(t *testing.T) {
	fsys := afero.NewMemMapFs()
	settings := &SettingsMap{"bad": make(chan int)}
	if err := WriteSettingsFile(fsys, "/settings.json", settings); err == nil {
		t.Error("expected marshal error for unsupported settings value")
	}
}

func TestWriteSettingsFilePreservesExistingPermissions(t *testing.T) {
	fsys := afero.NewMemMapFs()
	path := "/settings.json"
	if err := afero.WriteFile(fsys, path, []byte(`{"old":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	settings := &SettingsMap{"new": true}
	if err := WriteSettingsFile(fsys, path, settings); err != nil {
		t.Fatal(err)
	}
	info, err := fsys.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode() & os.ModePerm; got != 0600 {
		t.Errorf("mode = %v, want 0600", got)
	}
}

func TestMergeHookValuesStringFormatExisting(t *testing.T) {
	// Existing hook in legacy string format (non-claudio) must be preserved
	// and merged into array form alongside the claudio command.
	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": "/usr/bin/other-tool",
		},
	}
	claudioHooks, err := GenerateClaudioHooksForAgent("/usr/local/bin/claudio", AgentClaude)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatal(err)
	}
	hooksSection := (*merged)["hooks"].(map[string]interface{})
	arr, ok := hooksSection["PreToolUse"].([]interface{})
	if !ok {
		t.Fatalf("expected PreToolUse merged into array, got %T", hooksSection["PreToolUse"])
	}
	foundOther, foundClaudio := false, false
	for _, e := range arr {
		cfg := e.(map[string]interface{})
		for _, h := range cfg["hooks"].([]interface{}) {
			switch h.(map[string]interface{})["command"] {
			case "/usr/bin/other-tool":
				foundOther = true
			case "/usr/local/bin/claudio":
				foundClaudio = true
			}
		}
	}
	if !foundOther || !foundClaudio {
		t.Errorf("merge lost a command: other=%v claudio=%v", foundOther, foundClaudio)
	}
}

func TestMergeHooksRefreshesClaudioWithoutDroppingExistingHooks(t *testing.T) {
	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{"command": "/usr/bin/logger"},
					},
				},
				map[string]interface{}{
					"matcher": "*",
					"hooks": []interface{}{
						map[string]interface{}{"command": "/old/claudio"},
					},
				},
			},
		},
	}
	claudioHooks, err := GenerateClaudioHooksForAgent("/new/claudio", AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatal(err)
	}
	hooksSection := (*merged)["hooks"].(map[string]interface{})
	arr := hooksSection["PreToolUse"].([]interface{})

	foundLogger := false
	foundOldClaudio := false
	foundNewClaudio := false
	for _, e := range arr {
		cfg := e.(map[string]interface{})
		for _, h := range cfg["hooks"].([]interface{}) {
			switch h.(map[string]interface{})["command"] {
			case "/usr/bin/logger":
				foundLogger = true
			case "/old/claudio":
				foundOldClaudio = true
			case "/new/claudio":
				foundNewClaudio = true
			}
		}
	}
	if !foundLogger {
		t.Error("existing non-claudio hook was dropped")
	}
	if foundOldClaudio {
		t.Error("old claudio hook was not refreshed")
	}
	if !foundNewClaudio {
		t.Error("new claudio hook missing after refresh")
	}
}

func TestMergeHookValuesPreservesNonClaudioEntriesWhileRefreshingClaudio(t *testing.T) {
	entries := []interface{}{
		"raw-entry",
		map[string]interface{}{"matcher": "*"},
		map[string]interface{}{
			"matcher": "mixed",
			"hooks": []interface{}{
				"raw-hook",
				map[string]interface{}{"command": 42},
				map[string]interface{}{"command": "/old/claudio"},
				map[string]interface{}{"command": "/usr/bin/logger"},
			},
		},
		map[string]interface{}{
			"matcher": "claudio-only",
			"hooks": []interface{}{
				map[string]interface{}{"command": "/old/claudio"},
			},
		},
	}
	claudioValue := []interface{}{
		map[string]interface{}{
			"matcher": "*",
			"hooks": []interface{}{
				map[string]interface{}{"command": "/new/claudio"},
			},
		},
	}

	filtered := mergeHookValues(entries, claudioValue).([]interface{})
	if len(filtered) != 4 {
		t.Fatalf("merged entry count = %d, want 4: %#v", len(filtered), filtered)
	}

	foundLogger := false
	foundOldClaudio := false
	foundNewClaudio := false
	foundRawHook := false
	foundNumericCommand := false
	for _, entry := range filtered {
		cfg, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		hooksList, ok := cfg["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, hook := range hooksList {
			if hook == "raw-hook" {
				foundRawHook = true
				continue
			}
			cmd, ok := hook.(map[string]interface{})
			if !ok {
				continue
			}
			switch cmd["command"] {
			case 42:
				foundNumericCommand = true
			case "/old/claudio":
				foundOldClaudio = true
			case "/new/claudio":
				foundNewClaudio = true
			case "/usr/bin/logger":
				foundLogger = true
			}
		}
	}
	if !foundLogger || !foundRawHook || !foundNumericCommand {
		t.Errorf("non-claudio content not preserved: logger=%v raw=%v numeric=%v", foundLogger, foundRawHook, foundNumericCommand)
	}
	if foundOldClaudio {
		t.Error("old claudio command was not removed")
	}
	if !foundNewClaudio {
		t.Error("new claudio command was not appended")
	}
}

func TestIsClaudioHookFormats(t *testing.T) {
	if !IsClaudioHook("/usr/local/bin/claudio") {
		t.Error("expected string claudio command recognized")
	}
	if !IsClaudioHook(`"/usr/local/bin/claudio.exe"`) {
		t.Error("expected quoted windows claudio recognized")
	}
	if IsClaudioHook("/usr/bin/other") {
		t.Error("non-claudio command must not be recognized")
	}
	arr := []interface{}{
		map[string]interface{}{
			"hooks": []interface{}{
				map[string]interface{}{"command": "/opt/claudio"},
			},
		},
	}
	if !IsClaudioHook(arr) {
		t.Error("expected array-format claudio recognized")
	}
}

func TestIsClaudioHookFindsClaudioInMergedHookArrays(t *testing.T) {
	cases := []struct {
		name string
		arr  []interface{}
	}{
		{
			name: "claudio after existing hook",
			arr: []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{"command": "/usr/bin/logger"},
					},
				},
				map[string]interface{}{
					"matcher": "*",
					"hooks": []interface{}{
						map[string]interface{}{"command": "/usr/local/bin/claudio"},
					},
				},
			},
		},
		{
			name: "claudio before existing hook",
			arr: []interface{}{
				map[string]interface{}{
					"matcher": "*",
					"hooks": []interface{}{
						map[string]interface{}{"command": `C:\tools\claudio.exe`},
					},
				},
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{"command": "/usr/bin/logger"},
					},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !IsClaudioHook(tc.arr) {
				t.Error("expected merged hook array to be recognized when any entry is claudio")
			}
		})
	}
}

func TestMergeHooksMarshalErrorPropagates(t *testing.T) {
	// A channel value cannot be JSON-marshaled, forcing deepCopySettings to error.
	bad := &SettingsMap{"x": make(chan int)}
	claudioHooks, _ := GenerateClaudioHooksForAgent("/usr/local/bin/claudio", AgentClaude)
	if _, err := MergeHooksIntoSettings(bad, claudioHooks); err == nil {
		t.Error("expected error when existing settings cannot be deep-copied")
	}
}

func TestMergeHookValuesUnknownExistingFormat(t *testing.T) {
	// Existing PreToolUse hook stored as a number (neither string nor array)
	// exercises the fallback branch in mergeHookValues.
	existing := &SettingsMap{
		"hooks": map[string]interface{}{
			"PreToolUse": float64(42),
		},
	}
	claudioHooks, _ := GenerateClaudioHooksForAgent("/usr/local/bin/claudio", AgentClaude)
	merged, err := MergeHooksIntoSettings(existing, claudioHooks)
	if err != nil {
		t.Fatal(err)
	}
	hooksSection := (*merged)["hooks"].(map[string]interface{})
	if _, ok := hooksSection["PreToolUse"].([]interface{}); !ok {
		t.Errorf("expected PreToolUse coerced to array, got %T", hooksSection["PreToolUse"])
	}
}

func TestFindBestPathReturnsExistingFile(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(orig) }()

	// Codex project scope: ./.codex/hooks.json
	if err := afero.NewOsFs().MkdirAll(".codex", 0755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(afero.NewOsFs(), ".codex/hooks.json", []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := FindBestCodexPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(".codex", "hooks.json") {
		t.Errorf("expected existing codex file, got %q", got)
	}

	// Claude project scope: ./.claude/settings.json
	if err := afero.NewOsFs().MkdirAll(".claude", 0755); err != nil {
		t.Fatal(err)
	}
	if err := afero.WriteFile(afero.NewOsFs(), ".claude/settings.json", []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	cgot, err := FindBestSettingsPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if cgot != filepath.Join(".claude", "settings.json") {
		t.Errorf("expected existing claude file, got %q", cgot)
	}
}
