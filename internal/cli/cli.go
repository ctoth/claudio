package cli

import (
	"context"
	"encoding/json"
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
		RunE:  runStdinModeE, // Default behavior when no subcommand is provided
	}
	
	// Add install subcommand
	installCmd := newInstallCommand()
	rootCmd.AddCommand(installCmd)
	
	// Add uninstall subcommand
	uninstallCmd := newUninstallCommand()
	rootCmd.AddCommand(uninstallCmd)
	
	// Add persistent flags to root command for backward compatibility
	rootCmd.PersistentFlags().String("config", "", "Path to config file")
	rootCmd.PersistentFlags().String("volume", "", "Set volume (0.0 to 1.0)")
	rootCmd.PersistentFlags().String("soundpack", "", "Set soundpack to use")
	rootCmd.PersistentFlags().Bool("silent", false, "Silent mode - no audio playback")
	
	// Add version flag
	rootCmd.Flags().BoolP("version", "v", false, "Show version information")
	
	return &CLI{
		rootCmd:           rootCmd,
		configManager:     nil, // Lazy initialization - only create when needed
		soundMapper:       nil, // Lazy initialization - only create when needed
		soundpackResolver: nil, // Lazy initialization - only create when needed
		audioPlayer:       nil, // Lazy initialization - only create when needed
		terminalDetector:  nil, // Lazy initialization - only create when needed
	}
}

// contextWithCLI stores CLI instance in context for command handlers
func contextWithCLI(cli *CLI) context.Context {
	return context.WithValue(context.Background(), "cli", cli)
}

// cliFromContext extracts CLI instance from context
func cliFromContext(ctx context.Context) *CLI {
	if cli, ok := ctx.Value("cli").(*CLI); ok {
		return cli
	}
	return nil
}

// handleVersionFlag checks and handles the version flag
// Returns true if version was handled and processing should stop
func handleVersionFlag(cmd *cobra.Command) (bool, error) {
	version, _ := cmd.Flags().GetBool("version")
	if version {
		cmd.Print(`claudio version 1.2.0 (Version 1.2.0)
Claude Code Audio Plugin - Hook-based sound system
`)
		return true, nil
	}
	return false, nil
}

