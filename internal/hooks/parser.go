package hooks

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	captainhook "github.com/ctoth/captain-hook"
)

// EventCategory represents the type of hook event for sound mapping
type EventCategory int

const (
	Loading EventCategory = iota
	Success
	Error
	Interactive
	Completion
	System
	Silent
)

// categoryNames is the stable on-disk and CLI name of each category,
// indexed by its value.
var categoryNames = [...]string{
	Loading:     "loading",
	Success:     "success",
	Error:       "error",
	Interactive: "interactive",
	Completion:  "completion",
	System:      "system",
	Silent:      "silent",
}

func (c EventCategory) valid() bool {
	return c >= 0 && int(c) < len(categoryNames)
}

func (c EventCategory) String() string {
	if !c.valid() {
		return "unknown"
	}
	return categoryNames[c]
}

// Categories returns every EventCategory in value order.
func Categories() []EventCategory {
	out := make([]EventCategory, len(categoryNames))
	for i := range categoryNames {
		out[i] = EventCategory(i)
	}
	return out
}

// ParseEventCategory maps a category name ("success", "silent", ...) back to
// its EventCategory. Unknown names are an error listing the valid ones.
func ParseEventCategory(name string) (EventCategory, error) {
	if i := slices.Index(categoryNames[:], name); i >= 0 {
		return EventCategory(i), nil
	}
	return 0, fmt.Errorf("unknown category %q: must be one of %s", name, strings.Join(categoryNames[:], ", "))
}

// MarshalText stores a category as its stable name, so JSON encodes it as
// a string ("success") rather than the iota int.
func (c EventCategory) MarshalText() ([]byte, error) {
	if !c.valid() {
		return nil, fmt.Errorf("cannot marshal unknown category %d", int(c))
	}
	return []byte(categoryNames[c]), nil
}

// UnmarshalJSON accepts the stable name and, for rows recorded before
// categories were names, the legacy iota int.
func (c *EventCategory) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err == nil {
		parsed, err := ParseEventCategory(name)
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	}
	var n int
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("category must be a name or legacy int, got %s", data)
	}
	if !EventCategory(n).valid() {
		return fmt.Errorf("unknown legacy category %d", n)
	}
	*c = EventCategory(n)
	return nil
}

// HookEvent represents a parsed agent hook event.
type HookEvent struct {
	// Base fields (always present)
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
	EventName      string `json:"hook_event_name"`

	// Optional fields (event-specific)
	ToolName     *string          `json:"tool_name,omitempty"`
	ToolInput    *json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse *json.RawMessage `json:"tool_response,omitempty"`
	Prompt       *string          `json:"prompt,omitempty"`
	Message      *string          `json:"message,omitempty"`

	// payload is captain-hook's reading of the event. It is nil for a
	// HookEvent built by hand; parsed() then reads the fields above.
	payload *captainhook.Payload
}

// EventContext provides processed context for sound mapping
//
// The JSON keys are explicit because the tracking database stores this
// struct as JSON and queries it by key; renaming a field must not change
// the on-disk format.
type EventContext struct {
	Category     EventCategory `json:"Category"`
	ToolName     string        `json:"ToolName"`
	OriginalTool string        `json:"OriginalTool"` // Original tool before command extraction (for fallback)
	IsSuccess    bool          `json:"IsSuccess"`
	HasError     bool          `json:"HasError"`
	SoundHint    string        `json:"SoundHint"`
	FileType     string        `json:"FileType"`
	Operation    string        `json:"Operation"`

	// Command, Subcommand and Phase are the typed pieces the mapper builds
	// its command levels from. For tool events Command is the resolved tool
	// (the shell command for Bash, "mcp" for MCP tools, otherwise the tool
	// name), Subcommand is the parsed shell subcommand (may contain '-', as
	// in "port-forward"), and Phase is "start", "success", "error" or
	// PhaseUnknown. Lifecycle events leave them empty.
	Command    string `json:"Command,omitempty"`
	Subcommand string `json:"Subcommand,omitempty"`
	Phase      string `json:"Phase,omitempty"`
}

