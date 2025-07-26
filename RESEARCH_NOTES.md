# Claudio Research Notes

## Project Goal
Create a Hook-based plugin for Claude Code called Claudio which will allow playing sound files for various events.

## Claude Code Hooks Research

### Available Hook Events
1. `PreToolUse` - Before any tool runs
2. `PostToolUse` - After tool completes 
3. `UserPromptSubmit` - When user sends message
4. `Notification` - System notifications
5. `Stop` - When main agent finishes responding
6. `SubagentStop` - When sub-agent completes
7. `PreCompact` - Before context compaction

### JSON Fields Available via stdin
- `session_id` - Unique session identifier
- `transcript_path` - Path to conversation JSON
- `cwd` - Current working directory
- `hook_event_name` - Which event triggered
- `tool_name` - Specific tool being used
- `tool_input` - Tool parameters/arguments
- `tool_response` - Tool execution results (PostToolUse only)
- `prompt` - User's submitted text (UserPromptSubmit only)
- `stop_hook_active` - Hook continuation state

### Environment Variables
- `CLAUDE_PROJECT_DIR` - Project root directory

### Tools We Can Match Against
- Task, Bash, Glob, Grep, Read, Edit, MultiEdit, Write, WebFetch, WebSearch

## Hook Discovery & Configuration

### Configuration Files (Priority Order)
1. Enterprise managed policy settings (highest)
2. `~/.claude/settings.json` (user-level)
3. `.claude/settings.json` (project-level) 
4. `.claude/settings.local.json` (local project, lowest)

### Discovery Process
- Claude Code loads and merges all settings files on startup
- Creates snapshot of hook configurations for session
- Hooks executed based on settings, not path discovery
- Changes require Claude Code restart or `/hooks` menu review

### Complete Hook Events Available
1. **PreToolUse** - Before tool execution → loading/thinking sounds
2. **PostToolUse** - After tool completion → success/error sounds  
3. **UserPromptSubmit** - When user sends message → message sent sounds
4. **Notification** - Permission requests/idle timeout → alert sounds
5. **Stop** - Main agent finishes → completion sounds
6. **SubagentStop** - Task tool completes → sub-task done sounds
7. **PreCompact** - Before context compaction → organizing sounds

### Installation Strategy
1. User installs `claudio` binary to PATH
2. User configures hooks in `~/.claude/settings.json`
3. Claude Code automatically calls `claudio` on events
4. No additional discovery mechanism needed

### Complete Hook Configuration Example
```json
{
  "hooks": {
    "PreToolUse": [{"matcher": ".*", "hooks": [{"type": "command", "command": "claudio"}]}],
    "PostToolUse": [{"matcher": ".*", "hooks": [{"type": "command", "command": "claudio"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "claudio"}]}],
    "Notification": [{"hooks": [{"type": "command", "command": "claudio"}]}],
    "Stop": [{"hooks": [{"type": "command", "command": "claudio"}]}],
    "SubagentStop": [{"hooks": [{"type": "command", "command": "claudio"}]}],
    "PreCompact": [{"hooks": [{"type": "command", "command": "claudio"}]}]
  }
}
```

## Audio Technology Decision

### Option 1: Go + malgo (miniaudio wrapper)
- Use `github.com/gen2brain/malgo` - Go wrapper for miniaudio
- Much simpler than Rust
- Fast cross-compilation (`GOOS=windows go build`)
- Single binary per platform
- Examples available, straightforward API

### Option 2: Rust + miniaudio
- Use `miniaudio-rs` crate (Rust wrapper for miniaudio)
- More complex build system
- Excellent performance
- Steeper learning curve

### Recommendation: Go + malgo
- Simpler development and maintenance
- Go's cross-compilation is trivial
- Still uses battle-tested miniaudio underneath

### Distribution: GitHub Actions + Binary Releases
- Cross-compile for Windows, macOS, Linux
- Single binary per platform
- Can be in PATH or configured path

## CLI Interface Design

### Primary Usage (reads JSON from stdin)
```bash
claudio  # Reads hook JSON from stdin, plays contextual sound
```

### Configuration
```bash
claudio --set-soundpack retro                           # Set local soundpack
claudio --set-soundpack https://cdn.example.com/cyber/  # Set web soundpack
claudio --list-soundpacks                               # Show available soundpacks
claudio --clear-cache                                   # Clear web soundpack cache
```

### Direct Usage (for testing)
```bash
claudio --test success    # Play success sound from current soundpack
claudio --test error      # Play error sound from current soundpack
```

### Configuration File
- **Location**: `~/.claudio/config.json`
- **Format**: `{"soundpack": "retro"}` or `{"soundpack": "https://..."}`
- **Default**: Built-in soundpack if no config exists

## Sound Mapping Strategy: Specific to General

### Matching Priority (Most Specific First)
1. **Exact Tool + Args Match**: `PostToolUse` + `tool_name: "Write"` + `file_path: "*.rs"` → rust-save.wav
2. **Tool + Success/Failure**: `PostToolUse` + `tool_name: "Bash"` + success → terminal-success.wav  
3. **Tool Type Only**: `PostToolUse` + `tool_name: "Edit"` → edit.wav
4. **Event Type**: `PostToolUse` → tool-complete.wav
5. **Fallback**: Default success/error sounds

### Specific Examples
- `Write` + `*.rs` + success → rust-save.wav
- `Write` + `*.py` + success → python-save.wav  
- `Write` + success → file-save.wav (fallback)
- `Bash` + `git commit` → commit.wav
- `Bash` + `npm test` + failure → test-fail.wav
- `Grep` + found results → search-found.wav
- `UserPromptSubmit` → message-sent.wav

### Configuration Structure
```json
{
  "mappings": [
    {"event": "PostToolUse", "tool": "Write", "args_match": "*.rs", "result": "success", "sound": "rust-save.wav"},
    {"event": "PostToolUse", "tool": "Write", "result": "success", "sound": "file-save.wav"},
    {"event": "PostToolUse", "tool": "Bash", "sound": "terminal.wav"},
    {"event": "PostToolUse", "result": "success", "sound": "success.wav"},
    {"event": "*", "sound": "default.wav"}
  ]
}

## Soundpack Architecture Ideas
- Default soundpack built into binary
- External soundpack directories
- JSON config for soundpack mappings
- Fallback to default sounds if custom missing

## Audio Decoder Architecture

### Package Design
```go
package decoder

type AudioData struct {
    Samples    []byte
    Channels   uint32  
    SampleRate uint32
    Format     FormatType  // malgo.FormatS16, etc.
}

type Decoder interface {
    Decode(io.Reader) (*AudioData, error)
    CanDecode(filename string) bool
}

// Implementations
type WAVDecoder struct{}  // github.com/youpy/go-wav
type MP3Decoder struct{}  // github.com/hajimehoshi/go-mp3
```

### Usage
```go
decoders := []Decoder{&MP3Decoder{}, &WAVDecoder{}}

func DecodeAudioFile(filename string) (*AudioData, error) {
    for _, dec := range decoders {
        if dec.CanDecode(filename) {
            file, _ := os.Open(filename)
            return dec.Decode(file)
        }
    }
    return nil, ErrUnsupportedFormat
}
```

### Benefits
- Easy to add new formats later (OGG, FLAC, etc.)
- Clean separation from malgo audio output
- Testable with mocked decoders
- Consistent AudioData format for malgo

## Next Steps
1. Design overall architecture and user experience
2. Start implementation with TDD approach