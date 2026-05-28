# Codex Hook Support Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let Claudio play contextual sounds for OpenAI Codex CLI by tolerating Codex's hook payloads and adding an agent-aware installer that writes `~/.codex/hooks.json`.

**Architecture:** The core pipeline (parse → `GetContext` → `SoundMapper` → audio) is reused unchanged because Codex hook JSON shares field names with Claude Code. Two changes: the hook parser gains tolerance plus two new event cases, and the install/uninstall layer becomes agent-aware via a new `Agent` type that selects the config path, hook registry, and matcher.

**Tech Stack:** Go, cobra (CLI), afero (filesystem abstraction in tests), slog (logging). Existing JSON read/merge/write helpers are reused for `hooks.json`.

---

## Background facts the implementer needs

- At runtime, `internal/cli/cli.go` `processHookInput` does `json.Unmarshal` straight into `hooks.HookEvent`, then calls `GetContext()`. It does **not** call `hooks.Parse()`, and it only requires `hook_event_name` and `session_id`. So the runtime never rejects a null `transcript_path`. The fix to `Parse()` is for consistency and its own tests.
- `hooks.HookEvent` already has `ToolName`, `ToolInput`, `ToolResponse`, `Prompt`, `Message` as optional pointers — Codex's payload unmarshals into it with no struct changes.
- `mcp__<server>__<tool>` normalization to `mcp` already exists in `GetContext` (prior commit on this branch). Add a regression test only.
- Codex `hooks.json` top-level shape is `{"hooks": {EventName: [ {"matcher": "...", "hooks": [{"type":"command","command":"..."}]} ]}}`. This is a JSON object with a `hooks` key, so the existing `ReadSettingsFile` / `MergeHooksIntoSettings` / `WriteSettingsFile` operate on it unchanged.
- `internal/uninstall` is agent-agnostic: it detects Claudio entries by command basename via `install.IsClaudioHook`. Only the config path differs per agent.
- The `Agent` type lives in package `install`. Package `cli` imports `install` and uses `install.Agent`, `install.ParseAgent`, etc.

---

## File structure

- Modify `internal/hooks/parser.go` — relax `transcript_path` requirement in `Parse`; add `SubagentStart` and `PostCompact` cases in `GetContext`.
- Modify `internal/hooks/parser_test.go` — Codex payload tests.
- Create `internal/install/agent.go` — `Agent` type, `ParseAgent`, per-agent matcher / registry / enabled-hooks / hook-names / best-config-path.
- Create `internal/install/agent_test.go` — agent unit tests.
- Create `internal/install/codex_settings.go` — `FindCodexHooksPaths` / `FindBestCodexPath`.
- Create `internal/install/codex_settings_test.go` — path finder tests.
- Modify `internal/install/hook_registry.go` — add `CodexHooks` registry.
- Modify `internal/install/hook_registry_test.go` — Codex registry tests.
- Modify `internal/install/hooks.go` — add `GenerateClaudioHooksForAgent`; keep `GenerateClaudioHooks` as a Claude wrapper.
- Modify `internal/install/hooks_test.go` — agent generation tests.
- Modify `internal/cli/install_command.go` — `--agent` flag, agent-aware workflow, Codex trust reminder.
- Modify `internal/cli/install_command_test.go` (or `install_flags_test.go`) — flag + workflow tests.
- Modify `internal/cli/uninstall_command.go` — `--agent` flag, agent-aware path selection.
- Modify `internal/cli/uninstall_command_test.go` — flag tests.

---

## Task 1: Relax `transcript_path` requirement in parser

**Files:**
- Modify: `internal/hooks/parser.go:124-128`
- Test: `internal/hooks/parser_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/hooks/parser_test.go`:

```go
func TestParseCodexNullTranscriptPathSucceeds(t *testing.T) {
	parser := NewHookEventParser()
	// Codex sends transcript_path as null
	data := []byte(`{"session_id":"abc","cwd":"/tmp","hook_event_name":"SessionStart","transcript_path":null}`)

	event, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("expected nil error for null transcript_path, got: %v", err)
	}
	if event.EventName != "SessionStart" {
		t.Errorf("expected SessionStart, got %q", event.EventName)
	}
}

func TestParseCodexOmittedTranscriptPathSucceeds(t *testing.T) {
	parser := NewHookEventParser()
	data := []byte(`{"session_id":"abc","cwd":"/tmp","hook_event_name":"Stop"}`)

	_, err := parser.Parse(data)
	if err != nil {
		t.Fatalf("expected nil error for omitted transcript_path, got: %v", err)
	}
}

func TestParseStillRequiresSessionIDAndEventAndCwd(t *testing.T) {
	parser := NewHookEventParser()
	cases := map[string][]byte{
		"missing session_id": []byte(`{"cwd":"/tmp","hook_event_name":"Stop"}`),
		"missing event":      []byte(`{"session_id":"a","cwd":"/tmp"}`),
		"missing cwd":        []byte(`{"session_id":"a","hook_event_name":"Stop"}`),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parser.Parse(data); err == nil {
				t.Errorf("expected error for %s, got nil", name)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run TestParseCodex -v`
