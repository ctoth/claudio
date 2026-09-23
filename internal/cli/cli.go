package cli

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"claudio.click/internal/audio"
	"claudio.click/internal/config"
	"claudio.click/internal/hooks"
	"claudio.click/internal/safeio"
	"claudio.click/internal/soundpack"
	"claudio.click/internal/sounds"
	"claudio.click/internal/tracking"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"gopkg.in/natefinch/lumberjack.v2"
)

const Version = "1.14.0"

// CLI represents the command-line interface
type CLI struct {
	rootCmd           *cobra.Command
	configManager     *config.ConfigManager
	soundpackResolver soundpack.SoundpackResolver
	audioBackend      audio.AudioBackend
	trackingDB        *sql.DB // Optional tracking database
}

// NewCLI creates a new CLI instance. Command handlers are closures over
// the returned *CLI, so they share its config manager and lazily opened
// resources (resolver, audio backend, tracking DB) without a context lookup.
func NewCLI() *CLI {
	slog.Debug("creating new CLI instance")

	c := &CLI{configManager: config.NewConfigManager()}

	rootCmd := &cobra.Command{
		Use:     "claudio",
		Short:   "Coding-agent audio plugin",
		Long:    "Claudio is a hook-based audio plugin for coding agents that plays contextual sounds based on tool usage and events.",
		Version: Version,
		RunE:    c.runStdinMode, // Default behavior when no subcommand is provided
		// Run prints a failed command's error once. Usage text is for
		// --help only: on stdout an agent would read it as hook output.
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	// Preserve the historical version output shape ("claudio version X (Version X)\n...")
	// that downstream tooling and tests check against.
	rootCmd.SetVersionTemplate("claudio version " + Version + " (Version " + Version + ")\nCoding-agent audio plugin - Hook-based sound system\n")

	rootCmd.AddCommand(
		newInstallCommand(),
		newUninstallCommand(),
		newAnalyzeCommand(c),
		newSoundpackCommand(c),
		newVolumeCommand(c),
		newMuteCommand(c),
		newUnmuteCommand(c),
		newStatusCommand(c),
		// install-commands writes the /claudio slash command markdown;
		// uninstall-commands removes it.
		newInstallCommandsCommand(),
		newUninstallCommandsCommand(),
	)

	// Add persistent flags to root command for backward compatibility
	rootCmd.PersistentFlags().String("config", "", "Path to config file")
	rootCmd.PersistentFlags().String("volume", "", "Set volume (0.0 to 1.0)")
	rootCmd.PersistentFlags().String("soundpack", "", "Set soundpack to use")
	rootCmd.PersistentFlags().Bool("silent", false, "Silent mode - no audio playback")
	rootCmd.PersistentFlags().Bool("daemon-child", false, "Internal: run as detached hook worker")
	rootCmd.PersistentFlags().String("hook-input-file", "", "Internal: hook payload file for detached worker")
	rootCmd.PersistentFlags().String("hook-agent", "", "Internal: agent that invoked this hook")
	rootCmd.PersistentFlags().String("hook-event", "", "Internal: event name when hook payload omits it")
	_ = rootCmd.PersistentFlags().MarkHidden("daemon-child")
	_ = rootCmd.PersistentFlags().MarkHidden("hook-input-file")
	_ = rootCmd.PersistentFlags().MarkHidden("hook-agent")
	_ = rootCmd.PersistentFlags().MarkHidden("hook-event")

	// Note: cobra automatically registers a `--version` boolean flag (and
	// short `-v`) once rootCmd.Version is set.

	c.rootCmd = rootCmd
	return c
}

// loadAndValidateConfig loads configuration from flags and files, applies overrides, and validates
func (c *CLI) loadAndValidateConfig(cmd *cobra.Command) (*config.Config, error) {
	// Get flag values
	volumeStr, _ := cmd.Flags().GetString("volume")
	soundpackFlag, _ := cmd.Flags().GetString("soundpack")
	silent, _ := cmd.Flags().GetBool("silent")

	// Parse the volume flag before loading anything; its range is checked
	// with the rest of the final configuration below.
	var volumeOverride *float64
	if volumeStr != "" {
		vol, err := strconv.ParseFloat(volumeStr, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid volume value '%s': %w", volumeStr, err)
		}
		volumeOverride = &vol
	}

	// Load configuration. An unusable file is a warning, never a failed hook.
	loaded := c.loadConfig(cmd)
	loaded.warnIgnored(cmd)
	cfg := loaded.Config

	// Apply environment overrides
	cfg = c.configManager.ApplyEnvironmentOverrides(cfg)

	// Apply command line overrides
	if volumeOverride != nil {
		cfg.Volume = volumeOverride
		slog.Debug("volume override applied", "value", *volumeOverride)
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
	if err := c.configManager.ValidateConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// initializeAudioSystem sets up the soundpack resolver and, unless audio is
// disabled, the audio backend.
func (c *CLI) initializeAudioSystem(cfg *config.Config) error {
	slog.Debug("initializing audio system",
		"volume", cfg.Volume,
		"soundpack", cfg.DefaultSoundpack,
		"audio_backend", cfg.AudioBackend,
		"enabled", cfg.Enabled)

	c.soundpackResolver = soundpack.NewSoundpackResolver(c.selectSoundpackMapper(cfg))
	slog.Debug("soundpack resolver initialized",
		"soundpack_name", cfg.DefaultSoundpack,
		"resolver_type", c.soundpackResolver.GetType(),
		"resolver_name", c.soundpackResolver.GetName())

	if !cfg.Enabled {
		return nil
	}
	if err := c.initializeAudioSystemWithBackend(cfg); err != nil {
		return fmt.Errorf("error initializing audio backend: %w", err)
	}
	slog.Debug("audio backend system initialized")
	return nil
}

// embeddedSoundpackPrefix marks a soundpack source baked into the binary
// ("embedded:<platform>.json") rather than a filesystem path.
const embeddedSoundpackPrefix = "embedded:"

func isEmbeddedSoundpack(source string) bool {
	return strings.HasPrefix(source, embeddedSoundpackPrefix)
}

// errNoPlatformSoundpack reports that neither a platform JSON file next to
// the executable nor an embedded platform pack exists.
var errNoPlatformSoundpack = errors.New("no platform soundpack available")

// selectSoundpackMapper returns the mapper for the configured soundpack,
// falling back to the platform pack and then to an empty mapper, so a hook
// never fails over a missing soundpack.
func (c *CLI) selectSoundpackMapper(cfg *config.Config) soundpack.PathMapper {
	mapper, err := configuredSoundpackMapper(cfg)
	if err == nil {
		return mapper
	}
	slog.Debug("configured soundpack unavailable, trying platform soundpack",
		"soundpack", cfg.DefaultSoundpack, "error", err)

	mapper, err = c.platformSoundpackMapper()
	if err == nil {
		return mapper
	}
	if errors.Is(err, errNoPlatformSoundpack) {
		slog.Debug("no platform soundpack found, using empty mapper")
	} else {
		slog.Warn("platform soundpack failed to load", "error", err)
	}
	return soundpack.NewDirectoryMapper("fallback", []string{})
}

// resolveSoundpackSource turns a configured soundpack value into what to
// load: an embedded identifier or a filesystem path. A known name resolves
// exactly as `soundpack list` shows it (the lookup `soundpack use`
// validates against); any other value is taken as a path.
func resolveSoundpackSource(value string, configPaths []string) string {
	if isEmbeddedSoundpack(value) {
		return value
	}
	pack, ok := lookupSoundpack(value, configPaths)
	if !ok {
		return value
	}
	slog.Debug("resolved soundpack name", "name", value, "type", pack.Type, "path", pack.Path)
	if pack.Type == soundpackTypeEmbedded {
		return pack.Identifier
	}
	return pack.Path
}

// configuredSoundpackMapper loads cfg.DefaultSoundpack. A path that does
// not exist is an error, so the caller falls back to the platform pack; a
// path that exists but cannot be loaded as a pack is searched for by name
// in the soundpack base directories.
func configuredSoundpackMapper(cfg *config.Config) (soundpack.PathMapper, error) {
	source := resolveSoundpackSource(cfg.DefaultSoundpack, cfg.SoundpackPaths)
	if isEmbeddedSoundpack(source) {
		mapper, err := loadEmbeddedPlatformSoundpack(source)
		if err != nil {
			slog.Warn("failed to load embedded platform soundpack from config",
				"soundpack", cfg.DefaultSoundpack, "identifier", source, "error", err)
		}
		return mapper, err
	}

	if _, err := os.Stat(source); err != nil {
		return nil, fmt.Errorf("configured soundpack %q: %w", cfg.DefaultSoundpack, err)
	}
	basePaths := append(config.SoundpackPaths(cfg.DefaultSoundpack), cfg.SoundpackPaths...)
	return soundpack.CreateSoundpackMapperWithBasePaths(cfg.DefaultSoundpack, source, basePaths)
}

// platformSoundpackMapper loads the platform pack: a platform JSON file next
// to the executable (development builds) or the embedded one.
func (c *CLI) platformSoundpackMapper() (soundpack.PathMapper, error) {
	source := c.configManager.GetPlatformSoundpack(getPlatformExecutableDirectory())
	switch {
	case source == "default":
		return nil, errNoPlatformSoundpack
	case isEmbeddedSoundpack(source):
		return loadEmbeddedPlatformSoundpack(source)
	default:
		return soundpack.CreateSoundpackMapper(source, source)
	}
}

// initializeAudioSystemWithBackend creates and configures the audio backend
func (c *CLI) initializeAudioSystemWithBackend(cfg *config.Config) error {
	slog.Debug("initializing audio backend", "backend_type", cfg.AudioBackend)

	// Create audio backend using package-level constructor
	backend, err := audio.NewBackend(cfg.AudioBackend)
	if err != nil {
		return fmt.Errorf("failed to create audio backend '%s': %w", cfg.AudioBackend, err)
	}

	c.audioBackend = backend

	volume := cfg.EffectiveVolume()
	err = c.audioBackend.SetVolume(float32(volume))
	if err != nil {
		return fmt.Errorf("failed to set volume on backend: %w", err)
	}

	slog.Debug("audio backend initialized successfully",
		"backend_type", fmt.Sprintf("%T", c.audioBackend),
		"volume", volume)

	return nil
}

// payloadFingerprint is a short SHA-256 prefix that identifies a hook
// payload in logs without revealing its content.
func payloadFingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6])
}

