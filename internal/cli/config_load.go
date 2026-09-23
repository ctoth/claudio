package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
)

// configLoad is the outcome of resolving and reading the active config
// file. Every command loads through loadConfig so they agree on what
// "missing" and "broken" mean:
//
//   - no file (or an empty one): defaults, quietly;
//   - a file that cannot be read, parsed or validated: Err is set and
//     Config holds the defaults.
//
// What a command does with Err is its own policy: read-only surfaces
// (hook mode, analyze, status) call warnIgnored and carry on, the same way
// a hook must never fail over a config typo; commands that write the file
// back return Err so they never overwrite a file the user may still want.
type configLoad struct {
	Path    string // file consulted; "" when no config file exists
	Missing bool   // Path was named by --config but does not exist
	Config  *config.Config
	Err     error
}

// loadConfig resolves --config or, without it, the first existing XDG
// config file, and reads it with the shared missing/broken policy.
func (c *CLI) loadConfig(cmd *cobra.Command) configLoad {
	path, _ := cmd.Flags().GetString("config")
	if path == "" {
		path = c.configManager.FindConfigFile()
	}
	if path == "" {
		slog.Debug("no config file found; using defaults")
		return configLoad{Config: c.configManager.GetDefaultConfig()}
	}

	cfg, err := c.configManager.LoadFromFile(path)
	switch {
	case err == nil:
		return configLoad{Path: path, Config: cfg}
	case errors.Is(err, fs.ErrNotExist):
		slog.Debug("config file does not exist; using defaults", "path", path)
		return configLoad{Path: path, Missing: true, Config: c.configManager.GetDefaultConfig()}
	default:
		return configLoad{Path: path, Config: c.configManager.GetDefaultConfig(), Err: fmt.Errorf("config %s: %w", path, err)}
	}
}

// warnIgnored surfaces an unusable config once: a single warning line on
// stderr plus a log record. It is a no-op when the config loaded fine.
func (l configLoad) warnIgnored(cmd *cobra.Command) {
	if l.Err == nil {
		return
	}
	cmd.PrintErrf("Warning: ignoring %v; using defaults\n", l.Err)
	slog.Warn("ignoring unusable config file; using defaults", "path", l.Path, "error", l.Err)
}