// CommandInfo represents parsed command information from Bash tool input
type CommandInfo struct {
	Command       string // First non-flag word (e.g., "git", "npm")
	Subcommand    string // Second non-flag word (e.g., "commit", "install")
	HasSubcommand bool   // True if subcommand was found
}

// ParseHookEvent parses hook JSON data into a HookEvent
func ParseHookEvent(data []byte) (*HookEvent, error) {
	return ParseHookEventWithDefault(data, "")
}

// ParseHookEventWithDefault parses hook JSON and uses defaultEvent when the
// payload format does not include hook_event_name.
func ParseHookEventWithDefault(data []byte, defaultEvent string) (*HookEvent, error) {
	return ParseHookEventFrom("", data, defaultEvent)
}

// ParseHookEventFrom parses hook JSON sent by agent, the --hook-agent value
// of the hook command. With agent "" the payload is read by what it shows.
// defaultEvent names the event for payloads that do not.
//
// captain-hook reads the payload: each agent's field names, tool names and
// way of reporting a failed tool are its business, not claudio's.
func ParseHookEventFrom(agent string, data []byte, defaultEvent string) (*HookEvent, error) {
	if len(data) == 0 {
		return nil, errors.New("empty JSON data")
	}
	if defaultEvent == "" {
		defaultEvent = legacyEventName(data)
	}
	agent = strings.ToLower(strings.TrimSpace(agent))
	payload, err := captainhook.Parse(captainhook.Agent(agent), defaultEvent, data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse hook JSON: %w", err)
	}

	event := &HookEvent{
		SessionID:      payload.SessionID,
		TranscriptPath: payload.TranscriptPath,
		CWD:            payload.CWD,
		EventName:      NormalizeEventName(payload.Event),
		payload:        payload,
	}
	if payload.Prompt != "" {
		event.Prompt = &payload.Prompt
	}
	if payload.Message != "" {
		event.Message = &payload.Message
	}
	if tool := payload.Tool; tool != nil {
		event.ToolName = &tool.RawName
		if tool.Input != nil {
			event.ToolInput = &tool.Input
		}
		if tool.Response != nil {
			event.ToolResponse = &tool.Response
		}
	} else {
		// captain-hook hands back a response only with a tool; claudio's
		// legacy rules also read one on an event that names no tool.
		var toolless struct {
			ToolResponse *json.RawMessage `json:"tool_response"`
		}
		_ = json.Unmarshal(data, &toolless) // data parsed above; a miss leaves nil
		event.ToolResponse = toolless.ToolResponse
	}

	if event.SessionID == "" {
		return nil, errors.New("missing required field: session_id")
	}
	if event.CWD == "" {
		return nil, errors.New("missing required field: cwd")
	}
	return event, nil
}

// legacyEventName returns the payload's "hookEventName", which claudio has
// read since it first supported Copilot CLI. No current agent is known to
// send it, so it is not captain-hook's to know.
func legacyEventName(data []byte) string {
	var legacy struct {
		EventName string `json:"hookEventName"`
	}
	_ = json.Unmarshal(data, &legacy) // a payload without it has no legacy name
	return legacy.EventName
}

// parsed returns captain-hook's reading of the event. A HookEvent built by
// hand is serialized first: its fields are already a Claude Code payload.
func (e *HookEvent) parsed() *captainhook.Payload {
	if e.payload != nil {
		return e.payload
	}
	if data, err := json.Marshal(e); err == nil {
		if payload, err := captainhook.Parse("", e.EventName, data); err == nil {
			return payload
		}
	}
	return &captainhook.Payload{Event: e.EventName}
}

// Agent names the agent that sent the event ("claude", "codex"), or "" when
// neither the hook command nor the payload says.
func (e *HookEvent) Agent() string {
	return string(e.parsed().Agent)
}

// NormalizeEventName converts agent-specific hook keys to Claudio's canonical
// event names for sound mapping.
func NormalizeEventName(name string) string {
	switch strings.TrimSpace(name) {
	case "subagentStart":
		return "SubagentStart"
	case "notification":
		return "Notification"
	case "userPromptTransformed":
		return "UserPromptTransformed"
	default:
		return name
	}
}

// lifecycleEvent is the fixed context a tool-less event maps to.
type lifecycleEvent struct {
	category  EventCategory
	hint      string
	operation string
}