// readHookInput reads hook JSON from stdin or an internal payload file.
func readHookInput(cmd *cobra.Command) ([]byte, error) {
	hookInputFile, _ := cmd.Flags().GetString("hook-input-file")
	if hookInputFile != "" {
		f, err := os.Open(hookInputFile)
		if err != nil {
			return nil, fmt.Errorf("error reading hook input file: %w", err)
		}
		inputData, err := safeio.ReadAllCapped(f, safeio.MaxHookPayloadBytes, "hook spool")
		closeErr := f.Close()
		if err != nil {
			return nil, fmt.Errorf("error reading hook input file: %w", err)
		}
		if closeErr != nil {
			slog.Warn("failed to close hook input file", "file", hookInputFile, "error", closeErr)
		}

		if removeErr := os.Remove(hookInputFile); removeErr != nil {
			slog.Warn("failed to remove hook input file", "file", hookInputFile, "error", removeErr)
		}

		return inputData, nil
	}

	// Bounded, not ReadAllCapped: the hook stdin pipe is not guaranteed to
	// deliver EOF (Windows git-bash holds it open — see ReadJSONBounded), and
	// blocking here wedges the spawning agent at "running stop hooks".
	inputData, err := safeio.ReadJSONBounded(cmd.InOrStdin(), safeio.MaxHookPayloadBytes, safeio.DefaultHookReadDeadline, "hook payload")
	if err != nil {
		return nil, fmt.Errorf("error reading from stdin: %w", err)
	}

	return inputData, nil
}