Expected: FAIL — `TestParseCodexNullTranscriptPathSucceeds` and `TestParseCodexOmittedTranscriptPathSucceeds` fail with "missing required field: transcript_path".

- [ ] **Step 3: Remove the transcript_path validation block**

In `internal/hooks/parser.go`, delete this block (currently lines 124-128):

```go
	if event.TranscriptPath == "" {
		err := fmt.Errorf("missing required field: transcript_path")
		slog.Error("validation failed", "error", err)
		return nil, err
	}
```

Leave the `session_id`, `hook_event_name`, and `cwd` checks intact.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/hooks/ -run TestParse -v`
Expected: PASS (all parse tests, including pre-existing ones).

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/parser.go internal/hooks/parser_test.go
git commit -m "feat: accept hook events without transcript_path for codex"
```

---

## Task 2: Map Codex-only events SubagentStart and PostCompact

**Files:**
- Modify: `internal/hooks/parser.go` (inside `GetContext`, after the `SubagentStop` case, before `PreCompact`)
- Test: `internal/hooks/parser_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/hooks/parser_test.go`:

```go
func TestGetContextSubagentStart(t *testing.T) {
	event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "SubagentStart"}
	ctx := event.GetContext()
	if ctx.Category != Loading {
		t.Errorf("expected Loading, got %v", ctx.Category)
	}
	if ctx.SoundHint != "subagent-start" {
		t.Errorf("expected subagent-start, got %q", ctx.SoundHint)
	}
	if ctx.Operation != "subagent-start" {
		t.Errorf("expected operation subagent-start, got %q", ctx.Operation)
	}
}

func TestGetContextPostCompact(t *testing.T) {
	event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostCompact"}
	ctx := event.GetContext()
	if ctx.Category != System {
		t.Errorf("expected System, got %v", ctx.Category)
	}
	if ctx.SoundHint != "post-compact" {
		t.Errorf("expected post-compact, got %q", ctx.SoundHint)
	}
	if ctx.Operation != "post-compact" {
		t.Errorf("expected operation post-compact, got %q", ctx.Operation)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/hooks/ -run "TestGetContextSubagentStart|TestGetContextPostCompact" -v`
Expected: FAIL — both hit the `default` case, so `SoundHint` is `"default"` and `Category` is `Interactive`.

- [ ] **Step 3: Add the two cases**

In `internal/hooks/parser.go`, inside the `switch e.EventName` block in `GetContext`, add these cases immediately after the existing `case "SubagentStop":` block:

```go
	case "SubagentStart":
		context.Category = Loading
		context.SoundHint = "subagent-start"
		context.Operation = "subagent-start"
		slog.Debug("categorizing SubagentStart event as Loading", "hint", context.SoundHint, "operation", context.Operation)

	case "PostCompact":
		context.Category = System
		context.SoundHint = "post-compact"
		context.Operation = "post-compact"
		slog.Debug("categorizing PostCompact event as System", "hint", context.SoundHint, "operation", context.Operation)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/hooks/ -run "TestGetContext" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/parser.go internal/hooks/parser_test.go
git commit -m "feat: map codex SubagentStart and PostCompact events"
```

---

## Task 3: Codex tool-name fixtures (apply_patch, mcp regression)

**Files:**
- Test only: `internal/hooks/parser_test.go` (no production change expected; if a test fails, the fix belongs in `GetContext`)

- [ ] **Step 1: Write the tests**

Add to `internal/hooks/parser_test.go`:

```go
func TestGetContextCodexApplyPatchPreToolUse(t *testing.T) {
	tool := "apply_patch"
	event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PreToolUse", ToolName: &tool}
	ctx := event.GetContext()
	if ctx.Category != Loading {
		t.Errorf("expected Loading, got %v", ctx.Category)
	}
	if ctx.SoundHint != "apply_patch-start" {
		t.Errorf("expected apply_patch-start, got %q", ctx.SoundHint)
	}
}

func TestGetContextCodexApplyPatchPostToolUseSuccess(t *testing.T) {
	tool := "apply_patch"
	resp := json.RawMessage(`{"output":"done"}`)
	event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolName: &tool, ToolResponse: &resp}
	ctx := event.GetContext()
	if ctx.Category != Success {
		t.Errorf("expected Success, got %v", ctx.Category)
	}
	if ctx.SoundHint != "apply_patch-success" {
		t.Errorf("expected apply_patch-success, got %q", ctx.SoundHint)
	}
}

func TestGetContextCodexMcpToolNormalized(t *testing.T) {
	tool := "mcp__filesystem__read_file"
	event := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PreToolUse", ToolName: &tool}
	ctx := event.GetContext()
	if ctx.ToolName != "mcp" {
		t.Errorf("expected normalized tool name mcp, got %q", ctx.ToolName)
	}
	if ctx.SoundHint != "mcp-start" {
		t.Errorf("expected mcp-start, got %q", ctx.SoundHint)
	}
}
```

Note: `parser_test.go` must already import `encoding/json`. If not, add it.

- [ ] **Step 2: Run tests**

