package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"claudio.click/internal/safeio"
	"github.com/spf13/afero"
)

// MusePluginName is the Muse plugin id claudio installs under.
const MusePluginName = "claudio"

// museManifestPath is the manifest location inside a Muse plugin bundle.
var museManifestPath = filepath.Join(".muse-plugin", "plugin.json")

type museHookCapability struct {
	ID        string   `json:"id"`
	Event     string   `json:"event"`
	Command   []string `json:"command"`
	TimeoutMs int      `json:"timeoutMs"`
}

type museManifest struct {
	SchemaVersion int    `json:"schemaVersion"`
	Name          string `json:"name"`
	DisplayName   string `json:"displayName"`
	Version       string `json:"version"`
	Description   string `json:"description"`
	Compat        struct {
		Source      string `json:"source"`
		ManifestDir string `json:"manifestDir"`
	} `json:"compat"`
	Capabilities struct {
		Skills     []any                `json:"skills"`
		Commands   []any                `json:"commands"`
		Hooks      []museHookCapability `json:"hooks"`
		McpServers []any                `json:"mcpServers"`
		Reminders  []any                `json:"reminders"`
	} `json:"capabilities"`
}

// MuseHookID converts an event name to a portable Muse capability id
// (PreToolUse -> pre-tool-use).
func MuseHookID(event string) string {
	var b strings.Builder
	for i, r := range event {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

// MusePluginManifest returns the Muse native plugin manifest whose
// hooks each run executablePath. Muse passes Claude Code shaped hook
// payloads on stdin, which is exactly what claudio reads, so the hook
// command is claudio itself with no wrapper script.
func MusePluginManifest(executablePath string) []byte {
	m := museManifest{
		SchemaVersion: 1,
		Name:          MusePluginName,
		DisplayName:   "Claudio",
		Version:       "1.0.0",
		Description:   "Play sounds for Muse Code tool use and session events.",
	}
	m.Compat.Source = "native"
	m.Compat.ManifestDir = ".muse-plugin"
	m.Capabilities.Skills = []any{}
	m.Capabilities.Commands = []any{}
	m.Capabilities.McpServers = []any{}
	m.Capabilities.Reminders = []any{}
	for _, hook := range AgentMuse.EnabledHooks() {
		m.Capabilities.Hooks = append(m.Capabilities.Hooks, museHookCapability{
			ID:        MuseHookID(hook.Name),
			Event:     hook.Name,
			Command:   []string{executablePath},
			TimeoutMs: 5000,
		})
	}
	data, _ := json.MarshalIndent(m, "", "  ") // plain data always marshals
	return append(data, '\n')
}

// WriteMusePlugin writes (or rewrites) the claudio Muse plugin bundle in
// dir. It refuses to overwrite a bundle whose manifest names another
// plugin, so install never silently replaces the user's own plugin (which
// a later uninstall would then delete).
func WriteMusePlugin(dir, executablePath string) error {
	manifest := filepath.Join(dir, museManifestPath)
	if _, err := os.Stat(manifest); err == nil && !HasMusePlugin(dir) {
		return fmt.Errorf("refusing to overwrite %s: not written by claudio", manifest)
	}
	if err := os.MkdirAll(filepath.Dir(manifest), 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}
	return safeio.WriteFileAtomic(afero.NewOsFs(), manifest, MusePluginManifest(executablePath), 0o644, "claudio-*.json.tmp")
}

// HasMusePlugin reports whether dir holds a claudio-written Muse plugin.
func HasMusePlugin(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, museManifestPath))
	if err != nil {
		return false
	}
	var m struct {
		Name string `json:"name"`
	}
	return json.Unmarshal(data, &m) == nil && m.Name == MusePluginName
}

// RemoveMusePlugin deletes the bundle in dir when claudio wrote it. A
// missing bundle, or one claudio did not write, is left alone.
func RemoveMusePlugin(dir string) error {
	if !HasMusePlugin(dir) {
		return nil
	}
	return os.RemoveAll(dir)
}