// processHookInput processes parsed hook JSON payload.
func processHookInput(cmd *cobra.Command, cli *CLI, cfg *config.Config, inputData []byte) error {
	// If no input and we're just testing flags/config, return success
	if len(inputData) == 0 {
		slog.Info("no input data received - configuration test mode")
		return nil
	}

	defaultEvent, _ := cmd.Flags().GetString("hook-event")
	hookEvent, err := hooks.ParseHookEventWithDefault(inputData, defaultEvent)
	if err != nil {
		// The payload can carry prompts and tool output: identify it by
		// size and hash only. Run logs the error itself.
		slog.Warn("hook payload rejected",
			"payload_bytes", len(inputData),
			"payload_sha256", payloadFingerprint(inputData))
		return fmt.Errorf("error parsing hook JSON: %w", err)
	}

	slog.Info("hook event parsed",
		"event_name", hookEvent.EventName,
		"session_id", hookEvent.SessionID,
		"tool_name", getStringPtr(hookEvent.ToolName))

	// Process hook event.
	cli.processHookEvent(hookEvent, cfg, cmd.OutOrStdout(), cmd.ErrOrStderr())

	return nil
}

// runStdinMode handles the default behavior of reading hook JSON from stdin
func (c *CLI) runStdinMode(cmd *cobra.Command, _ []string) error {
	// Load and validate configuration. (Note: --version is handled by
	// cobra itself before RunE is invoked because rootCmd.Version is set.)
	cfg, err := c.loadAndValidateConfig(cmd)
	if err != nil {
		return err
	}

	// Setup logging with file logging support
	setupLogging(cfg, cmd.ErrOrStderr())

	// Read hook input payload once so we can optionally detach.
	inputData, err := readHookInput(cmd)
	if err != nil {
		return err
	}

	// Structural failures must reach the caller before detachment redirects the
	// worker's stderr. This does not open a device or test playback.
	if cfg.Enabled && len(inputData) > 0 {
		if _, err := audio.ResolveBackend(cfg.AudioBackend); err != nil {
			return err
		}
	}

	// Default behavior: detach hook processing so the invoking hook returns immediately.
	if shouldDetachHookProcessing(cmd, cfg, inputData) {
		if err := spawnDetachedHookWorker(cmd, inputData); err != nil {
			return err
		}
		return writeJSONHookSuccessResponse(cmd, inputData)
	}

	// Initialize tracking (before audio system initialization). Pass the
	// already-loaded cfg so a user-supplied --config is honored
	// (initializeTracking previously called LoadConfig itself, dropping
	// the override).
	c.initializeTracking(cfg)

	// Initialize audio and soundpack systems
	err = c.initializeAudioSystem(cfg)
	if err != nil {
		return err
	}

	// Process hook input payload.
	if err := processHookInput(cmd, c, cfg, inputData); err != nil {
		return err
	}
	return writeJSONHookSuccessResponse(cmd, inputData)
}