// loadAndValidateConfig loads configuration from flags and files, applies overrides, and validates
func loadAndValidateConfig(cmd *cobra.Command, cli *CLI) (*config.Config, error) {
	// Get flag values
	configFile, _ := cmd.Flags().GetString("config")
	volumeStr, _ := cmd.Flags().GetString("volume")
	soundpackFlag, _ := cmd.Flags().GetString("soundpack")
	silent, _ := cmd.Flags().GetBool("silent")
	
	// Validate volume flag early to match old behavior
	if volumeStr != "" {
		vol, err := strconv.ParseFloat(volumeStr, 64)
		if err != nil {
			cmd.PrintErrf("Error: invalid volume value '%s': %v\n", volumeStr, err)
			slog.Error("invalid volume value", "value", volumeStr, "error", err)
			return nil, fmt.Errorf("invalid volume value '%s': %w", volumeStr, err)
		}
		if vol < 0.0 || vol > 1.0 {
			cmd.PrintErrf("Error: volume must be between 0.0 and 1.0, got %f\n", vol)
			slog.Error("volume out of range", "value", vol)
			return nil, fmt.Errorf("volume must be between 0.0 and 1.0, got %f", vol)
		}
	}
	
	// Load configuration
	var cfg *config.Config
	var err error
	if configFile != "" {
		cfg, err = cli.configManager.LoadFromFile(configFile)
		if err != nil {
			// If config file doesn't exist, use defaults
			slog.Warn("config file not found, using defaults", "file", configFile, "error", err)
			cfg = cli.configManager.GetDefaultConfig()
		}
	} else {
		cfg, err = cli.configManager.LoadConfig()
		if err != nil {
			cmd.PrintErrf("Error loading config: %v\n", err)
			slog.Error("config load failed", "error", err)
			return nil, fmt.Errorf("error loading config: %w", err)
		}
	}

	// Apply environment overrides
	cfg = cli.configManager.ApplyEnvironmentOverrides(cfg)

	// Apply command line overrides
	if volumeStr != "" {
		// Volume already validated above, just parse and apply
		vol, _ := strconv.ParseFloat(volumeStr, 64)
		cfg.Volume = vol
		slog.Debug("volume override applied", "value", vol)
	}

	if soundpackFlag != "" {
		cfg.DefaultSoundpack = soundpackFlag
		slog.Debug("soundpack override applied", "value", soundpackFlag)
	}

	if silent {
		cfg.Enabled = false
		slog.Debug("silent mode enabled")
	}

	// Validate final configuration
	err = cli.configManager.ValidateConfig(cfg)
	if err != nil {
		cmd.PrintErrf("Error: invalid configuration: %v\n", err)
		slog.Error("config validation failed", "error", err)
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	return cfg, nil
}

// initializeAudioSystem sets up the soundpack resolver and audio context
func initializeAudioSystem(cmd *cobra.Command, cli *CLI, cfg *config.Config) (*audio.Context, error) {
	slog.Info("configuration loaded",
		"volume", cfg.Volume,
		"soundpack", cfg.DefaultSoundpack,
		"enabled", cfg.Enabled)

	// Initialize unified soundpack resolver with auto-detection
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
	
	cli.soundpackResolver = soundpack.NewSoundpackResolver(mapper)
	
	slog.Info("soundpack resolver initialized",
		"soundpack_name", cfg.DefaultSoundpack,
		"resolver_type", cli.soundpackResolver.GetType(),
		"resolver_name", cli.soundpackResolver.GetName())
	
	// Set audio player volume
	err = cli.audioPlayer.SetVolume(float32(cfg.Volume))
	if err != nil {
		cmd.PrintErrf("Error setting volume: %v\n", err)
		slog.Error("volume setting failed", "error", err)
		return nil, fmt.Errorf("error setting volume: %w", err)
	}

	// Initialize audio context if not in silent mode
	var audioCtx *audio.Context
	if cfg.Enabled {
		audioCtx, err = audio.NewContext()
		if err != nil {
			cmd.PrintErrf("Error initializing audio: %v\n", err)
			slog.Error("audio initialization failed", "error", err)
			return nil, fmt.Errorf("error initializing audio: %w", err)
		}
		slog.Debug("audio context initialized")
	}
	
	return audioCtx, nil
}

// processHookInput reads hook JSON from stdin and processes it
func processHookInput(cmd *cobra.Command, cli *CLI, cfg *config.Config, audioCtx *audio.Context) error {
	// Read hook JSON from stdin
	inputData, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		cmd.PrintErrf("Error reading from stdin: %v\n", err)
		slog.Error("stdin read failed", "error", err)
		return fmt.Errorf("error reading from stdin: %w", err)
	}

	// If no input and we're just testing flags/config, return success
	if len(inputData) == 0 {
		slog.Info("no input data received - configuration test mode")
		return nil
	}

	// Parse hook JSON
	var hookEvent hooks.HookEvent
	err = json.Unmarshal(inputData, &hookEvent)
	if err != nil {
		cmd.PrintErrf("Error parsing hook JSON: %v\n", err)
		slog.Error("hook JSON parsing failed", "error", err)
		return fmt.Errorf("error parsing hook JSON: %w", err)
	}

	// Validate hook event
	if hookEvent.EventName == "" {
		cmd.PrintErrln("Error: missing required field 'hook_event_name'")
		slog.Error("missing hook_event_name field")
		return fmt.Errorf("missing required field 'hook_event_name'")
	}

	if hookEvent.SessionID == "" {
		cmd.PrintErrln("Error: missing required field 'session_id'")
		slog.Error("missing session_id field")
		return fmt.Errorf("missing required field 'session_id'")
	}

	slog.Info("hook event parsed",
		"event_name", hookEvent.EventName,
		"session_id", hookEvent.SessionID,
		"tool_name", hookEvent.ToolName)

	// Process hook event
	cli.processHookEvent(&hookEvent, cfg, audioCtx, cmd.OutOrStdout(), cmd.ErrOrStderr())
	
	return nil
}

