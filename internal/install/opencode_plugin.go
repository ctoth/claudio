package install

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claudio.click/internal/safeio"
	"github.com/spf13/afero"
)

// openCodePluginMarker is the first line of every plugin claudio writes, so
// uninstall only ever deletes its own file.
const openCodePluginMarker = "// claudio OpenCode plugin: written by `claudio install`, removed by `claudio uninstall`."

// openCodePluginTemplate forwards OpenCode plugin events to claudio as
// Claude Code shaped hook payloads. __CLAUDIO__ becomes a JS string literal.
const openCodePluginTemplate = openCodePluginMarker + `
import { spawn } from "node:child_process"
import { readFileSync } from "node:fs"
import { dirname, join } from "node:path"
import { fileURLToPath } from "node:url"

const CLAUDIO = __CLAUDIO__

function send(payload) {
  try {
    const child = spawn(CLAUDIO, [], { stdio: ["pipe", "ignore", "ignore"], windowsHide: true })
    child.on("error", () => {})
    child.stdin.on("error", () => {})
    child.stdin.end(JSON.stringify(payload))
    child.unref()
  } catch {}
}

export const ClaudioPlugin = async ({ directory, worktree }) => {
  // Global and project installs both load for the same directory. The
  // project copy wins; the global copy stands down when one exists. This is
  // decided on every load, so project reloads and extra directories keep
  // playing instead of being silenced by a stale process-wide guard.
  for (let dir = directory; dir; dir = dirname(dir)) {
    const projectPlugin = join(dir, ".opencode", "plugins", "claudio.js")
    if (fileURLToPath(import.meta.url) !== projectPlugin) {
      try {
        if (readFileSync(projectPlugin, "utf8").startsWith(__MARKER__)) return {}
      } catch {}
    }
    if (dir === worktree || dirname(dir) === dir) break
  }
  const subagents = new Set()
  const hook = (event, sessionID, extra = {}) =>
    send({ hook_event_name: event, session_id: sessionID || "opencode", cwd: directory, ...extra })

  return {
    "chat.message": async (input) => {
      if (!subagents.has(input.sessionID)) hook("UserPromptSubmit", input.sessionID)
    },
    "tool.execute.before": async (input, output) =>
      hook("PreToolUse", input.sessionID, { tool_name: input.tool, tool_input: output?.args ?? {} }),
    "tool.execute.after": async (input, output) => {
      const exit = output?.metadata?.exit
      hook("PostToolUse", input.sessionID, {
        tool_name: input.tool,
        tool_input: input.args ?? {},
        tool_response: typeof exit === "number" ? "Exit code: " + exit : "",
      })
    },
    event: async ({ event }) => {
      const p = event.properties ?? {}
      switch (event.type) {
        case "session.created":
          if (p.info?.parentID) {
            subagents.add(p.info.id)
            hook("SubagentStart", p.info.id)
          } else hook("SessionStart", p.info?.id)
          break
        case "session.idle":
          hook(subagents.has(p.sessionID) ? "SubagentStop" : "Stop", p.sessionID)
          break
        case "session.error":
          // Esc interrupts surface as MessageAbortedError; only real
          // failures should play the error sound.
          if (p.error?.name !== "MessageAbortedError") hook("StopFailure", p.sessionID)
          break
        case "session.compacted":
          hook("PostCompact", p.sessionID)
          break
        case "permission.asked":
        case "permission.updated":
          hook("PermissionRequest", p.sessionID)
          break
      }
    },
  }
}
`

// OpenCodePluginSource returns the plugin source that runs executablePath.
func OpenCodePluginSource(executablePath string) string {
	literal, _ := json.Marshal(executablePath) // a string always marshals
	marker, _ := json.Marshal(openCodePluginMarker + "\n")
	source := strings.Replace(openCodePluginTemplate, "__CLAUDIO__", string(literal), 1)
	return strings.Replace(source, "__MARKER__", string(marker), 1)
}

// WriteOpenCodePlugin writes (or rewrites) the claudio plugin at path. It
// refuses to overwrite a claudio.js that claudio did not write, so install
// never silently replaces the user's own plugin (which a later uninstall
// would then delete).
func WriteOpenCodePlugin(path, executablePath string) error {
	if _, err := os.Stat(path); err == nil && !HasOpenCodePlugin(path) {
		return fmt.Errorf("refusing to overwrite %s: not written by claudio", path)
	}
	source := OpenCodePluginSource(executablePath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}
	return safeio.WriteFileAtomic(afero.NewOsFs(), path, []byte(source), 0o644, "claudio-*.js.tmp")
}

// HasOpenCodePlugin reports whether path holds a claudio-written plugin.
func HasOpenCodePlugin(path string) bool {
	data, err := os.ReadFile(path)
	return err == nil && bytes.HasPrefix(data, []byte(openCodePluginMarker))
}

// RemoveOpenCodePlugin deletes the plugin at path when claudio wrote it. A
// missing file, or one claudio did not write, is left alone.
func RemoveOpenCodePlugin(path string) error {
	if !HasOpenCodePlugin(path) {
		return nil
	}
	return os.Remove(path)
}