func writeJSONHookSuccessResponse(cmd *cobra.Command, inputData []byte) error {
	if len(inputData) == 0 {
		return nil
	}
	hookAgent, _ := cmd.Flags().GetString("hook-agent")
	hookAgent = strings.ToLower(strings.TrimSpace(hookAgent))
	if hookAgent == "gemini" || hookAgent == "qwen" || hookAgent == "copilot" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "{}"); err != nil {
			return fmt.Errorf("failed to write %s hook response: %w", hookAgent, err)
		}
	}
	return nil
}

// Run executes the CLI with the given arguments and I/O streams
func (c *CLI) Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	slog.Debug("CLI run started", "args", args)
	setupDefaultCommandLogging(stderr)

	// Ensure resources are cleaned up on exit
	defer func() {
		if c.audioBackend != nil {
			err := c.audioBackend.Close()
			if err != nil {
				slog.Error("error closing audio backend", "error", err)
			}
		}
		if c.trackingDB != nil {
			err := c.trackingDB.Close()
			if err != nil {
				slog.Error("error closing tracking database", "error", err)
			}
		}
	}()

	c.rootCmd.SetArgs(args[1:])
	c.rootCmd.SetIn(stdin)
	c.rootCmd.SetOut(stdout)
	c.rootCmd.SetErr(stderr)

	// Cobra's own error and usage printing is silenced on the root, so this
	// is the only place a command error reaches stderr; the log record is
	// WARN so the ERROR-only stderr handler does not repeat it.
	if err := c.rootCmd.Execute(); err != nil {
		fmt.Fprintf(stderr, "Error: %v\n", err)
		slog.Warn("command failed", "error", err)
		return 1
	}
	return 0
}

