package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"claudio.click/internal/audio"
	"claudio.click/internal/config"
	"claudio.click/internal/tracking"
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
func newStatusCommand(c *CLI) *cobra.Command {
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
		RunE: c.runStatus,
	}
}

func (c *CLI) runStatus(cmd *cobra.Command, _ []string) error {
	// Resolve which config file (if any) actually exists on disk so
	// we can report its location. We don't use the writable path —
	// for status we want to show the first FOUND config, mirroring
	// the search order in LoadConfig.
	loaded := c.loadConfig(cmd)
	loaded.warnIgnored(cmd)
	configPathDisplay, cfg := describeConfigFile(loaded), loaded.Config

	// Apply env overrides so the report reflects runtime-effective values.
	cfg = c.configManager.ApplyEnvironmentOverrides(cfg)

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
	backend, backendErr := audio.ResolveBackend(cfg.AudioBackend)
	if backendErr != nil {
		fmt.Fprintf(out, "  audio backend:  %s -> %s (unavailable: %v)\n", cfg.AudioBackend, backend, backendErr)
	} else {
		fmt.Fprintf(out, "  audio backend:  %s -> %s (available; playback not tested)\n", cfg.AudioBackend, backend)
	}

	if cfg.FileLogging != nil && cfg.FileLogging.Enabled {
		path := c.configManager.ResolveLogFilePath(cfg.FileLogging.Filename)
		fmt.Fprintf(out, "  file logging:   enabled (%s)\n", path)
	} else {
		fmt.Fprintln(out, "  file logging:   disabled")
	}

	if cfg.SoundTracking != nil && cfg.SoundTracking.Enabled {
		trkPath := cfg.SoundTracking.DatabasePath
		if trkPath == "" {
			trkPath = tracking.DefaultDatabasePath()
		}
		fmt.Fprintf(out, "  tracking:       enabled (%s)\n", trkPath)
	} else {
		fmt.Fprintln(out, "  tracking:       disabled")
	}

	fmt.Fprintf(out, "  version:        %s\n", Version)

	slog.Debug("status reported", "enabled", cfg.Enabled, "volume", cfg.Volume)
	return nil
}

// describeConfigFile is the status line for which config file is in effect,
// matching what hook mode does with the same file.
func describeConfigFile(l configLoad) string {
	switch {
	case l.Path == "":
		return "(none - using defaults)"
	case l.Missing:
		return l.Path + " (not found - using defaults)"
	case l.Err != nil:
		return l.Path + " (invalid, ignored - using defaults)"
	default:
		return l.Path
	}
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
