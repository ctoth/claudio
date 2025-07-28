package hooks

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// EventCategory represents the type of hook event for sound mapping
type EventCategory int

const (
	Loading EventCategory = iota
	Success
	Error
	Interactive
)

func (c EventCategory) String() string {
	switch c {
	case Loading:
		return "loading"
	case Success:
		return "success"
	case Error:
		return "error"
	case Interactive:
		return "interactive"
	default:
		return "unknown"
	}
}

// HookEvent represents a parsed Claude Code hook event
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
}

// EventContext provides processed context for sound mapping
type EventContext struct {
	Category    EventCategory
	ToolName    string
	IsSuccess   bool
	HasError    bool
	SoundHint   string
	FileType    string
	Operation   string
}

// CommandInfo represents parsed command information from Bash tool input
type CommandInfo struct {
	Command       string // First non-flag word (e.g., "git", "npm")
	Subcommand    string // Second non-flag word (e.g., "commit", "install")
	HasSubcommand bool   // True if subcommand was found
}

// HookEventParser parses Claude Code hook JSON into structured events
type HookEventParser struct{}

// NewHookEventParser creates a new hook event parser
func NewHookEventParser() *HookEventParser {
	slog.Debug("creating new hook event parser")
	return &HookEventParser{}
}

// Parse parses hook JSON data into a HookEvent
func (p *HookEventParser) Parse(data []byte) (*HookEvent, error) {
	if len(data) == 0 {
		err := fmt.Errorf("empty JSON data")
		slog.Error("parse failed: empty data", "error", err)
		return nil, err
	}

	slog.Debug("parsing hook JSON", "size_bytes", len(data))

	var event HookEvent
	err := json.Unmarshal(data, &event)
	if err != nil {
		slog.Error("failed to unmarshal hook JSON", "error", err, "data_preview", string(data[:min(100, len(data))]))
		return nil, fmt.Errorf("failed to parse hook JSON: %w", err)
	}

	// Validate required fields
	if event.SessionID == "" {
		err := fmt.Errorf("missing required field: session_id")
		slog.Error("validation failed", "error", err)
		return nil, err
	}

	if event.EventName == "" {
		err := fmt.Errorf("missing required field: hook_event_name")
		slog.Error("validation failed", "error", err)
		return nil, err
	}

	if event.CWD == "" {
		err := fmt.Errorf("missing required field: cwd")
		slog.Error("validation failed", "error", err)
		return nil, err
	}

	if event.TranscriptPath == "" {
		err := fmt.Errorf("missing required field: transcript_path")
		slog.Error("validation failed", "error", err)
		return nil, err
	}

	slog.Info("hook event parsed successfully",
		"event_name", event.EventName,
		"session_id", event.SessionID,
		"tool_name", getStringPtr(event.ToolName),
		"has_tool_response", event.ToolResponse != nil)

	return &event, nil
}

// GetContext extracts actionable context from the hook event for sound mapping
func (e *HookEvent) GetContext() *EventContext {
	context := &EventContext{
		ToolName: getStringPtr(e.ToolName),
	}

	slog.Debug("extracting event context",
		"event_name", e.EventName,
		"tool_name", context.ToolName)

	switch e.EventName {
	case "UserPromptSubmit":
		context.Category = Interactive
		context.SoundHint = "message-sent"
		context.Operation = "prompt"

	case "Notification":
		context.Category = Interactive
		context.SoundHint = "notification"
		context.Operation = "notification"

	case "PreToolUse":
		context.Category = Loading
		context.Operation = "tool-start"
		
		if context.ToolName != "" {
			context.SoundHint = strings.ToLower(context.ToolName) + "-thinking"
		} else {
			context.SoundHint = "tool-loading"
		}

	case "PostToolUse":
		// Analyze tool response to determine success/error and specific error type
		success, hasError, errorType := e.analyzeToolResponse()
		context.IsSuccess = success
		context.HasError = hasError

		if hasError {
			context.Category = Error
			
			// Use specific error type if available
			if errorType != "" {
				context.SoundHint = errorType
			} else if context.ToolName != "" {
				context.SoundHint = strings.ToLower(context.ToolName) + "-error"
			} else {
				context.SoundHint = "tool-error"
			}
		} else {
			context.Category = Success
			if context.ToolName != "" {
				context.SoundHint = strings.ToLower(context.ToolName) + "-success"
			} else {
				context.SoundHint = "tool-success"
			}
		}

		context.Operation = "tool-complete"

	case "Stop", "SubagentStop":
		context.Category = Success
		context.SoundHint = "completion"
		context.Operation = "stop"

	case "PreCompact":
		context.Category = Loading
		context.SoundHint = "organizing"
		context.Operation = "compact"

	default:
		slog.Warn("unknown hook event type", "event_name", e.EventName)
		context.Category = Interactive
		context.SoundHint = "default"
		context.Operation = "unknown"
	}

	// Extract file type context for file operations
	if context.ToolName != "" {
		context.FileType = e.extractFileType()
	}

	slog.Info("event context extracted",
		"event_name", e.EventName,
		"category", context.Category.String(),
		"sound_hint", context.SoundHint,
		"tool_name", context.ToolName,
		"is_success", context.IsSuccess,
		"has_error", context.HasError,
		"file_type", context.FileType,
		"operation", context.Operation)

	return context
}