func setupDefaultCommandLogging(stderr io.Writer) {
	slog.SetDefault(slog.New(newStartupHandler(slog.NewTextHandler(stderr, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))))
}

// processHookEvent processes the parsed hook event
func (c *CLI) processHookEvent(hookEvent *hooks.HookEvent, cfg *config.Config, stdout, stderr io.Writer) {
	slog.Debug("processing hook event", "event_name", hookEvent.EventName)

	// Extract hook context directly from event
	eventCtx := hookEvent.GetContext()

	slog.Debug("hook context parsed",
		"category", eventCtx.Category.String(),
		"operation", eventCtx.Operation,
		"tool", eventCtx.ToolName,
		"hint", eventCtx.SoundHint)

	// Compose the resolution pipeline. The mapper drives chain construction
	// and resolution; the LookupBuffer (when tracking is on) wires
	// per-candidate observation into the EventRecorder.RecordEvent payload.
	//
	// End-state dependency direction: sounds knows nothing about tracking.
	// The CLI is the orchestrator that buys observation from the resolver
	// (via soundpack.WithObserver) and writes it to tracking — preserving
	// Chunk 13's one-RecordEvent-per-MapSound invariant at the CLI seam.
	ctx := context.Background()

	var buf *tracking.LookupBuffer
	var dbHook *tracking.DBHook
	var observer soundpack.PathObserver
	if c.trackingDB != nil {
		buf = tracking.NewLookupBuffer()
		dbHook = tracking.NewDBHook(c.trackingDB, hookEvent.SessionID)
		observer = buf.Observer()
		slog.Debug("created LookupBuffer + DBHook for tracking", "session_id", hookEvent.SessionID)
	} else {
		slog.Debug("tracking disabled; mapper resolves without an observer")
	}

	soundMapper := sounds.NewSoundMapperWithResolver(c.soundpackResolver, observer)

	result := soundMapper.MapSound(ctx, eventCtx)
	if result == nil {
		slog.Warn("no sound mapping found for event")
		return
	}

	// One RecordEvent per MapSound, post-resolution, with the full deduped
	// lookup chain and the chosen winner. Errors are logged at WARN and do
	// NOT propagate — tracking is best-effort.
	if buf != nil && dbHook != nil {
		if err := dbHook.RecordEvent(ctx, eventCtx, string(result.ChainType), buf.Lookups(), result.SelectedPath); err != nil {
			slog.Warn("sound tracking RecordEvent failed (continuing)",
				"error", err,
				"chain_type", result.ChainType,
				"selected_path", result.SelectedPath,
				"lookups", len(buf.Lookups()))
		}
	}

	slog.Debug("sound mapped",
		"fallback_level", result.FallbackLevel,
		"total_paths", result.TotalPaths,
		"selected_path", result.SelectedPath)

	// Play sound if audio is enabled
	if cfg.Enabled && c.audioBackend != nil {
		err := c.playSoundWithBackend(result.SelectedPath, cfg.EffectiveVolume())
		if err != nil {
			slog.Error("sound playback failed", "sound_path", result.SelectedPath, "error", err)
			return
		}
		slog.Debug("sound played successfully", "sound_path", result.SelectedPath)
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

	// Create audio source from file path; the backend owns decoding.
	source := audio.NewFileSource(fullPath)

	// Play using audio backend
	ctx := context.Background()
	err = c.audioBackend.Play(ctx, source)
	if err != nil {
		return fmt.Errorf("failed to play sound with backend: %w", err)
	}

	slog.Debug("sound playback completed successfully", "path", soundPath, "backend_type", fmt.Sprintf("%T", c.audioBackend))
	return nil
}

// setupLogging configures slog with dual-level logging:
// - stderr: ERROR level only (for genuine user-facing errors)
// - file: configured level (for full debugging history)
func setupLogging(cfg *config.Config, stderrWriter io.Writer) {
	// Parse configured log level for file logging
	var fileLevel slog.Level
	if err := fileLevel.UnmarshalText([]byte(cfg.LogLevel)); err != nil {
		fileLevel = slog.LevelInfo // Default level if parsing fails
	}

	var handlers []slog.Handler

	// Preserve an already-installed verbose handler (test setup) by adding it
	// as one of the multi-handler outputs instead of returning early. The
	// previous early-return silently dropped file logging whenever a test
	// installed a DEBUG-level default handler, violating the chunk-1
	// "Dual Output" contract.
	currentHandler := slog.Default().Handler()
	startup, _ := currentHandler.(*startupHandler)
	if textHandler, ok := currentHandler.(*slog.TextHandler); ok {
		if textHandler.Enabled(context.Background(), slog.LevelDebug) && fileLevel > slog.LevelDebug {
			slog.Debug("preserving existing verbose logger as additional handler", "config_level", fileLevel.String(), "current_allows", "DEBUG")
			handlers = append(handlers, currentHandler)
		}
	}

	// Always create stderr handler with ERROR level only
	// This ensures users only see genuine errors, not debug/info/warn spam
	stderrHandler := slog.NewTextHandler(stderrWriter, &slog.HandlerOptions{
		Level: slog.LevelError,
	})
	handlers = append(handlers, stderrHandler)

	// Add file logging handler if enabled
	if cfg.FileLogging != nil && cfg.FileLogging.Enabled {
		// Resolve log file path using config manager
		configManager := config.NewConfigManager()
		logFilePath := configManager.ResolveLogFilePath(cfg.FileLogging.Filename)

		// Create log file directory if needed
		logDir := filepath.Dir(logFilePath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			// Log error to stderr handler, but continue
			slog.Error("failed to create log directory", "path", logDir, "error", err)
		} else {
			// Create lumberjack logger for file rotation
			fileWriter := &lumberjack.Logger{
				Filename:   logFilePath,
				MaxSize:    cfg.FileLogging.MaxSizeMB,
				MaxBackups: cfg.FileLogging.MaxBackups,
				MaxAge:     cfg.FileLogging.MaxAgeDays,
				Compress:   cfg.FileLogging.Compress,
			}

			// File handler uses the configured log level (can be debug, info, warn, error)
			fileHandler := slog.NewTextHandler(fileWriter, &slog.HandlerOptions{
				Level: fileLevel,
			})
			handlers = append(handlers, fileHandler)
			// stderr already printed the startup ERRORs; only the file lacks them.
			if startup != nil {
				startup.replay(context.Background(), fileHandler)
			}

			// Use the stderr handler we already created to log this
			// (won't show to user since it's DEBUG level and stderr is ERROR only)
			slog.Debug("file logging enabled", "path", logFilePath)
		}
	}

	// Combine handlers using multi-level handler
	multiHandler := NewMultiLevelHandler(handlers...)

	// Set as default logger
	slog.SetDefault(slog.New(multiHandler))

	// This debug log will only go to file, not stderr (since stderr is ERROR only)
	slog.Debug("logging setup completed",
		"file_level", fileLevel.String(),
		"stderr_level", "error",
		"handlers", len(handlers),
		"file_enabled", cfg.FileLogging != nil && cfg.FileLogging.Enabled)
}

// initializeTracking initializes the tracking database if enabled in the
// supplied configuration. The caller must pass the cfg they already
// loaded — re-loading inside this function (the previous behavior) lost
// any --config override the user passed, because the second LoadConfig
// went through the env+default search path instead.
func (c *CLI) initializeTracking(cfg *config.Config) {
	slog.Debug("initializeTracking() called", "trackingDB_nil", c.trackingDB == nil)

	if c.trackingDB != nil {
		slog.Debug("tracking database already initialized, skipping")
		return // Already initialized
	}

	if cfg == nil {
		slog.Debug("initializeTracking called with nil cfg; skipping")
		return
	}

	slog.Debug("tracking config loaded",
		"tracking_nil", cfg.SoundTracking == nil,
		"enabled", cfg.SoundTracking != nil && cfg.SoundTracking.Enabled,
		"db_path", func() string {
			if cfg.SoundTracking != nil {
				return cfg.SoundTracking.DatabasePath
			}
			return ""
		}())

	// Check if tracking is enabled
	if cfg.SoundTracking == nil || !cfg.SoundTracking.Enabled {
		slog.Debug("sound tracking disabled, skipping database initialization",
			"tracking_nil", cfg.SoundTracking == nil,
			"enabled", cfg.SoundTracking != nil && cfg.SoundTracking.Enabled)
		return
	}

	// Determine database path
	var dbPath string
	if cfg.SoundTracking.DatabasePath != "" {
		dbPath = cfg.SoundTracking.DatabasePath
		slog.Debug("using custom database path from config", "path", dbPath)
	} else {
		// Use default XDG cache path
		var err error
		dbPath, err = tracking.GetDatabasePath()
		if err != nil {
			slog.Error("failed to get database path, continuing without tracking", "error", err)
			return // Graceful degradation
		}
		slog.Debug("using default XDG database path", "path", dbPath)
	}

	slog.Debug("attempting to initialize tracking database", "path", dbPath)

	// Initialize database with graceful degradation
	db, err := tracking.NewDatabase(dbPath)
	if err != nil {
		slog.Error("failed to initialize tracking database, continuing without tracking",
			"path", dbPath, "error", err)
		return // Graceful degradation - continue without tracking
	}

	c.trackingDB = db
	slog.Info("tracking database initialized successfully", "path", dbPath)
}

// getStringPtr safely dereferences a string pointer, returning empty string if nil
func getStringPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// getPlatformExecutableDirectory returns the directory containing the current executable for platform JSON detection
func getPlatformExecutableDirectory() string {
	executable, err := os.Executable()
	if err != nil {
		slog.Warn("failed to get executable directory for platform detection, using current directory", "error", err)
		return "."
	}

	execDir := filepath.Dir(executable)
	slog.Debug("executable directory detected for platform detection", "executable", executable, "directory", execDir)

	return execDir
}

// loadEmbeddedPlatformSoundpack loads a platform soundpack from embedded data
func loadEmbeddedPlatformSoundpack(identifier string) (soundpack.PathMapper, error) {
	filename, ok := strings.CutPrefix(identifier, embeddedSoundpackPrefix)
	if !ok {
		return nil, fmt.Errorf("invalid embedded soundpack identifier: %s", identifier)
	}

	slog.Debug("loading embedded platform soundpack", "filename", filename)

	data, err := config.GetEmbeddedPlatformSoundpackData(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded platform soundpack: %w", err)
	}

	basePaths := embeddedPlatformSoundpackBasePaths(filename, data)
	mapper, err := soundpack.LoadEmbeddedPlatformSoundpack(data, basePaths...)
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded platform soundpack: %w", err)
	}

	slog.Debug("embedded platform soundpack loaded successfully", "filename", filename)
	return mapper, nil
}

