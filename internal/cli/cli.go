package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"strconv"

	"claudio/internal/audio"
	"claudio/internal/config"
	"claudio/internal/hooks"
	"claudio/internal/sounds"
)

// CLI represents the command-line interface
type CLI struct {
	configManager *config.ConfigManager
	soundMapper   *sounds.SoundMapper
	soundLoader   *SoundLoader
	audioPlayer   *audio.AudioPlayer
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	slog.Debug("creating new CLI instance")
	return &CLI{
		configManager: config.NewConfigManager(),
		soundMapper:   sounds.NewSoundMapper(),
		soundLoader:   nil, // Will be initialized when soundpack paths are known
		audioPlayer:   audio.NewAudioPlayer(),
	}
}

// Run executes the CLI with the given arguments and I/O streams
func (c *CLI) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	slog.Debug("CLI run started", "args", args)
	
	// Ensure audio player is cleaned up on exit
	defer func() {
		if c.audioPlayer != nil {
			err := c.audioPlayer.Close()
			if err != nil {
				slog.Error("error closing audio player", "error", err)
			}
		}
	}()

	// Parse command line flags
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		showHelp     = fs.Bool("help", false, "Show help information")
		showVersion  = fs.Bool("version", false, "Show version information")
		configFile   = fs.String("config", "", "Path to config file")
		volume       = fs.String("volume", "", "Set volume (0.0 to 1.0)")
		soundpack    = fs.String("soundpack", "", "Set soundpack to use")
		silent       = fs.Bool("silent", false, "Silent mode - no audio playback")
	)

	err := fs.Parse(args[1:])
	if err != nil {
		slog.Error("flag parsing failed", "error", err)
		return 1
	}

	// Handle help flag
	if *showHelp {
		c.printHelp(stdout)
		return 0
	}

	// Handle version flag
	if *showVersion {
		c.printVersion(stdout)
		return 0
	}

	// Check for extra arguments
	if len(fs.Args()) > 0 {
		fmt.Fprintf(stderr, "Error: unexpected arguments: %v\n", fs.Args())
		slog.Error("unexpected arguments", "args", fs.Args())
		return 1
	}

	// Load configuration
	var cfg *config.Config
	if *configFile != "" {
		cfg, err = c.configManager.LoadFromFile(*configFile)
		if err != nil {
			// If config file doesn't exist, use defaults
			slog.Warn("config file not found, using defaults", "file", *configFile, "error", err)
			cfg = c.configManager.GetDefaultConfig()
		}
	} else {
		cfg, err = c.configManager.LoadConfig()
		if err != nil {
			fmt.Fprintf(stderr, "Error loading config: %v\n", err)
			slog.Error("config load failed", "error", err)
			return 1
		}
	}

	// Apply environment overrides
	cfg = c.configManager.ApplyEnvironmentOverrides(cfg)

	// Apply command line overrides
	if *volume != "" {
		vol, err := strconv.ParseFloat(*volume, 64)
		if err != nil {
			fmt.Fprintf(stderr, "Error: invalid volume value '%s': %v\n", *volume, err)
			slog.Error("invalid volume value", "value", *volume, "error", err)
			return 1
		}
		if vol < 0.0 || vol > 1.0 {
			fmt.Fprintf(stderr, "Error: volume must be between 0.0 and 1.0, got %f\n", vol)
			slog.Error("volume out of range", "value", vol)
			return 1
		}
		cfg.Volume = vol
		slog.Debug("volume override applied", "value", vol)
	}

	if *soundpack != "" {
		cfg.DefaultSoundpack = *soundpack
		slog.Debug("soundpack override applied", "value", *soundpack)
	}

	if *silent {
		cfg.Enabled = false
		slog.Debug("silent mode enabled")
	}

	// Validate final configuration
	err = c.configManager.ValidateConfig(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "Error: invalid configuration: %v\n", err)
		slog.Error("config validation failed", "error", err)
		return 1
	}

	slog.Info("configuration loaded",
		"volume", cfg.Volume,
		"soundpack", cfg.DefaultSoundpack,
		"enabled", cfg.Enabled)

	// Initialize sound loader with soundpack paths
	c.soundLoader = NewSoundLoader(cfg.SoundpackPaths)
	
	// Set audio player volume
	err = c.audioPlayer.SetVolume(float32(cfg.Volume))
	if err != nil {
		fmt.Fprintf(stderr, "Error setting volume: %v\n", err)
		slog.Error("volume setting failed", "error", err)
		return 1
	}

	// Initialize audio context if not in silent mode
	var audioCtx *audio.Context
	if cfg.Enabled {
		audioCtx, err = audio.NewContext()
		if err != nil {
			fmt.Fprintf(stderr, "Error initializing audio: %v\n", err)
			slog.Error("audio initialization failed", "error", err)
			return 1
		}
		defer audioCtx.Close()
		slog.Debug("audio context initialized")
	}

	// Read hook JSON from stdin
	inputData, err := io.ReadAll(stdin)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading from stdin: %v\n", err)
		slog.Error("stdin read failed", "error", err)
		return 1
	}

	// If no input and we're just testing flags/config, return success
	if len(inputData) == 0 {
		slog.Debug("no input data received - configuration test mode")
		return 0
	}

	// Parse hook JSON
	var hookEvent hooks.HookEvent
	err = json.Unmarshal(inputData, &hookEvent)
	if err != nil {
		fmt.Fprintf(stderr, "Error parsing hook JSON: %v\n", err)
		slog.Error("hook JSON parsing failed", "error", err)
		return 1
	}

	// Validate hook event
	if hookEvent.EventName == "" {
		fmt.Fprintf(stderr, "Error: missing required field 'hook_event_name'\n")
		slog.Error("missing hook_event_name field")
		return 1
	}

	if hookEvent.SessionID == "" {
		fmt.Fprintf(stderr, "Error: missing required field 'session_id'\n")
		slog.Error("missing session_id field")
		return 1
	}

	slog.Info("hook event parsed",
		"event_name", hookEvent.EventName,
		"session_id", hookEvent.SessionID,
		"tool_name", hookEvent.ToolName)

	// Process hook event
	return c.processHookEvent(&hookEvent, cfg, audioCtx, stdout, stderr)
}

