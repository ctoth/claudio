package config

import (
	"encoding/json"
	"strings"
	"testing"
)

type packFile struct {
	Name     string            `json:"name"`
	Mappings map[string]string `json:"mappings"`
}

func readEmbeddedPack(t *testing.T, name string) packFile {
	t.Helper()
	data, err := GetEmbeddedPlatformSoundpackData(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var p packFile
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return p
}

// The WSL pack is windows.json seen through /mnt/c, so it must cover every
// windows.json key with the same file.
func TestEmbeddedWSLPack_MirrorsWindowsPack(t *testing.T) {
	win := readEmbeddedPack(t, "windows.json")
	wsl := readEmbeddedPack(t, "wsl.json")

	if len(wsl.Mappings) != len(win.Mappings) {
		t.Errorf("wsl.json has %d keys, windows.json has %d", len(wsl.Mappings), len(win.Mappings))
	}
	for key, winPath := range win.Mappings {
		got, ok := wsl.Mappings[key]
		if !ok {
			t.Errorf("wsl.json missing key %s", key)
			continue
		}
		want := "/mnt/c/" + strings.ReplaceAll(winPath[len(`C:\`):], `\`, "/")
		if got != want {
			t.Errorf("wsl.json[%s] = %q, want %q", key, got, want)
		}
	}
	if wsl.Name == "" || wsl.Name == win.Name {
		t.Errorf("wsl pack name %q should be set and distinct from windows %q", wsl.Name, win.Name)
	}
	if !hasEmbeddedPlatformFile("wsl.json") {
		t.Error("hasEmbeddedPlatformFile(wsl.json) = false")
	}
}

func TestWindowsToWSLPath(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Windows\Media\chimes.wav`: "/mnt/c/Windows/Media/chimes.wav",
		`d:\Sounds\a b.wav`:           "/mnt/d/Sounds/a b.wav",
		`D:/x.wav`:                    "/mnt/d/x.wav",
		"/already/posix.wav":          "/already/posix.wav",
		"relative.wav":                "relative.wav",
	} {
		if got := windowsToWSLPath(in); got != want {
			t.Errorf("windowsToWSLPath(%q) = %q, want %q", in, got, want)
		}
	}
}