func embeddedPlatformSoundpackBasePaths(filename string, data []byte) []string {
	ids := []string{}

	if spFile, err := soundpack.PeekJSONSoundpackFromBytes(data); err == nil {
		ids = append(ids, spFile.Name)
		if strings.HasSuffix(spFile.Name, "-default") {
			ids = append(ids, strings.TrimSuffix(spFile.Name, "-default"))
		}
	}

	fileID := strings.TrimSuffix(filename, filepath.Ext(filename))
	ids = append(ids, fileID, fileID+"-default", "default", "")

	seen := make(map[string]struct{})
	var paths []string
	for _, id := range ids {
		for _, path := range config.SoundpackPaths(id) {
			cleaned := filepath.Clean(path)
			if _, exists := seen[cleaned]; exists {
				continue
			}
			seen[cleaned] = struct{}{}
			paths = append(paths, cleaned)
		}
	}

	// The native-Linux pack references its default tones by bare filename
	// (e.g. "default-success.wav") because a bare Linux box ships no
	// guaranteed system WAVs. Those tones are embedded in the binary and
	// materialized to the cache dir here, appended LAST so the shipped
	// defaults are a fallback only: a user who installs linux-default sounds
	// in their XDG data dir still wins. Packs that map to absolute system
	// paths (Windows/macOS/WSL) name no embedded sound, so this is a no-op.
	if extracted := ensureEmbeddedDefaultSoundsExtracted(data); extracted != "" {
		cleaned := filepath.Clean(extracted)
		if _, exists := seen[cleaned]; !exists {
			paths = append(paths, cleaned)
		}
	}
	return paths
}