// analyzeToolResponse examines tool response to determine success/error status and error type
func (e *HookEvent) analyzeToolResponse() (success bool, hasError bool, errorType string) {
	if e.ToolResponse == nil {
		slog.Debug("no tool response to analyze")
		return true, false, "" // No response usually means success
	}

	var response map[string]interface{}
	err := json.Unmarshal(*e.ToolResponse, &response)
	if err != nil {
		slog.Error("failed to parse tool response", "error", err)
		return false, true, ""
	}

	slog.Debug("analyzing tool response", "response_keys", getMapKeys(response))

	// Check for interruption first (more specific than stderr)
	if interrupted, ok := response["interrupted"].(bool); ok && interrupted {
		slog.Debug("tool was interrupted")
		return false, true, "tool-interrupted"
	}

	// Check for common error indicators
	if stderr, ok := response["stderr"].(string); ok && stderr != "" {
		slog.Debug("tool response has stderr", "stderr_length", len(stderr))
		return false, true, ""
	}

	// Check for tool-specific error patterns
	if e.ToolName != nil {
		switch *e.ToolName {
		case "Bash":
			// Bash is success if no stderr and not interrupted
			return true, false, ""

		case "Read", "LS", "Glob":
			// File tools are success if they have content
			if content, ok := response["content"]; ok && content != nil {
				return true, false, ""
			}
			return false, true, ""

		case "Edit", "Write", "MultiEdit":
			// Edit tools should indicate success/failure explicitly
			if success, ok := response["success"].(bool); ok {
				return success, !success, ""
			}
			// If no explicit success field, assume success
			return true, false, ""

		case "Grep":
			// Grep is success if it returns results
			if numLines, ok := response["numLines"].(float64); ok {
				return numLines >= 0, false, "" // Even 0 results is success
			}
			return true, false, ""

		default:
			slog.Debug("unknown tool type for response analysis", "tool_name", *e.ToolName)
		}
	}

	// Default: assume success if no clear error indicators
	return true, false, ""
}

// extractFileType attempts to extract file type from tool input
func (e *HookEvent) extractFileType() string {
	if e.ToolInput == nil {
		return ""
	}

	var input map[string]interface{}
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
				slog.Debug("extracted file type", "field", field, "path", path, "file_type", fileType)
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

	var input map[string]interface{}
	err := json.Unmarshal(*e.ToolInput, &input)
	if err != nil {
		slog.Debug("failed to parse tool input for command extraction", "error", err)
		return CommandInfo{}
	}

	command, ok := input["command"].(string)
	if !ok || command == "" {
		return CommandInfo{}
	}

	// Split command string into words
	words := strings.Fields(strings.TrimSpace(command))
	if len(words) == 0 {
		return CommandInfo{}
	}

	var cmd, subCmd string

	// Find first and second non-flag words
	for _, word := range words {
		if !strings.HasPrefix(word, "-") {
			if cmd == "" {
				cmd = word
			} else if subCmd == "" {
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

	slog.Debug("extracted command info",
		"original", command,
		"command", result.Command,
		"subcommand", result.Subcommand,
		"has_subcommand", result.HasSubcommand)

	return result
}

// Helper functions

func getStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func extractFileExtension(path string) string {
	lastDot := strings.LastIndex(path, ".")
	if lastDot == -1 || lastDot == len(path)-1 {
		return ""
	}
	
	ext := strings.ToLower(path[lastDot+1:])
	
	// Filter out common non-file-type extensions
	switch ext {
	case "tmp", "bak", "log", "old", "orig":
		return ""
	default:
		return ext
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}