// lifecycleEvents maps every tool-less event with a fixed context. Error
// category events also set HasError. Silent events carry no hint.
var lifecycleEvents = map[string]lifecycleEvent{
	"UserPromptSubmit":    {Interactive, "message-sent", "prompt"},
	"UserPromptExpansion": {Interactive, "prompt-expansion", "prompt-expansion"},
	"PostToolBatch":       {Success, "tool-batch", "tool-batch"},
	"Stop":                {Completion, "agent-complete", "stop"},
	"SubagentStop":        {Completion, "subagent-complete", "subagent-stop"},
	"SubagentStart":       {Loading, "subagent-start", "subagent-start"},
	"PostCompact":         {System, "post-compact", "post-compact"},
	"PreCompact":          {System, "compacting", "compact"},
	"PreCompress":         {System, "compacting", "compact"},
	"SessionStart":        {System, "session-start", "session-start"},
	"PermissionRequest":   {Interactive, "permission-request", "permission-request"},
	"PermissionDenied":    {Error, "permission-denied", "permission-denied"},
	"SessionEnd":          {Interactive, "session-end", "session-end"},
	"BeforeAgent":         {Interactive, "before-agent", "before-agent"},
	"AfterAgent":          {Completion, "agent-complete", "after-agent"},
	"StopFailure":         {Error, "stop-failure", "stop-failure"},
	"ErrorOccurred":       {Error, "error-occurred", "error-occurred"},
	"Setup":               {System, "setup", "setup"},
	"TaskCreated":         {Loading, "task-created", "task-created"},
	"TaskCompleted":       {Completion, "task-completed", "task-completed"},
	"TeammateIdle":        {Interactive, "teammate-idle", "teammate-idle"},
	"InstructionsLoaded":  {System, "instructions-loaded", "instructions-loaded"},
	"ConfigChange":        {System, "config-change", "config-change"},
	"CwdChanged":          {System, "cwd-changed", "cwd-changed"},
	"FileChanged":         {System, "file-changed", "file-changed"},
	"WorktreeCreate":      {System, "worktree-create", "worktree-create"},
	"WorktreeRemove":      {System, "worktree-remove", "worktree-remove"},
	"Elicitation":         {Interactive, "elicitation", "elicitation"},
	"ElicitationResult":   {Interactive, "elicitation-result", "elicitation-result"},
	"TodoCreated":         {Loading, "todo-created", "todo-created"},
	"TodoCompleted":       {Completion, "todo-completed", "todo-completed"},
	"DirectoryAdded":      {System, "directory-added", "directory-added"},
	"PreModelSwitch":      {System, "model-switching", "pre-model-switch"},
	"PostModelSwitch":     {System, "model-switched", "post-model-switch"},
	"Interrupt":           {Interactive, "interrupt", "interrupt"},
	"SessionDelete":       {Interactive, "session-delete", "session-delete"},
	"MessageDisplay":      {Silent, "", "message-display"},
	// Copilot fires UserPromptTransformed for every prompt, right after
	// UserPromptSubmit, so a sound would double the prompt sound.
	"UserPromptTransformed": {Silent, "", "prompt-transformed"},
	"BeforeModel":           {Silent, "", "beforemodel"},
	"AfterModel":            {Silent, "", "aftermodel"},
	"BeforeToolSelection":   {Silent, "", "beforetoolselection"},
}

// unknownEvent is the context for events claudio does not recognize.
var unknownEvent = lifecycleEvent{Interactive, "default", "unknown"}

// GetContext extracts actionable context from the hook event for sound mapping
func (e *HookEvent) GetContext() *EventContext {
	context := &EventContext{
		ToolName: e.toolName(),
	}

	switch e.EventName {
	case "Notification":
		context.Category = Interactive
		context.SoundHint = e.detectNotificationType()
		context.Operation = "notification"

	case "PreToolUse", "BeforeTool":
		e.populatePreToolContext(context)

	case "PostToolUse", "AfterTool":
		e.populatePostToolContext(context, false)

	case "PostToolUseFailure":
		e.populatePostToolContext(context, true)

	default:
		spec, ok := lifecycleEvents[e.EventName]
		if !ok {
			slog.Warn("unknown hook event type", "event_name", e.EventName)
			spec = unknownEvent
		}
		context.Category = spec.category
		context.SoundHint = spec.hint
		context.Operation = spec.operation
		context.HasError = spec.category == Error
	}

	// Extract file type context for file operations
	if context.ToolName != "" && context.OriginalTool == "" {
		context.FileType = e.extractFileType()
	}

	return context
}