// processHookEvent processes the parsed hook event
func (c *CLI) processHookEvent(hookEvent *hooks.HookEvent, cfg *config.Config, audioCtx *audio.Context, stdout, stderr io.Writer) int {
	slog.Debug("processing hook event", "event_name", hookEvent.EventName)

	// Extract hook context directly from event
	context := hookEvent.GetContext()

	slog.Info("hook context parsed",
		"category", context.Category.String(),
		"operation", context.Operation,
		"tool", context.ToolName,
		"hint", context.SoundHint)

	// Map to sound file
	result := c.soundMapper.MapSound(context)
	if result == nil {
		slog.Warn("no sound mapping found for event")
		return 0
	}

	slog.Info("sound mapped",
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"selected_path", result.SelectedPath)

	// Play sound if audio is enabled
	if cfg.Enabled && audioCtx != nil {
		err := c.playSound(audioCtx, result.SelectedPath, cfg.Volume)
		if err != nil {
			fmt.Fprintf(stderr, "Error playing sound: %v\n", err)
			slog.Error("sound playback failed", "sound_path", result.SelectedPath, "error", err)
			return 1
		}
		slog.Info("sound played successfully", "sound_path", result.SelectedPath)
	} else {
		slog.Debug("audio disabled, skipping sound playback")
	}

	return 0
}

// playSound plays the specified sound file
func (c *CLI) playSound(audioCtx *audio.Context, soundPath string, volume float64) error {
	slog.Debug("loading and playing sound", "path", soundPath, "volume", volume)

	// Load the sound file
	audioData, err := c.soundLoader.LoadSound(soundPath)
	if err != nil {
		if IsFileNotFoundError(err) {
			slog.Warn("sound file not found, skipping playback", "path", soundPath, "error", err)
			return nil // Don't treat missing sound files as errors
		}
		return fmt.Errorf("failed to load sound: %w", err)
	}

	// Generate a unique sound ID for this playback
	soundID := fmt.Sprintf("claudio-%d", len(soundPath)+int(volume*1000))
	
	// Preload the sound into the audio player
	err = c.audioPlayer.PreloadSound(soundID, audioData)
	if err != nil {
		return fmt.Errorf("failed to preload sound: %w", err)
	}
	
	// Play the sound
	err = c.audioPlayer.PlaySound(soundID)
	if err != nil {
		// Clean up preloaded sound on error
		c.audioPlayer.UnloadSound(soundID)
		return fmt.Errorf("failed to play sound: %w", err)
	}
	
	slog.Info("sound playback started successfully", "path", soundPath, "sound_id", soundID)
	return nil
}

// printHelp prints help information
func (c *CLI) printHelp(w io.Writer) {
	help := `claudio - Claude Code Audio Plugin

usage: claudio [flags]

Usage:
  claudio [flags]

Reads Claude Code hook JSON from stdin and plays appropriate sounds.

Flags:
  --help              Show this help message
  --version           Show version information
  --config FILE       Path to config file
  --volume FLOAT      Set volume (0.0 to 1.0)
  --soundpack NAME    Set soundpack to use
  --silent            Silent mode - no audio playback

Environment Variables:
  CLAUDIO_VOLUME      Override volume setting
  CLAUDIO_SOUNDPACK   Override soundpack setting
  CLAUDIO_ENABLED     Override enabled setting (true/false)
  CLAUDIO_LOG_LEVEL   Override log level (debug/info/warn/error)

Examples:
  echo '{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Bash"}' | claudio
  claudio --volume 0.8 --soundpack mechanical < hook.json
  claudio --silent < hook.json
`
	fmt.Fprint(w, help)
}

// printVersion prints version information
func (c *CLI) printVersion(w io.Writer) {
	version := `claudio version 1.0.0 (Version 1.0.0)
Claude Code Audio Plugin - Hook-based sound system
`
	fmt.Fprint(w, version)
}