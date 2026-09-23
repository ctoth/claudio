package cli

import (
	"fmt"
	"log/slog"
	"math"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// newVolumeCommand returns the `claudio volume [LEVEL]` subcommand.
//
// Zero args  : prints the current persisted volume (or "default" if unset).
// One arg    : parses as float64 in [0.0, 1.0], persists to config.json
//
//	using the atomic write primitives + advisory lock.
//
// Note: the persistent `--volume` flag is for transient overrides on
// the hook/stdin path; this subcommand persists the value.
// CLAUDIO_VOLUME env var still takes precedence at runtime.
func newVolumeCommand(c *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "volume [LEVEL]",
		Short: "Get or set the persistent volume preference",
		Long: `Get or set the persistent volume preference in config.json.

With no argument: prints the current configured volume.
With a single argument (0.0 to 1.0): persists that volume to config.json.

Note: the CLAUDIO_VOLUME environment variable, if set, still takes
precedence at runtime over the persisted value. The transient
"--volume" flag also overrides the persisted value for a single hook
invocation.`,
		Args: cobra.MaximumNArgs(1),
		RunE: c.runVolume,
	}
}

func (c *CLI) runVolume(cmd *cobra.Command, args []string) error {
	// Read-only path: print and return.
	if len(args) == 0 {
		cfg, err := c.loadConfigForVerb(cmd)
		if err != nil {
			return err
		}
		// Apply env overrides so the reported value matches what hook
		// invocations actually use, and what `claudio status` reports.
		// WRITE path below intentionally does NOT do this — persistence
		// must be deterministic regardless of env state.
		cfg = c.configManager.ApplyEnvironmentOverrides(cfg)
		if cfg.Volume == nil {
			fmt.Fprintln(cmd.OutOrStdout(), "volume: default (no persisted setting)")
		} else if os.Getenv("CLAUDIO_VOLUME") != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "volume: %.2f (from CLAUDIO_VOLUME)\n", *cfg.Volume)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "volume: %.2f\n", *cfg.Volume)
		}
		return nil
	}

	// Write path.
	v, err := strconv.ParseFloat(args[0], 64)
	if err != nil {
		return fmt.Errorf("invalid volume %q: must be a float between 0.0 and 1.0", args[0])
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Errorf("invalid volume %q: must be a finite float between 0.0 and 1.0", args[0])
	}
	if v < 0.0 || v > 1.0 {
		return fmt.Errorf("volume must be between 0.0 and 1.0, got %f", v)
	}

	previous := "default"
	if err := c.mutateConfigForCommand(cmd, func(cfg *config.Config) error {
		if cfg.Volume != nil {
			previous = fmt.Sprintf("%.2f", *cfg.Volume)
		}
		cfg.Volume = &v
		return nil
	}); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "volume: %s -> %.2f\n", previous, v)
	slog.Info("volume persisted", "previous", previous, "value", v)
	return nil
}