func (e *HookEvent) populatePreToolContext(context *EventContext) {
	context.Category = Loading
	context.Operation = "tool-start"
	e.populateToolIdentity(context, "start", "")
}

// toolName returns claudio's name for the event's tool: captain-hook's name
// for it, then claudio's own vocabulary for what captain-hook leaves alone
// ("LS", "MultiEdit", and "mcp" for every MCP tool).
func (e *HookEvent) toolName() string {
	tool := e.parsed().Tool
	if tool == nil {
		return normalizeToolName(getStringPtr(e.ToolName))
	}
	if tool.Name == "MCP" {
		return "mcp"
	}
	return normalizeToolName(tool.Name)
}

// PhaseUnknown is the Phase of a tool call that ended without the payload
// saying how. The mapper gives it the neutral tool-complete sound.
const PhaseUnknown = "unknown"

// populatePostToolContext scores a finished tool call from captain-hook's
// Outcome. forceError marks the agent's failure event.
//
// An unknown outcome plays the neutral tool-complete sound only for a shell
// command that did send a result: Codex sends a command's output and never
// its exit code. Any other unknown outcome counts as a success, as it
// always has: no agent is known to report a failed non-shell tool this way.
func (e *HookEvent) populatePostToolContext(context *EventContext, forceError bool) {
	context.Operation = "tool-complete"

	outcome, hasResponse := captainhook.OutcomeSuccess, false
	if tool := e.parsed().Tool; tool != nil {
		outcome, hasResponse = tool.Outcome, tool.Response != nil
	}
	if forceError && outcome != captainhook.OutcomeInterrupted {
		outcome = captainhook.OutcomeFailure
	}
	if outcome != captainhook.OutcomeFailure && outcome != captainhook.OutcomeInterrupted {
		if legacy := e.legacyOutcome(context.ToolName); legacy != captainhook.OutcomeNone {
			outcome = legacy
		}
	}

	switch {
	case outcome == captainhook.OutcomeInterrupted:
		context.Category, context.HasError = Error, true
		e.populateToolIdentity(context, "error", "tool-interrupted")
	case outcome == captainhook.OutcomeFailure:
		context.Category, context.HasError = Error, true
		e.populateToolIdentity(context, "error", "")
	case outcome == captainhook.OutcomeUnknown && hasResponse && context.ToolName == "Bash":
		context.Category = Success
		e.populateToolIdentity(context, PhaseUnknown, "tool-complete")
	default:
		context.Category, context.IsSuccess = Success, true
		e.populateToolIdentity(context, "success", "")
	}
}

// legacyOutcome applies the failure signs claudio read from a tool response
// before captain-hook read payloads, for the cases captain-hook does not
// call a failure. It returns OutcomeNone when none of them applies.
//
// Current Claude Code sends a failed tool as PostToolUseFailure and leaves
// "stderr" empty on a PostToolUse (checked against 2.1.290), so these do
// not fire there. They are kept because older agent versions that relied
// on them cannot be ruled out:
//   - a response that is neither an object nor text
//   - a response with non-empty "stderr"
//   - "success": false from an edit tool
//   - "interrupted", "isError" or "error" on an event that names no tool
//     (captain-hook reads a response only when there is a tool)
func (e *HookEvent) legacyOutcome(tool string) captainhook.Outcome {
	if e.ToolResponse == nil {
		return captainhook.OutcomeNone
	}
	var response map[string]any
	if json.Unmarshal(*e.ToolResponse, &response) != nil {
		var text string
		if json.Unmarshal(*e.ToolResponse, &text) == nil {
			return captainhook.OutcomeNone // text: captain-hook has read it
		}
		slog.Warn("tool response is neither an object nor text; treating it as an error")
		return captainhook.OutcomeFailure
	}
	if tool == "" {
		if interrupted, _ := response["interrupted"].(bool); interrupted {
			return captainhook.OutcomeInterrupted
		}
		if isError, _ := response["isError"].(bool); isError {
			return captainhook.OutcomeFailure
		}
		if value, ok := response["error"]; ok && value != nil && value != "" {
			return captainhook.OutcomeFailure
		}
	}
	if stderr, _ := response["stderr"].(string); stderr != "" {
		return captainhook.OutcomeFailure
	}
	if success, ok := response["success"].(bool); ok && !success {
		switch tool {
		case "Edit", "Write", "MultiEdit":
			return captainhook.OutcomeFailure
		}
	}
	return captainhook.OutcomeNone
}

