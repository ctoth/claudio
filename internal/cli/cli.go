package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"

	"github.com/ctoth/claudio/internal/audio"
	"github.com/ctoth/claudio/internal/config"
	"github.com/ctoth/claudio/internal/hooks"
	"github.com/ctoth/claudio/internal/sounds"
	"github.com/ctoth/claudio/internal/soundpack"
	"github.com/spf13/cobra"
)

// CLI represents the command-line interface
type CLI struct {
	rootCmd           *cobra.Command
	configManager     *config.ConfigManager
	soundMapper       *sounds.SoundMapper
	soundpackResolver soundpack.SoundpackResolver
	audioPlayer       *audio.AudioPlayer
	terminalDetector  TerminalDetector
}

// NewCLI creates a new CLI instance
func NewCLI() *CLI {
	slog.Debug("creating new CLI instance")
	
	rootCmd := &cobra.Command{
		Use:   "claudio",
		Short: "Claude Code Audio Plugin",
		Long:  "Claudio is a hook-based audio plugin for Claude Code that plays contextual sounds based on tool usage and events.",
		Run:   runStdinMode, // Default behavior when no subcommand is provided
	}
	
	// Add install subcommand
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install claudio hooks into Claude Code settings",
		Long:  "Install claudio hooks into Claude Code settings to enable audio feedback for tool usage and events.",
		Run:   runInstallCommand,
	}
	rootCmd.AddCommand(installCmd)
	
	return &CLI{
		rootCmd:           rootCmd,
		configManager:     config.NewConfigManager(),
		soundMapper:       sounds.NewSoundMapper(),
		soundpackResolver: nil, // Will be initialized when soundpack is configured
		audioPlayer:       audio.NewAudioPlayer(),
		terminalDetector:  &DefaultTerminalDetector{},
	}
}

// runStdinMode handles the default behavior of reading hook JSON from stdin
func runStdinMode(cmd *cobra.Command, args []string) {
	// This is a placeholder - will be implemented when we migrate the Run method
	// For now, this allows the test to pass
}

// runInstallCommand handles the install subcommand
func runInstallCommand(cmd *cobra.Command, args []string) {
	// This is a placeholder - will be implemented in later commits
	// For now, this allows the test to pass
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
		soundpackFlag = fs.String("soundpack", "", "Set soundpack to use")
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

	if *soundpackFlag != "" {
		cfg.DefaultSoundpack = *soundpackFlag
		slog.Debug("soundpack override applied", "value", *soundpackFlag)
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

	// Initialize unified soundpack resolver with auto-detection
	// Try exact path first (for JSON soundpacks or exact directory paths)
	// Then fallback to XDG directory search for directory soundpacks
	xdgDirs := config.NewXDGDirs()
	soundpackPaths := xdgDirs.GetSoundpackPaths(cfg.DefaultSoundpack)
	soundpackPaths = append(soundpackPaths, cfg.SoundpackPaths...)
	
	// Use factory to create appropriate mapper with fallback to base paths
	mapper, err := soundpack.CreateSoundpackMapperWithBasePaths(
		cfg.DefaultSoundpack, 
		cfg.DefaultSoundpack, // Try exact path first
		soundpackPaths,       // Fallback to base directory search
	)
	if err != nil {
		slog.Warn("failed to create soundpack mapper, using default empty mapper", 
			"soundpack", cfg.DefaultSoundpack, 
			"error", err)
		// Create empty directory mapper as fallback to prevent crashes
		mapper = soundpack.NewDirectoryMapper("fallback", []string{})
	}
	
	c.soundpackResolver = soundpack.NewSoundpackResolver(mapper)
	
	slog.Info("soundpack resolver initialized",
		"soundpack_name", cfg.DefaultSoundpack,
		"resolver_type", c.soundpackResolver.GetType(),
		"resolver_name", c.soundpackResolver.GetName())
	
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
		slog.Info("no input data received - configuration test mode")
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

	// Use unified soundpack resolver to resolve sound file path
	fullPath, err := c.soundpackResolver.ResolveSound(soundPath)
	if err != nil {
		if soundpack.IsFileNotFoundError(err) {
			slog.Warn("sound file not found, skipping playback", "path", soundPath)
			return nil // Don't treat missing sound files as errors
		}
		return fmt.Errorf("failed to resolve sound path: %w", err)
	}
	
	// TEMPORARY TEST: Use paplay directly to avoid crackling
	cmd := exec.Command("paplay", fullPath)
	slog.Debug("running paplay command", "command", cmd.String())
	
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to play sound with paplay: %w", err)
	}
	
	slog.Info("sound playback completed", "path", soundPath)
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