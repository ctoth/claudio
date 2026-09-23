package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// wslPackFile is the embedded WSL platform pack. It is not a file on disk:
// WSL plays the same Windows system sounds as windows.json, reached through
// the /mnt/<drive> mounts, so it is derived from windows.json and can never
// fall behind it.
const wslPackFile = "wsl.json"

const (
	wslPackName        = "windows-media-wsl-soundpack"
	wslPackDescription = "Windows Media soundpack for WSL (windows.json via /mnt/c)"
)

// wslPackData derives wsl.json once per process.
var wslPackData = sync.OnceValues(func() ([]byte, error) {
	windows, err := platformSoundpacks.ReadFile("windows.json")
	if err != nil {
		return nil, fmt.Errorf("read embedded windows.json: %w", err)
	}
	return deriveWSLPack(windows)
})

// deriveWSLPack rewrites every mapping of a Windows JSON soundpack to its
// WSL mount path and renames the pack.
func deriveWSLPack(windowsData []byte) ([]byte, error) {
	var pack struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Version     string            `json:"version"`
		Mappings    map[string]string `json:"mappings"`
	}
	if err := json.Unmarshal(windowsData, &pack); err != nil {
		return nil, fmt.Errorf("parse windows.json: %w", err)
	}
	pack.Name = wslPackName
	pack.Description = wslPackDescription
	for key, path := range pack.Mappings {
		pack.Mappings[key] = windowsToWSLPath(path)
	}
	return json.MarshalIndent(pack, "", "  ")
}

// windowsToWSLPath maps a drive-letter path (C:\a\b or C:/a/b) to its WSL
// mount (/mnt/c/a/b). Other paths are returned unchanged.
func windowsToWSLPath(path string) string {
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') {
		return path
	}
	drive := path[0] | 0x20 // ASCII lower-case
	if drive < 'a' || drive > 'z' {
		return path
	}
	return "/mnt/" + string(drive) + "/" + strings.ReplaceAll(path[3:], `\`, "/")
}
