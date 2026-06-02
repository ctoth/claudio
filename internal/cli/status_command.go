package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// newStatusCommand returns the `claudio status` read-only subcommand.
// Prints the effective configuration (file values + environment
// overrides) so the user can see what their persisted settings will
// produce.
//
// Screen-reader cue: when Enabled=false the output contains the
// literal token `MUTED` beside the enabled line. This is intentional
// and load-bearing — claudio's primary maintainer reads via screen
// reader, so the cue must be a real word in the output, not a color
// or icon.
func newStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current claudio configuration",
		Long: `Show the current effective claudio configuration.

Reports persisted values from config.json combined with any
environment-variable overrides currently set. Does not apply CLI
flag overrides — those are transient and meaningful only for a
single hook invocation.

When audio is disabled, the output includes the literal token MUTED
next to the enabled line. This is a screen-reader cue.`,
		Args: cobra.NoArgs,
		RunE: runStatusE,
	}
}

func runStatusE(cmd *cobra.Command, _ []string) error {
	cli := cliFromContext(cmd.Context())
	if cli == nil {
		return fmt.Errorf("CLI instance not found in context")
	}
	cli.initializeConfigManager()

	// Resolve which config file (if any) actually exists on disk so
	// we can report its location. We don't use the writable path —
	// for status we want to show the first FOUND config, mirroring
	// the search order in LoadConfig.
	configPathDisplay, cfg, err := loadConfigForStatus(cmd, cli)
	if err != nil {
		return err
	}

	// Apply env overrides so the report reflects runtime-effective values.
	cfg = cli.configManager.ApplyEnvironmentOverrides(cfg)

	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "claudio status")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  config file:    %s\n", configPathDisplay)

	// Enabled — with the literal MUTED token when false. Screen-reader cue.
	if cfg.Enabled {
		fmt.Fprintln(out, "  enabled:        true")
	} else {
		// IMPORTANT: the literal token "MUTED" MUST appear here. It is
		// the audible screen-reader cue, not a visual decoration.
		fmt.Fprintln(out, "  enabled:        false (MUTED)")
	}

	// Volume — annotate the source so the user understands precedence.
	volStr, volSource := describeVolume(cfg)
	fmt.Fprintf(out, "  volume:         %s (%s)\n", volStr, volSource)

	fmt.Fprintf(out, "  soundpack:      %s\n", cfg.DefaultSoundpack)
	fmt.Fprintf(out, "  log level:      %s\n", cfg.LogLevel)
	fmt.Fprintf(out, "  audio backend:  %s\n", cfg.AudioBackend)

	if cfg.FileLogging != nil && cfg.FileLogging.Enabled {
		path := cli.configManager.ResolveLogFilePath(cfg.FileLogging.Filename)
		fmt.Fprintf(out, "  file logging:   enabled (%s)\n", path)
	} else {
		fmt.Fprintln(out, "  file logging:   disabled")
	}

	if cfg.SoundTracking != nil && cfg.SoundTracking.Enabled {
		trkPath := cfg.SoundTracking.DatabasePath
		if trkPath == "" {
			trkPath = "(default XDG path)"
		}
		fmt.Fprintf(out, "  tracking:       enabled (%s)\n", trkPath)
	} else {
		fmt.Fprintln(out, "  tracking:       disabled")
	}

	fmt.Fprintf(out, "  version:        %s\n", Version)

	slog.Debug("status reported", "enabled", cfg.Enabled, "volume", cfg.Volume)
	return nil
}

// loadConfigForStatus reports the configured file path (for display)
// and the loaded config. Search order matches LoadConfig: --config
// flag wins; else first XDG path that exists; else defaults.
func loadConfigForStatus(cmd *cobra.Command, cli *CLI) (string, *config.Config, error) {
	if flag, _ := cmd.Flags().GetString("config"); flag != "" {
		cfg, err := cli.configManager.LoadFromFile(flag)
		if err != nil {
			return flag, nil, fmt.Errorf("load --config %s: %w", flag, err)
		}
		return flag, cfg, nil
	}
	paths := config.NewXDGDirs().GetConfigPaths("config.json")
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			cfg, err := cli.configManager.LoadFromFile(p)
			if err != nil {
				return p, nil, fmt.Errorf("load %s: %w", p, err)
			}
			return p, cfg, nil
		}
	}
	return "(none - using defaults)", cli.configManager.GetDefaultConfig(), nil
}

// describeVolume returns a printable value and a source annotation
// (env / file / default) for the status report.
func describeVolume(cfg *config.Config) (string, string) {
	// If CLAUDIO_VOLUME is set in the environment, ApplyEnvironmentOverrides
	// already set cfg.Volume from it — annotate accordingly.
	if envVol := os.Getenv("CLAUDIO_VOLUME"); envVol != "" {
		if cfg.Volume != nil {
			return fmt.Sprintf("%.2f", *cfg.Volume), "from CLAUDIO_VOLUME"
		}
	}
	if cfg.Volume == nil {
		return "default", "no persisted setting"
	}
	return fmt.Sprintf("%.2f", *cfg.Volume), "from config.json"
}