Run: `go test ./internal/hooks/ -run "Codex" -v`
Expected: PASS (these exercise existing generic and mcp paths). If `apply_patch` fails because of unexpected casing, the production fix is to ensure `GetContext` lowercases the hint — but `strings.ToLower("apply_patch")` is `apply_patch`, so it should pass as written.

- [ ] **Step 3: Commit**

```bash
git add internal/hooks/parser_test.go
git commit -m "test: cover codex apply_patch and mcp tool mapping"
```

---

## Task 4: Add the `Agent` type and `ParseAgent`

**Files:**
- Create: `internal/install/agent.go`
- Test: `internal/install/agent_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/install/agent_test.go`:

```go
package install

import "testing"

func TestParseAgentValid(t *testing.T) {
	cases := map[string]Agent{
		"claude": AgentClaude,
		"codex":  AgentCodex,
	}
	for in, want := range cases {
		got, err := ParseAgent(in)
		if err != nil {
			t.Fatalf("ParseAgent(%q) returned error: %v", in, err)
		}
		if got != want {
			t.Errorf("ParseAgent(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestParseAgentInvalid(t *testing.T) {
	if _, err := ParseAgent("gemini"); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestAgentMatcher(t *testing.T) {
	if AgentClaude.Matcher() != ".*" {
		t.Errorf("claude matcher = %q, want .*", AgentClaude.Matcher())
	}
	if AgentCodex.Matcher() != "*" {
		t.Errorf("codex matcher = %q, want *", AgentCodex.Matcher())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run "TestParseAgent|TestAgentMatcher" -v`
Expected: FAIL — `Agent`, `AgentClaude`, `AgentCodex`, `ParseAgent`, `Matcher` undefined (build error).

- [ ] **Step 3: Write the implementation**

Create `internal/install/agent.go`:

```go
package install

import (
	"fmt"
	"log/slog"
)

// Agent identifies which coding agent Claudio installs hooks for.
type Agent string

const (
	AgentClaude Agent = "claude"
	AgentCodex  Agent = "codex"
)

// ParseAgent validates and converts a string into an Agent.
func ParseAgent(s string) (Agent, error) {
	switch Agent(s) {
	case AgentClaude, AgentCodex:
		return Agent(s), nil
	default:
		return "", fmt.Errorf("invalid agent '%s': must be 'claude' or 'codex'", s)
	}
}

// String returns the agent's string form.
func (a Agent) String() string { return string(a) }

// Matcher returns the default hook matcher pattern for the agent.
// Codex uses "*"; Claude Code uses ".*".
func (a Agent) Matcher() string {
	if a == AgentCodex {
		return "*"
	}
	return ".*"
}

// Registry returns the hook definitions supported for the agent.
func (a Agent) Registry() []HookDefinition {
	if a == AgentCodex {
		return CodexHooks
	}
	return AllHooks
}

// EnabledHooks returns the agent's default-enabled hook definitions.
func (a Agent) EnabledHooks() []HookDefinition {
	var enabled []HookDefinition
	for _, h := range a.Registry() {
		if h.DefaultEnabled {
			enabled = append(enabled, h)
		}
	}
	slog.Debug("agent enabled hooks", "agent", a, "count", len(enabled))
	return enabled
}

// HookNames returns the names of every hook in the agent's registry.
func (a Agent) HookNames() []string {
	reg := a.Registry()
	names := make([]string, len(reg))
	for i, h := range reg {
		names[i] = h.Name
	}
	return names
}

// BestConfigPath returns the config file path to install hooks into for the agent and scope.
func (a Agent) BestConfigPath(scope string) (string, error) {
	if a == AgentCodex {
		return FindBestCodexPath(scope)
	}
	return FindBestSettingsPath(scope)
}
```

Note: `CodexHooks` and `FindBestCodexPath` are defined in Tasks 5 and 6. This file will not compile until those exist; that is expected in TDD — implement them in the next tasks before running the full package build. To keep this task self-contained and green, implement Tasks 5 and 6 stubs first if running strictly per-task; otherwise proceed in order.

- [ ] **Step 4: Defer full run**

The package build depends on Tasks 5 and 6. Run `go vet ./internal/install/` after Task 6 instead. For now:

Run: `go build ./internal/install/ 2>&1 | head`
Expected: build errors referencing `CodexHooks` and `FindBestCodexPath` (resolved in Tasks 5-6).

- [ ] **Step 5: Commit after Task 6**

This task commits together with Tasks 5 and 6 since they are mutually dependent (see Task 6 Step 5).

---

## Task 5: Codex config path finder

**Files:**
- Create: `internal/install/codex_settings.go`
- Test: `internal/install/codex_settings_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/install/codex_settings_test.go`:

