package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ctoth/claudio/internal/audio"
	"github.com/ctoth/claudio/internal/config"
	"github.com/ctoth/claudio/internal/hooks"
	"github.com/ctoth/claudio/internal/sounds"
	"github.com/ctoth/claudio/internal/soundpack"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

const Version = "1.3.1"

// CLI represents the command-line interface
type CLI struct {
	rootCmd           *cobra.Command
	configManager     *config.ConfigManager
	soundMapper       *sounds.SoundMapper
	soundpackResolver soundpack.SoundpackResolver
	audioBackend      audio.AudioBackend
	backendFactory    audio.BackendFactory
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
		audioBackend:      nil, // Lazy initialization - only create when needed
		backendFactory:    nil, // Lazy initialization - only create when needed
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
		cmd.Printf("claudio version %s (Version %s)\nClaude Code Audio Plugin - Hook-based sound system\n", Version, Version)
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

// initializeAudioSystem sets up the soundpack resolver and audio backend
func initializeAudioSystem(cmd *cobra.Command, cli *CLI, cfg *config.Config) error {
	slog.Debug("configuration loaded",
		"volume", cfg.Volume,
		"soundpack", cfg.DefaultSoundpack,
		"audio_backend", cfg.AudioBackend,
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
	
	slog.Debug("soundpack resolver initialized",
		"soundpack_name", cfg.DefaultSoundpack,
		"resolver_type", cli.soundpackResolver.GetType(),
		"resolver_name", cli.soundpackResolver.GetName())
	
	// Initialize audio backend system if not in silent mode
	if cfg.Enabled {
		err = cli.initializeAudioSystemWithBackend(cfg)
		if err != nil {
			cmd.PrintErrf("Error initializing audio backend: %v\n", err)
			slog.Error("audio backend initialization failed", "error", err)
			return fmt.Errorf("error initializing audio backend: %w", err)
		}
		slog.Debug("audio backend system initialized")
	}
	
	return nil
}

// initializeAudioSystemWithBackend creates and configures the audio backend
func (c *CLI) initializeAudioSystemWithBackend(cfg *config.Config) error {
	slog.Debug("initializing audio backend", "backend_type", cfg.AudioBackend)
	
	// Create audio backend using factory
	backend, err := c.backendFactory.CreateBackend(cfg.AudioBackend)
	if err != nil {
		slog.Error("failed to create audio backend", "backend_type", cfg.AudioBackend, "error", err)
		return fmt.Errorf("failed to create audio backend '%s': %w", cfg.AudioBackend, err)
	}
	
	c.audioBackend = backend
	
	// Start the backend
	err = c.audioBackend.Start()
	if err != nil {
		slog.Error("failed to start audio backend", "error", err)
		return fmt.Errorf("failed to start audio backend: %w", err)
	}
	
	// Set volume on backend
	err = c.audioBackend.SetVolume(float32(cfg.Volume))
	if err != nil {
		slog.Error("failed to set volume on backend", "volume", cfg.Volume, "error", err)
		return fmt.Errorf("failed to set volume on backend: %w", err)
	}
	
	slog.Debug("audio backend initialized successfully",
		"backend_type", fmt.Sprintf("%T", c.audioBackend),
		"volume", cfg.Volume)
	
	return nil
}

// processHookInput reads hook JSON from stdin and processes it
func processHookInput(cmd *cobra.Command, cli *CLI, cfg *config.Config) error {
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
		"tool_name", getStringPtr(hookEvent.ToolName))

	// Process hook event
	cli.processHookEvent(&hookEvent, cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())
	
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

	// Setup logging with file logging support
	setupLogging(cfg, cmd.ErrOrStderr())

	// No need for additional initialization - systems already initialized

	// Initialize audio and soundpack systems
	err = initializeAudioSystem(cmd, cli, cfg)
	if err != nil {
		return err
	}

	// Process hook input from stdin
	return processHookInput(cmd, cli, cfg)
}


// Run executes the CLI with the given arguments and I/O streams
func (c *CLI) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	slog.Debug("CLI run started", "args", args)
	
	// CRITICAL: Check for version flag BEFORE any system initialization
	// This prevents unnecessary audio player creation for simple version requests
	if len(args) > 1 && (args[1] == "--version" || args[1] == "-v") {
		// Show version immediately without initializing any systems
		fmt.Fprintf(stdout, "claudio version %s (Version %s)\nClaude Code Audio Plugin - Hook-based sound system\n", Version, Version)
		return 0
	}
	
	// Initialize systems only when actually needed (not for version flag)
	c.initializeSystems()
	
	// Ensure audio backend is cleaned up on exit
	defer func() {
		if c.audioBackend != nil {
			err := c.audioBackend.Close()
			if err != nil {
				slog.Error("error closing audio backend", "error", err)
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

// initializeConfigManager initializes only the config manager early for log level configuration
func (c *CLI) initializeConfigManager() {
	if c.configManager == nil {
		c.configManager = config.NewConfigManager()
	}
}

// initializeRemainingSystemsAfterConfig initializes systems that can wait until after log level is configured
func (c *CLI) initializeRemainingSystemsAfterConfig() {
	if c.soundMapper == nil {
		c.soundMapper = sounds.NewSoundMapper()
	}
	if c.backendFactory == nil {
		c.backendFactory = audio.NewBackendFactory()
	}
	if c.terminalDetector == nil {
		c.terminalDetector = &DefaultTerminalDetector{}
	}
	// soundpackResolver and audioBackend are initialized in initializeAudioSystem when needed
}

// initializeSystems lazily initializes remaining CLI components when actually needed
func (c *CLI) initializeSystems() {
	// Config manager should already be initialized
	c.initializeConfigManager()
	
	if c.soundMapper == nil {
		c.soundMapper = sounds.NewSoundMapper()
	}
	if c.backendFactory == nil {
		c.backendFactory = audio.NewBackendFactory()
	}
	if c.terminalDetector == nil {
		c.terminalDetector = &DefaultTerminalDetector{}
	}
	// soundpackResolver and audioBackend are initialized in initializeAudioSystem when needed
}

// processHookEvent processes the parsed hook event
func (c *CLI) processHookEvent(hookEvent *hooks.HookEvent, cfg *config.Config, stdout, stderr io.Writer) {
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
	if cfg.Enabled && c.audioBackend != nil {
		err := c.playSoundWithBackend(result.SelectedPath, cfg.Volume)
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

// playSoundWithBackend plays the specified sound file using the configured audio backend
func (c *CLI) playSoundWithBackend(soundPath string, volume float64) error {
	slog.Debug("loading and playing sound with backend", "path", soundPath, "volume", volume)

	// Use unified soundpack resolver to resolve sound file path
	fullPath, err := c.soundpackResolver.ResolveSound(soundPath)
	if err != nil {
		if soundpack.IsFileNotFoundError(err) {
			slog.Warn("sound file not found, skipping playback", "path", soundPath)
			return nil // Don't treat missing sound files as errors
		}
		return fmt.Errorf("failed to resolve sound path: %w", err)
	}
	
	// Create audio source from file path
	source := audio.NewFileSource(fullPath)
	
	// Play using audio backend
	ctx := context.Background()
	err = c.audioBackend.Play(ctx, source)
	if err != nil {
		slog.Error("backend playback failed", "path", fullPath, "backend_type", fmt.Sprintf("%T", c.audioBackend), "error", err)
		return fmt.Errorf("failed to play sound with backend: %w", err)
	}
	
	slog.Info("sound playback completed successfully", "path", soundPath, "backend_type", fmt.Sprintf("%T", c.audioBackend))
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
  CLAUDIO_VOLUME        Override volume setting
  CLAUDIO_SOUNDPACK     Override soundpack setting
  CLAUDIO_ENABLED       Override enabled setting (true/false)
  CLAUDIO_LOG_LEVEL     Override log level (debug/info/warn/error)
  CLAUDIO_AUDIO_BACKEND Override audio backend (auto/system_command/malgo)

Examples:
  echo '{"hook_event_name":"PostToolUse","session_id":"test","tool_name":"Bash"}' | claudio
  claudio --volume 0.8 --soundpack mechanical < hook.json
  claudio --silent < hook.json
`
	fmt.Fprint(w, help)
}

// printVersion prints version information
func (c *CLI) printVersion(w io.Writer) {
	fmt.Fprintf(w, "claudio version %s (Version %s)\nClaude Code Audio Plugin - Hook-based sound system\n", Version, Version)
}

// setupLogging configures slog with file logging when enabled
func setupLogging(cfg *config.Config, stderrWriter io.Writer) {
	// Parse log level
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		level = slog.LevelInfo // Default level if parsing fails
	}

	// Check if current logger is already more verbose than config specifies
	// This preserves test logger setup
	currentHandler := slog.Default().Handler()
	if textHandler, ok := currentHandler.(*slog.TextHandler); ok {
		// Check if current handler allows DEBUG level but config wants higher level
		if textHandler.Enabled(context.Background(), slog.LevelDebug) && level > slog.LevelDebug {
			// Current handler allows DEBUG but config wants higher level - preserve current handler
			slog.Debug("preserving existing verbose logger setup", "config_level", level.String(), "current_allows", "DEBUG")
			return
		}
	}

	// Always include stderr
	var writers []io.Writer
	writers = append(writers, stderrWriter)

	// Add file logging if enabled
	if cfg.FileLogging != nil && cfg.FileLogging.Enabled {
		// Resolve log file path using config manager
		configManager := config.NewConfigManager()
		logFilePath := configManager.ResolveLogFilePath(cfg.FileLogging.Filename)
		
		// Create log file directory if needed
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			slog.Error("failed to create log directory", "path", logDir, "error", err)
			// Continue without file logging rather than failing
		} else {
			// Create lumberjack logger for file rotation
			fileWriter := &lumberjack.Logger{
				Filename:   logFilePath,
				MaxSize:    cfg.FileLogging.MaxSizeMB,
				MaxBackups: cfg.FileLogging.MaxBackups,
				MaxAge:     cfg.FileLogging.MaxAgeDays,
				Compress:   cfg.FileLogging.Compress,
			}
			writers = append(writers, fileWriter)
			slog.Debug("file logging enabled", "path", logFilePath)
		}
	}

	// Create MultiWriter to combine all writers
	multiWriter := io.MultiWriter(writers...)

	// Create slog handler with combined writer
	handler := slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
		Level: level,
	})

	// Set as default logger
	slog.SetDefault(slog.New(handler))
	
	slog.Debug("logging setup completed", 
		"level", level.String(), 
		"writers", len(writers),
		"file_enabled", cfg.FileLogging != nil && cfg.FileLogging.Enabled)
}

// getStringPtr safely dereferences a string pointer, returning empty string if nil
func getStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}