// ensureEmbeddedDefaultSoundsExtracted materializes any of the pack's mapping
// values that name an embedded default sound into a cache directory and
// returns that directory (or "" if the pack references none). The shipped
// tones back the native-Linux platform pack; Windows/macOS/WSL packs map to
// absolute system paths that name no embedded sound, so this is a no-op for
// them. Extraction targets the cache dir — not the user's data dir — because
// these bytes are regenerable from the binary, not user-installed content.
func ensureEmbeddedDefaultSoundsExtracted(data []byte) string {
	spFile, err := soundpack.PeekJSONSoundpackFromBytes(data)
	if err != nil {
		return ""
	}

	destDir := config.CachePath("embedded-soundpacks", spFile.Name)
	var wrote bool
	for _, value := range spFile.Mappings {
		soundBytes, err := config.GetEmbeddedSoundData(value)
		if err != nil {
			continue // absolute path or otherwise not an embedded sound
		}
		if err := writeCachedSoundIfMissing(filepath.Join(destDir, value), soundBytes); err != nil {
			slog.Warn("failed to materialize embedded default sound",
				"name", value, "dir", destDir, "error", err)
			continue
		}
		wrote = true
	}

	if !wrote {
		return ""
	}
	slog.Debug("materialized embedded default sounds", "pack", spFile.Name, "dir", destDir)
	return destDir
}

// writeCachedSoundIfMissing writes data to path only when it is not already
// present, via a temp file + rename so concurrent claudio hook processes
// never observe a torn file. The embedded bytes are identical across runs, so
// a redundant write under a race is harmless.
func writeCachedSoundIfMissing(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// 0600 matches the os.CreateTemp default these cached sounds always had.
	return safeio.WriteFileAtomic(afero.NewOsFs(), path, data, 0o600, ".sound-*.tmp")
}