```go
package install

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestFindCodexHooksPathsUserScope(t *testing.T) {
	paths, err := FindCodexHooksPaths("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one user-scope path")
	}
	// Every candidate must end with .codex/hooks.json
	want := filepath.Join(".codex", "hooks.json")
	for _, p := range paths {
		if !strings.HasSuffix(p, want) {
			t.Errorf("path %q does not end with %q", p, want)
		}
	}
}

func TestFindCodexHooksPathsProjectScope(t *testing.T) {
	paths, err := FindCodexHooksPaths("project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("expected at least one project-scope path")
	}
	if !strings.Contains(paths[0], filepath.Join(".codex", "hooks.json")) {
		t.Errorf("project path %q missing .codex/hooks.json", paths[0])
	}
}

func TestFindCodexHooksPathsInvalidScope(t *testing.T) {
	if _, err := FindCodexHooksPaths("bogus"); err == nil {
		t.Error("expected error for invalid scope, got nil")
	}
}

func TestFindBestCodexPathReturnsFirstWhenNoneExist(t *testing.T) {
	// In CI/dev there is usually no ~/.codex/hooks.json; first candidate is returned for creation.
	got, err := FindBestCodexPath("user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == "" {
		t.Fatal("expected a non-empty path")
	}
	_ = runtime.GOOS // platform-specific home handling is exercised indirectly
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run "Codex" -v`
Expected: FAIL — `FindCodexHooksPaths` / `FindBestCodexPath` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/install/codex_settings.go`. It reuses the existing `getHomeDirectory()` helper from `claude_settings.go`:

```go
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// FindCodexHooksPaths returns candidate ~/.codex/hooks.json paths for the scope, in priority order.
func FindCodexHooksPaths(scope string) ([]string, error) {
	switch scope {
	case "user":
		return findCodexUserScopePaths(), nil
	case "project":
		return []string{
			filepath.Join(".", ".codex", "hooks.json"),
			filepath.Join(".codex", "hooks.json"),
		}, nil
	default:
		return nil, fmt.Errorf("invalid scope '%s': must be 'user' or 'project'", scope)
	}
}

func findCodexUserScopePaths() []string {
	var paths []string

	homeDir := getHomeDirectory()
	if homeDir != "" {
		paths = append(paths, filepath.Join(homeDir, ".codex", "hooks.json"))
	}

	if runtime.GOOS == "windows" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" && userProfile != homeDir {
			paths = append(paths, filepath.Join(userProfile, ".codex", "hooks.json"))
		}
	}

	if len(paths) == 0 {
		paths = append(paths, filepath.Join("~", ".codex", "hooks.json"))
	}

	return paths
}

