package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
	"claudio.click/internal/listen"
	"claudio.click/internal/sounds"
)

const (
	defaultListenAddr   = "127.0.0.1:19190"
	defaultListenMaxAge = 30 * time.Second
	// listenShutdownGrace bounds how long stopping waits for open requests.
	listenShutdownGrace = 5 * time.Second
)

// newListenCommand returns the `claudio listen` subcommand: a long-running
// HTTP listener that plays the sound for each event posted to it.
func newListenCommand(c *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listen",
		Short: "Play sounds for events sent from other machines",
		Long: `Listen for sound events over HTTP and play them on this machine.

Run this where the speakers are. Whatever sees the event (a coding agent on
another machine, in a container or in the cloud) sends one JSON event per
request:

  curl -X POST http://127.0.0.1:19190/events \
    -H "Authorization: Bearer <token>" \
    -d '{"source":"claude","category":"completion","hint":"agent-complete","operation":"stop"}'

Only "category" is required. The sound is chosen here, from this machine's
soundpack, volume and mute setting.

Without a token only a loopback address is allowed. Other users of this
machine can read a command line, so prefer --token-file to --token.

With --stdin no web server is started. Events are read from stdin, one JSON
event per line, until it ends, so anything that can be piped can carry them:

  curl -sN https://relay.example/my-stream | claudio listen --stdin`,
		Args: cobra.NoArgs,
		RunE: c.runListen,
	}
	cmd.Flags().String("addr", defaultListenAddr, "Address to listen on")
	cmd.Flags().String("token", "", "Token every request must carry")
	cmd.Flags().String("token-file", "", "File holding the token every request must carry")
	cmd.Flags().Duration("max-age", defaultListenMaxAge, "Drop events older than this; 0 plays every event")
	cmd.Flags().Bool("stdin", false, "Read events from stdin, one JSON event per line, instead of serving HTTP")
	cmd.MarkFlagsMutuallyExclusive("token", "token-file")
	// An address and a token belong to the web server that --stdin replaces.
	cmd.MarkFlagsMutuallyExclusive("stdin", "addr")
	cmd.MarkFlagsMutuallyExclusive("stdin", "token")
	cmd.MarkFlagsMutuallyExclusive("stdin", "token-file")
	return cmd
}

// listenToken returns the token from --token or --token-file, or "" when
// neither is given. A token file that cannot be read or holds no token is
// an error, never "no token".
func listenToken(cmd *cobra.Command) (string, error) {
	token, _ := cmd.Flags().GetString("token")
	path, _ := cmd.Flags().GetString("token-file")
	if path == "" {
		return token, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot read token file: %w", err)
	}
	token = strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("token file %s is empty", path)
	}
	return token, nil
}

// isLoopbackAddr reports whether a host:port address is reachable only from
// this machine.
func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func (c *CLI) runListen(cmd *cobra.Command, _ []string) error {
	addr, _ := cmd.Flags().GetString("addr")
	maxAge, _ := cmd.Flags().GetDuration("max-age")
	fromStdin, _ := cmd.Flags().GetBool("stdin")
	token, err := listenToken(cmd)
	if err != nil {
		return err
	}
	if !fromStdin && token == "" && !isLoopbackAddr(addr) {
		return fmt.Errorf("listening on %s needs a token: pass --token-file or --token", addr)
	}
	if maxAge < 0 {
		return fmt.Errorf("invalid --max-age %s: must not be negative", maxAge)
	}

	cfg, err := c.loadAndValidateConfig(cmd)
	if err != nil {
		return err
	}
	setupLogging(cfg, cmd.ErrOrStderr())
	c.initializeTracking(cfg)
	if err := c.initializeAudioSystem(cfg); err != nil {
		return err
	}

	server := listen.New(listen.Options{Token: token, MaxAge: maxAge}, func(event sounds.Event) {
		c.playEvent(event, cfg)
	})

	if fromStdin {
		// The pipe is the whole transport: play until it ends. A read error
		// fails the command so whatever supervises it can reconnect.
		slog.Info("playing sound events from stdin", "max_age", maxAge)
		err := server.ReadEvents(cmd.InOrStdin())
		server.Wait()
		return err
	}

	netListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	httpServer := &http.Server{Handler: server, ReadHeaderTimeout: 10 * time.Second}

	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	slog.Info("listening for sound events", "addr", netListener.Addr().String(), "token_required", token != "", "max_age", maxAge)
	fmt.Fprintf(cmd.OutOrStdout(), "listening on http://%s\n", netListener.Addr())

	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(netListener) }()

	select {
	case err := <-serveErr:
		return fmt.Errorf("listener stopped: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), listenShutdownGrace)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Warn("listener did not shut down cleanly", "error", err)
	}
	server.Wait()
	slog.Info("listener stopped")
	return nil
}

// playEvent plays the sound for one received event.
func (c *CLI) playEvent(event sounds.Event, cfg *config.Config) {
	eventCtx, err := event.Context()
	if err != nil {
		// The listener has already refused events without a known category.
		slog.Warn("event not played", "source", event.Source, "id", event.ID, "error", err)
		return
	}
	c.playEventContext(eventCtx, event.Session, cfg)
}