// runStdinModeE handles the default behavior of reading hook JSON from stdin
func runStdinModeE(cmd *cobra.Command, args []string) error {
	// Extract CLI instance from context
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		slog.Error("CLI instance not found in context")
		return fmt.Errorf("CLI instance not found in context")
	}
	
	// Handle version flag first
	handled, err := handleVersionFlag(cmd)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	
	// Load and validate configuration
	cfg, err := loadAndValidateConfig(cmd, cli)
	if err != nil {
		return err
	}

	// Initialize audio and soundpack systems
	audioCtx, err := initializeAudioSystem(cmd, cli, cfg)
	if err != nil {
		return err
	}
	if audioCtx != nil {
		defer audioCtx.Close()
	}

	// Process hook input from stdin
	return processHookInput(cmd, cli, cfg, audioCtx)
}


// Run executes the CLI with the given arguments and I/O streams
func (c *CLI) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	slog.Debug("CLI run started", "args", args)
	
	// CRITICAL: Check for version flag BEFORE any system initialization
	// This prevents unnecessary audio player creation for simple version requests
	if len(args) > 1 && (args[1] == "--version" || args[1] == "-v") {
		// Show version immediately without initializing any systems
		fmt.Fprint(stdout, `claudio version 1.2.0 (Version 1.2.0)
Claude Code Audio Plugin - Hook-based sound system
`)
		return 0
	}
	
	// Initialize systems only when actually needed (not for version flag)
	c.initializeSystems()
	
	// Ensure audio player is cleaned up on exit
	defer func() {
		if c.audioPlayer != nil {
			err := c.audioPlayer.Close()
			if err != nil {
				slog.Error("error closing audio player", "error", err)
			}
		}
	}()

	// Configure cobra to use the provided I/O streams
	c.rootCmd.SetArgs(args[1:]) // Skip program name
	c.rootCmd.SetIn(stdin)
	c.rootCmd.SetOut(stdout)
	c.rootCmd.SetErr(stderr)
	
	// Store CLI instance for access in command handlers
	c.rootCmd.SetContext(contextWithCLI(c))
	
	// Execute cobra command
	if err := c.rootCmd.Execute(); err != nil {
		slog.Error("cobra execution failed", "error", err)
		return 1
	}
	
	return 0
}

// initializeSystems lazily initializes all CLI components when actually needed
func (c *CLI) initializeSystems() {
	if c.configManager == nil {
		c.configManager = config.NewConfigManager()
	}
	if c.soundMapper == nil {
		c.soundMapper = sounds.NewSoundMapper()
	}
	if c.audioPlayer == nil {
		c.audioPlayer = audio.NewAudioPlayer()
	}
	if c.terminalDetector == nil {
		c.terminalDetector = &DefaultTerminalDetector{}
	}
	// soundpackResolver is initialized in initializeAudioSystem when needed
}

// processHookEvent processes the parsed hook event
func (c *CLI) processHookEvent(hookEvent *hooks.HookEvent, cfg *config.Config, audioCtx *audio.Context, stdout, stderr io.Writer) {
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
		return
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
			return
		}
		slog.Info("sound played successfully", "sound_path", result.SelectedPath)
	} else {
		slog.Debug("audio disabled, skipping sound playback")
	}
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
	version := `claudio version 1.2.0 (Version 1.2.0)
Claude Code Audio Plugin - Hook-based sound system
`
	fmt.Fprint(w, version)
}