// FindBestCodexPath returns the first existing Codex hooks path, or the first candidate for creation.
func FindBestCodexPath(scope string) (string, error) {
	paths, err := FindCodexHooksPaths(scope)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no codex hooks paths found for scope: %s", scope)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return paths[0], nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/install/ -run "Codex" -v`
Expected: PASS for the path-finder tests (agent tests still need Task 6's `CodexHooks`).

- [ ] **Step 5: Commit after Task 6** (mutually dependent with Tasks 4 and 6).

---

## Task 6: Codex hook registry

**Files:**
- Modify: `internal/install/hook_registry.go` (append the `CodexHooks` var)
- Test: `internal/install/hook_registry_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/install/hook_registry_test.go`:

```go
func TestCodexRegistryContents(t *testing.T) {
	want := map[string]bool{
		"PreToolUse": true, "PostToolUse": true, "UserPromptSubmit": true,
		"Stop": true, "SubagentStop": true, "SubagentStart": true,
		"PreCompact": true, "PostCompact": true, "SessionStart": true,
		"PermissionRequest": true,
	}
	got := map[string]bool{}
	for _, h := range CodexHooks {
		got[h.Name] = true
	}
	if len(got) != len(want) {
		t.Errorf("codex registry has %d events, want %d", len(got), len(want))
	}
	for name := range want {
		if !got[name] {
			t.Errorf("codex registry missing %q", name)
		}
	}
	// Codex has no Notification or SessionEnd
	if got["Notification"] || got["SessionEnd"] {
		t.Error("codex registry must not contain Notification or SessionEnd")
	}
}

func TestAgentEnabledHooksAndNames(t *testing.T) {
	if len(AgentCodex.EnabledHooks()) != len(CodexHooks) {
		t.Errorf("expected all codex hooks enabled by default")
	}
	if len(AgentCodex.HookNames()) != 10 {
		t.Errorf("expected 10 codex hook names, got %d", len(AgentCodex.HookNames()))
	}
	if len(AgentClaude.HookNames()) != len(AllHooks) {
		t.Errorf("claude hook names mismatch")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run "TestCodexRegistry|TestAgentEnabledHooks" -v`
Expected: FAIL — `CodexHooks` undefined.

- [ ] **Step 3: Write the implementation**

Append to `internal/install/hook_registry.go` (after the `AllHooks` var):

```go
// CodexHooks defines the registry of OpenAI Codex CLI hooks supported by Claudio.
// Codex lacks Notification and SessionEnd; it adds SubagentStart and PostCompact.
var CodexHooks = []HookDefinition{
	{Name: "PreToolUse", Category: hooks.Loading, Description: "Play loading sounds before Codex tool execution", DefaultEnabled: true},
	{Name: "PostToolUse", Category: hooks.Success, Description: "Play success/error sounds after Codex tool execution", DefaultEnabled: true},
	{Name: "UserPromptSubmit", Category: hooks.Interactive, Description: "Play interaction sounds when user submits prompts", DefaultEnabled: true},
	{Name: "Stop", Category: hooks.Completion, Description: "Play sounds when Codex finishes responding", DefaultEnabled: true},
	{Name: "SubagentStop", Category: hooks.Completion, Description: "Play sounds when a Codex subagent finishes", DefaultEnabled: true},
	{Name: "SubagentStart", Category: hooks.Loading, Description: "Play sounds when a Codex subagent starts", DefaultEnabled: true},
	{Name: "PreCompact", Category: hooks.System, Description: "Play sounds before Codex context compaction", DefaultEnabled: true},
	{Name: "PostCompact", Category: hooks.System, Description: "Play sounds after Codex context compaction", DefaultEnabled: true},
	{Name: "SessionStart", Category: hooks.System, Description: "Play sounds when a Codex session starts or resumes", DefaultEnabled: true},
	{Name: "PermissionRequest", Category: hooks.Interactive, Description: "Play sounds for Codex permission requests", DefaultEnabled: true},
}
```

- [ ] **Step 4: Run the full install package build and the new tests**

Run: `go test ./internal/install/ -run "TestParseAgent|TestAgentMatcher|TestCodex|TestAgentEnabledHooks|TestFindCodex|TestFindBestCodex" -v`
Expected: PASS (Tasks 4, 5, 6 now compile together).

- [ ] **Step 5: Commit Tasks 4-6 together**

```bash
git add internal/install/agent.go internal/install/agent_test.go \
        internal/install/codex_settings.go internal/install/codex_settings_test.go \
        internal/install/hook_registry.go internal/install/hook_registry_test.go
git commit -m "feat: add codex agent type, registry, and config path finder"
```

---

## Task 7: Agent-aware hook generation

**Files:**
- Modify: `internal/install/hooks.go:21-63` (`GenerateClaudioHooks`)
- Test: `internal/install/hooks_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/install/hooks_test.go`:

```go
func TestGenerateClaudioHooksForCodexAgent(t *testing.T) {
	fsys := afero.NewMemMapFs()
	result, err := GenerateClaudioHooksForAgent(fsys, "/usr/local/bin/claudio", AgentCodex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hooks, ok := result.(HooksMap)
	if !ok {
		t.Fatalf("expected HooksMap, got %T", result)
	}
	if len(hooks) != len(CodexHooks) {
		t.Errorf("expected %d codex hooks, got %d", len(CodexHooks), len(hooks))
	}
	if _, ok := hooks["PostCompact"]; !ok {
		t.Error("expected PostCompact in codex hooks")
	}
	if _, ok := hooks["Notification"]; ok {
		t.Error("codex hooks must not include Notification")
	}
	// Codex matcher must be "*"
	arr := hooks["Stop"].([]interface{})
	cfg := arr[0].(map[string]interface{})
	if cfg["matcher"] != "*" {
		t.Errorf("codex matcher = %v, want *", cfg["matcher"])
	}
}

func TestGenerateClaudioHooksDefaultsToClaude(t *testing.T) {
	fsys := afero.NewMemMapFs()
	result, err := GenerateClaudioHooks(fsys, "/usr/local/bin/claudio")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hooks := result.(HooksMap)
	if len(hooks) != len(GetEnabledHooks()) {
		t.Errorf("claude generation count mismatch")
	}
	arr := hooks["PreToolUse"].([]interface{})
	cfg := arr[0].(map[string]interface{})
	if cfg["matcher"] != ".*" {
		t.Errorf("claude matcher = %v, want .*", cfg["matcher"])
	}
}
```

Note: `hooks_test.go` must import `github.com/spf13/afero`. The existing afero-based tests already do; confirm the import is present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/install/ -run "TestGenerateClaudioHooksForCodexAgent|TestGenerateClaudioHooksDefaultsToClaude" -v`
Expected: FAIL — `GenerateClaudioHooksForAgent` undefined.

- [ ] **Step 3: Refactor `GenerateClaudioHooks` and add the agent variant**

In `internal/install/hooks.go`, replace the body of `GenerateClaudioHooks` (lines 21-63) with a thin wrapper and a new agent-aware function:

```go
// GenerateClaudioHooks creates the Claude Code hook configuration (backward-compatible default).
func GenerateClaudioHooks(filesystem afero.Fs, executablePath string) (interface{}, error) {
	return GenerateClaudioHooksForAgent(filesystem, executablePath, AgentClaude)
}

// GenerateClaudioHooksForAgent creates hook configuration for the given agent.
// Returns a hooks map suitable for Claude settings.json or Codex hooks.json.
func GenerateClaudioHooksForAgent(filesystem afero.Fs, executablePath string, agent Agent) (interface{}, error) {
	slog.Debug("generating Claudio hooks configuration",
		"agent", agent, "executable_path", executablePath)

	enabledHooks := agent.EnabledHooks()
	matcher := agent.Matcher()
	slog.Debug("retrieved enabled hooks for agent", "agent", agent, "count", len(enabledHooks))

	hooks := make(HooksMap)

	createHookConfig := func() interface{} {
		return []interface{}{
			map[string]interface{}{
				"matcher": matcher,
				"hooks": []interface{}{
					map[string]interface{}{
						"type":    "command",
						"command": executablePath,
					},
				},
			},
		}
	}

	for _, hookDef := range enabledHooks {
		hooks[hookDef.Name] = createHookConfig()
		slog.Debug("added hook from registry",
			"agent", agent, "hook_name", hookDef.Name, "category", hookDef.Category)
	}

	slog.Info("generated Claudio hooks configuration",
		"agent", agent, "hook_count", len(hooks), "hooks", getHookNamesList(hooks))

	return hooks, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/install/ -run "TestGenerateClaudioHooks" -v`
Expected: PASS (new and pre-existing generation tests).

- [ ] **Step 5: Commit**

```bash
git add internal/install/hooks.go internal/install/hooks_test.go
git commit -m "feat: generate agent-specific claudio hooks"
```

---

## Task 8: Agent-aware install command

**Files:**
- Modify: `internal/cli/install_command.go`
- Test: `internal/cli/install_command_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cli/install_command_test.go`:

```go
func TestInstallCommandRejectsInvalidAgent(t *testing.T) {
	cmd := newInstallCommand()
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestInstallCommandCodexDryRunShowsTrustReminder(t *testing.T) {
	cmd := newInstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "hooks.json") {
		t.Errorf("expected codex hooks.json path in output, got: %s", s)
	}
	if !strings.Contains(s, "/hooks") {
		t.Errorf("expected /hooks trust reminder in output, got: %s", s)
	}
}
```

Note: ensure imports `bytes`, `io`, `strings` exist in the test file.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run "TestInstallCommandRejectsInvalidAgent|TestInstallCommandCodexDryRun" -v`
Expected: FAIL — `--agent` flag unknown (cobra errors) / trust reminder absent.

- [ ] **Step 3: Add the `--agent` flag**

In `newInstallCommand()` in `internal/cli/install_command.go`, after the `--scope` flag registration, add:

```go
	// Add --agent flag with validation
	cmd.Flags().StringP("agent", "a", "claude", "Target agent: 'claude' for Claude Code, 'codex' for OpenAI Codex CLI")
```

- [ ] **Step 4: Wire the flag through `runInstallCommandE`**

In `runInstallCommandE`, after the scope validation block and before `dryRun` retrieval, add agent parsing:

```go
	// Get and validate agent flag
	agentStr, err := cmd.Flags().GetString("agent")
	if err != nil {
		return fmt.Errorf("failed to get agent flag: %w", err)
	}
	agent, err := install.ParseAgent(agentStr)
	if err != nil {
		return err
	}
```

Replace the settings-path lookup:

```go
	settingsPath, err := install.FindBestSettingsPath(scope.String())
	if err != nil {
		return fmt.Errorf("failed to find Claude Code settings path: %w", err)
	}
```

with:

```go
	settingsPath, err := agent.BestConfigPath(scope.String())
	if err != nil {
		return fmt.Errorf("failed to find %s config path: %w", agent, err)
	}
```

In the dry-run branch, replace `hookNames := install.GetHookNames()` with `hookNames := agent.HookNames()`.

After the dry-run "Would install hooks" lines, add the Codex trust reminder inside the `!quiet` branch:

```go
		if agent == install.AgentCodex {
			cmd.Printf("After install, run /hooks in Codex to trust the claudio hook.\n")
		}
```

Update the actual-install call:

```go
	err = runInstallWorkflow(scope.String(), settingsPath)
```

to:

```go
	err = runInstallWorkflow(agent, scope.String(), settingsPath)
```

And in the post-success `!quiet` block, after the existing success lines, add:

```go
		if agent == install.AgentCodex {
			cmd.Printf("Run /hooks in Codex to trust the claudio hook.\n")
		}
```

- [ ] **Step 5: Update `runInstallWorkflow` to take the agent**

Change the signature and the two registry calls in `runInstallWorkflow`:

```go
func runInstallWorkflow(agent install.Agent, scope string, settingsPath string) error {
```

Replace `claudioHooks, err := install.GenerateClaudioHooks(prodFS, execPath)` with:

```go
	claudioHooks, err := install.GenerateClaudioHooksForAgent(prodFS, execPath, agent)
```

Replace the verification loop's `expectedHooks := install.GetHookNames()` with:

```go
		expectedHooks := agent.HookNames()
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestInstallCommand" -v`
Expected: PASS. (Full cli build requires the working cgo linker fixed earlier.)

- [ ] **Step 7: Commit**

```bash
git add internal/cli/install_command.go internal/cli/install_command_test.go
git commit -m "feat: add --agent flag to install with codex trust reminder"
```

---

## Task 9: Agent-aware uninstall command

**Files:**
- Modify: `internal/cli/uninstall_command.go`
- Test: `internal/cli/uninstall_command_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/cli/uninstall_command_test.go`:

```go
func TestUninstallCommandRejectsInvalidAgent(t *testing.T) {
	cmd := newUninstallCommand()
	cmd.SetArgs([]string{"--agent", "gemini", "--dry-run"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid agent, got nil")
	}
}

func TestUninstallCommandCodexDryRunUsesCodexPath(t *testing.T) {
	cmd := newUninstallCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--agent", "codex", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "hooks.json") {
		t.Errorf("expected codex hooks.json path, got: %s", out.String())
	}
}
```

Note: ensure imports `bytes`, `io`, `strings` exist.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run "TestUninstallCommandRejectsInvalidAgent|TestUninstallCommandCodexDryRun" -v`
Expected: FAIL — `--agent` flag unknown / Claude path used instead of codex.

- [ ] **Step 3: Add the `--agent` flag**

In `newUninstallCommand()`, after the `--scope` flag, add:

```go
	cmd.Flags().StringP("agent", "a", "claude", "Target agent: 'claude' for Claude Code, 'codex' for OpenAI Codex CLI")
```

- [ ] **Step 4: Wire the flag through `runUninstallCommandE`**

After scope validation, add:

```go
	agentStr, err := cmd.Flags().GetString("agent")
	if err != nil {
		return fmt.Errorf("failed to get agent flag: %w", err)
	}
	agent, err := install.ParseAgent(agentStr)
	if err != nil {
		return err
	}
```

Replace:

```go
	settingsPath, err := install.FindBestSettingsPath(scope.String())
	if err != nil {
		return fmt.Errorf("failed to find Claude Code settings path: %w", err)
	}
```

with:

```go
	settingsPath, err := agent.BestConfigPath(scope.String())
	if err != nil {
		return fmt.Errorf("failed to find %s config path: %w", agent, err)
	}
```

The uninstall workflow itself is agent-agnostic (it detects Claudio entries by command basename), so `uninstall.RunUninstallWorkflow(scope.String(), settingsPath)` is unchanged.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/cli/ -run "TestUninstallCommand" -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/uninstall_command.go internal/cli/uninstall_command_test.go
git commit -m "feat: add --agent flag to uninstall command"
```

---

## Task 10: End-to-end Codex install into hooks.json (afero integration)

**Files:**
- Test: `internal/install/hooks_test.go` (add an integration-style test using the in-memory filesystem and the existing merge/read/write helpers)

- [ ] **Step 1: Write the test**

Add to `internal/install/hooks_test.go`:

```go
func TestCodexInstallMergesIntoHooksJSON(t *testing.T) {
	fsys := afero.NewMemMapFs()
	path := "/home/u/.codex/hooks.json"

	// Pre-existing user hook that is NOT claudio — must be preserved.
	existing := []byte(`{"hooks":{"PreToolUse":[{"matcher":"*","hooks":[{"type":"command","command":"/usr/bin/logger"}]}]}}`)
	if err := afero.WriteFile(fsys, path, existing, 0644); err != nil {
		t.Fatal(err)
	}

	settings, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatal(err)
	}
	codexHooks, err := GenerateClaudioHooksForAgent(fsys, "/usr/local/bin/claudio", AgentCodex)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := MergeHooksIntoSettings(settings, codexHooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteSettingsFile(fsys, path, merged); err != nil {
		t.Fatal(err)
	}

	readBack, err := ReadSettingsFile(fsys, path)
	if err != nil {
		t.Fatal(err)
	}
	hooksSection := (*readBack)["hooks"].(map[string]interface{})

	// Claudio's PostCompact must be present.
	if _, ok := hooksSection["PostCompact"]; !ok {
		t.Error("expected PostCompact after codex install")
	}
	// Pre-existing non-claudio PreToolUse logger must survive alongside claudio.
	preArr := hooksSection["PreToolUse"].([]interface{})
	foundLogger := false
	for _, e := range preArr {
		cfg := e.(map[string]interface{})
		hooksList := cfg["hooks"].([]interface{})
		for _, h := range hooksList {
			if h.(map[string]interface{})["command"] == "/usr/bin/logger" {
				foundLogger = true
			}
		}
	}
	if !foundLogger {
		t.Error("pre-existing non-claudio hook was lost during merge")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/install/ -run "TestCodexInstallMergesIntoHooksJSON" -v`
Expected: PASS (reuses existing merge logic; no new production code needed). If it fails, the defect is in how `MergeHooksIntoSettings` handles the new event names — investigate before changing the test.

- [ ] **Step 3: Commit**

```bash
git add internal/install/hooks_test.go
git commit -m "test: cover codex install merge into hooks.json"
```

---

## Task 11: Coverage ratchet to >= 90%

**Files:**
- Test: `internal/hooks/parser_test.go`, `internal/install/*_test.go`

**Goal:** Push `internal/hooks` (baseline 80.2%) and `internal/install` (baseline 81.0%) to >= 90%. Use the coverage report to target uncovered branches rather than guessing.

- [ ] **Step 1: Generate per-function coverage for hooks**

Run:
```bash
go test ./internal/hooks/ -coverprofile=hooks.cov && go tool cover -func=hooks.cov | sort -k3 -n | head -30
```
Expected: a list of functions with their coverage; the lowest-covered are your targets.

- [ ] **Step 2: Add tests for the lowest-covered hooks branches**

Likely targets (add tests that hit these branches):

```go
func TestAnalyzeToolResponseInterrupted(t *testing.T) {
	resp := json.RawMessage(`{"interrupted":true}`)
	tool := "Bash"
	e := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolName: &tool, ToolResponse: &resp}
	ctx := e.GetContext()
	if !ctx.HasError {
		t.Error("expected HasError for interrupted response")
	}
	if ctx.SoundHint != "tool-interrupted" {
		t.Errorf("expected tool-interrupted, got %q", ctx.SoundHint)
	}
}

func TestAnalyzeToolResponseIsError(t *testing.T) {
	resp := json.RawMessage(`{"isError":true}`)
	tool := "apply_patch"
	e := &HookEvent{SessionID: "a", CWD: "/tmp", EventName: "PostToolUse", ToolName: &tool, ToolResponse: &resp}
	ctx := e.GetContext()
	if !ctx.HasError {
		t.Error("expected HasError for isError response")
	}
}

func TestParseEmptyDataErrors(t *testing.T) {
	if _, err := NewHookEventParser().Parse(nil); err == nil {
		t.Error("expected error for empty data")
	}
}

func TestParseInvalidJSONErrors(t *testing.T) {
	if _, err := NewHookEventParser().Parse([]byte(`{not json`)); err == nil {
		t.Error("expected error for malformed json")
	}
}
```

- [ ] **Step 3: Generate per-function coverage for install and add targeted tests**

Run:
```bash
go test ./internal/install/ -coverprofile=install.cov && go tool cover -func=install.cov | sort -k3 -n | head -30
```

Add tests for any uncovered `Agent` branches and the Windows `USERPROFILE` path in `findCodexUserScopePaths`. Example for the agent default/invalid paths and project scope:

```go
func TestAgentBestConfigPathProjectScope(t *testing.T) {
	p, err := AgentCodex.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(p, filepath.Join(".codex", "hooks.json")) {
		t.Errorf("got %q", p)
	}
	cp, err := AgentClaude.BestConfigPath("project")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cp, filepath.Join(".claude", "settings.json")) {
		t.Errorf("got %q", cp)
	}
}

func TestAgentBestConfigPathInvalidScope(t *testing.T) {
	if _, err := AgentCodex.BestConfigPath("bogus"); err == nil {
		t.Error("expected error for invalid scope")
	}
}
```

(Add `path/filepath` and `strings` imports to `agent_test.go` if missing.)

- [ ] **Step 4: Verify coverage targets met**

Run:
```bash
go test ./internal/hooks/ ./internal/install/ -cover
```
Expected: both packages report `coverage: >= 90.0%`. If not, repeat Steps 1-3 against the remaining uncovered functions.

- [ ] **Step 5: Clean up coverage artifacts and commit**

```bash
rm -f hooks.cov install.cov
git add internal/hooks/parser_test.go internal/install/agent_test.go
git commit -m "test: ratchet hooks and install coverage to >=90%"
```

---

## Task 12: Full verification

**Files:** none (verification only)

- [ ] **Step 1: Run the whole suite with the cgo linker available**

Run: `go test ./... -count=1`
Expected: all packages PASS, including `internal/cli`.

- [ ] **Step 2: Build the binary and smoke-test a Codex payload**

```bash
go build -o claudio .
echo '{"session_id":"test","cwd":"/test","hook_event_name":"PostToolUse","tool_name":"apply_patch","tool_response":{"output":"ok"}}' | ./claudio --silent
echo '{"session_id":"test","cwd":"/test","hook_event_name":"SubagentStart"}' | ./claudio --silent
```
Expected: exit code 0, no error output (silent mode skips audio).

- [ ] **Step 3: Smoke-test the Codex installer in dry-run**

```bash
./claudio install --agent codex --dry-run
./claudio install --agent codex --print
```
Expected: output shows a `.codex/hooks.json` path and the `/hooks` trust reminder.

- [ ] **Step 4: Clean up the build artifact**

```bash
rm -f claudio
```

- [ ] **Step 5: Final coverage confirmation**

Run: `go test ./internal/hooks/ ./internal/install/ ./internal/cli/ -cover`
Expected: hooks >= 90%, install >= 90%, cli no lower than its 67.0% baseline (ideally higher from the new command tests).

---

## Self-review notes

- Spec section "Parser tolerance" → Tasks 1, 2, 3.
- Spec "Agent abstraction" → Tasks 4, 5, 6, 7.
- Spec "CLI wiring" (install + trust reminder, uninstall) → Tasks 8, 9.
- Spec "Data flow / merge into hooks.json" → Task 10.
- Spec "Coverage targets" → Task 11; full verification → Task 12.
- Type consistency: `Agent`, `AgentClaude`, `AgentCodex`, `ParseAgent`, `Matcher()`, `Registry()`, `EnabledHooks()`, `HookNames()`, `BestConfigPath()`, `CodexHooks`, `FindCodexHooksPaths`, `FindBestCodexPath`, `GenerateClaudioHooksForAgent` are used consistently across tasks. `runInstallWorkflow` gains an `install.Agent` first parameter in Task 8 and is only called from `runInstallCommandE`.
- Known environment risk: `internal/cli` needs the cgo linker (`ld.exe`) fixed earlier this session. The `hooks` and `install` packages have no cgo dependency.
