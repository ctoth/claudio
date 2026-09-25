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

export const ClaudioPlugin = async ({ directory }) => {
  // Global and project installs both load; only the first one plays.
  if (globalThis.__claudioPlugin) return {}
  globalThis.__claudioPlugin = true
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
          hook("StopFailure", p.sessionID)
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
	return strings.Replace(openCodePluginTemplate, "__CLAUDIO__", string(literal), 1)
}

// WriteOpenCodePlugin writes (or rewrites) the claudio plugin at path.
func WriteOpenCodePlugin(path, executablePath string) error {
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