// noToolHints is the hint for a tool event that names no tool, by phase.
var noToolHints = map[string]string{
	"start":   "tool-loading",
	"success": "tool-success",
	"error":   "tool-error",
}

// populateToolIdentity resolves the tool a tool event is about and sets
// Command, Subcommand, Phase and SoundHint. Bash becomes its shell command
// (OriginalTool "Bash") and MCP tools become "mcp" (OriginalTool the full
// MCP name). The hint is errorType when set, otherwise
// command[-subcommand]-phase.
func (e *HookEvent) populateToolIdentity(context *EventContext, phase, errorType string) {
	switch {
	case context.ToolName == "Bash":
		if info := e.extractCommandInfo(); info.Command != "" {
			context.OriginalTool = "Bash"
			context.ToolName = info.Command
			context.Subcommand = info.Subcommand
		}
	case isMCPToolName(context.ToolName):
		context.OriginalTool = getStringPtr(e.ToolName)
		context.ToolName = "mcp"
	}
	context.Command = context.ToolName
	context.Phase = phase

	switch {
	case errorType != "":
		context.SoundHint = errorType
	case context.ToolName == "":
		context.SoundHint = noToolHints[phase]
	case context.Subcommand != "":
		context.SoundHint = strings.ToLower(context.Command) + "-" + strings.ToLower(context.Subcommand) + "-" + phase
	default:
		context.SoundHint = strings.ToLower(context.Command) + "-" + phase
	}
}

func normalizeToolName(toolName string) string {
	if toolName == "" {
		return ""
	}
	if isMCPToolName(toolName) {
		return "mcp"
	}

	key := strings.ToLower(strings.TrimSpace(toolName))
	key = strings.ReplaceAll(key, "-", "_")
	switch key {
	case "bash", "shell", "powershell", "run_shell_command", "shell_command":
		return "Bash"
	case "write", "writefile", "write_file", "create":
		return "Write"
	case "edit", "edit_file", "replace":
		return "Edit"
	case "multiedit", "multi_edit":
		return "MultiEdit"
	case "read", "readfile", "read_file", "read_many_files", "view":
		return "Read"
	case "ls", "list", "list_directory":
		return "LS"
	case "grep", "grep_search":
		return "Grep"
	case "glob":
		return "Glob"
	case "webfetch", "web_fetch":
		return "WebFetch"
	case "websearch", "web_search", "google_web_search":
		return "WebSearch"
	case "todowrite", "todo_write", "write_todos":
		return "TodoWrite"
	case "read_mcp_resource", "list_mcp_resources":
		return "mcp"
	default:
		return toolName
	}
}

func isMCPToolName(toolName string) bool {
	return toolName == "mcp" || strings.HasPrefix(toolName, "mcp__") || strings.HasPrefix(toolName, "mcp_")
}

// extractFileType attempts to extract file type from tool input
func (e *HookEvent) extractFileType() string {
	if e.ToolInput == nil {
		return ""
	}

	var input map[string]any
	err := json.Unmarshal(*e.ToolInput, &input)
	if err != nil {
		slog.Debug("failed to parse tool input for file type extraction", "error", err)
		return ""
	}

	// Look for file paths in common input fields
	for _, field := range []string{"file_path", "path", "filename"} {
		if path, ok := input[field].(string); ok && path != "" {
			fileType := extractFileExtension(path)
			if fileType != "" {
				return fileType
			}
		}
	}

	return ""
}

// extractCommandInfo parses command information from Bash tool input
func (e *HookEvent) extractCommandInfo() CommandInfo {
	if e.ToolInput == nil {
		return CommandInfo{}
	}

	var input map[string]any
	err := json.Unmarshal(*e.ToolInput, &input)
	if err != nil {
		slog.Debug("failed to parse tool input for command extraction", "error", err)
		return CommandInfo{}
	}

	command, ok := input["command"].(string)
	if !ok || command == "" {
		return CommandInfo{}
	}

	// Split command string into words. Command Code passes arguments
	// separately in "args".
	words := strings.Fields(strings.TrimSpace(command))
	if args, ok := input["args"].([]any); ok {
		for _, arg := range args {
			if s, ok := arg.(string); ok {
				words = append(words, s)
			}
		}
	}
	if len(words) == 0 {
		return CommandInfo{}
	}

	var cmd, subCmd string

	// Find first and second non-flag words
	for _, word := range words {
		if !strings.HasPrefix(word, "-") {
			if cmd == "" {
				cmd = word
			} else if subCmd == "" && isValidSubcommand(cmd, word) {
				subCmd = word
				break
			}
		}
	}

	result := CommandInfo{
		Command:       cmd,
		Subcommand:    subCmd,
		HasSubcommand: subCmd != "",
	}

	return result
}

// isValidSubcommand determines if a word is likely a subcommand rather than an argument
func isValidSubcommand(command, word string) bool {
	// Paths, file names and URLs are not subcommands
	if strings.Contains(word, "/") || strings.Contains(word, ".") {
		return false
	}

	// Known command patterns that have subcommands
	knownSubcommands := map[string][]string{
		"git":     {"add", "commit", "push", "pull", "clone", "checkout", "branch", "merge", "rebase", "status", "log", "diff", "fetch", "remote", "tag", "stash", "reset", "revert"},
		"npm":     {"install", "uninstall", "update", "start", "stop", "restart", "test", "run", "build", "publish", "pack", "init", "config", "cache", "audit", "fund", "outdated"},
		"docker":  {"build", "run", "pull", "push", "start", "stop", "restart", "kill", "rm", "rmi", "ps", "images", "logs", "exec", "compose", "volume", "network"},
		"cargo":   {"build", "run", "test", "doc", "new", "init", "add", "install", "update", "search", "publish", "bench", "clean", "check", "fmt", "clippy"},
		"go":      {"build", "run", "test", "install", "get", "mod", "fmt", "vet", "generate", "clean", "env", "bug", "version", "doc"},
		"pip":     {"install", "uninstall", "list", "show", "freeze", "search", "download", "wheel", "hash", "completion", "debug", "help"},
		"yarn":    {"add", "install", "remove", "upgrade", "start", "build", "test", "run", "init", "cache", "config", "info", "why"},
		"kubectl": {"get", "describe", "create", "apply", "delete", "patch", "replace", "expose", "scale", "autoscale", "rollout", "logs", "exec", "port-forward", "proxy", "cp", "auth", "config"},
	}

	if subcommands, exists := knownSubcommands[command]; exists {
		return slices.Contains(subcommands, word)
	}

	// For unknown commands, be conservative - only allow alphanumeric subcommands
	// This catches cases like "systemctl start" but rejects "ls /path/to/file"
	for _, r := range word {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}

	return true
}

// Helper functions

func getStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func extractFileExtension(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	// Filter out common non-file-type extensions
	switch ext {
	case "tmp", "bak", "log", "old", "orig":
		return ""
	default:
		return ext
	}
}

// Notification type detection keywords
var (
	permissionKeywords = []string{"permission", "needs permission", "needs your permission"}
	idleKeywords       = []string{"idle", "been idle", "idle for"}
)

// detectNotificationType analyzes notification message content to generate
// specific sound hints. The message is user content and is never logged.
func (e *HookEvent) detectNotificationType() string {
	if e.Message == nil {
		return "notification"
	}
	message := strings.ToLower(*e.Message)
	for _, keyword := range permissionKeywords {
		if strings.Contains(message, keyword) {
			return "notification-permission"
		}
	}
	for _, keyword := range idleKeywords {
		if strings.Contains(message, keyword) {
			return "notification-idle"
		}
	}
	return "notification"
}
