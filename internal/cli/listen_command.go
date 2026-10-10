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
	"time"

	"github.com/spf13/cobra"

	"claudio.click/internal/config"
	"claudio.click/internal/listen"
	"claudio.click/internal/sounds"
)

const (
	defaultListenAddr   = "127.0.0.1:19190"
	defaultListenMaxAge = 30 * time.Second
	// listenTokenEnv keeps the token off the command line, where other
	// users of the machine could read it.
	listenTokenEnv = "CLAUDIO_LISTEN_TOKEN"
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
    -H "Authorization: Bearer $CLAUDIO_LISTEN_TOKEN" \
    -d '{"source":"claude","category":"completion","hint":"agent-complete","operation":"stop"}'

Only "category" is required. The sound is chosen here, from this machine's
soundpack, volume and mute setting.

Without a token only a loopback address is allowed. Set the token with
--token or, better, the CLAUDIO_LISTEN_TOKEN environment variable.`,
		Args: cobra.NoArgs,
		RunE: c.runListen,
	}
	cmd.Flags().String("addr", defaultListenAddr, "Address to listen on")
	cmd.Flags().String("token", "", "Token every request must carry (default $"+listenTokenEnv+")")
	cmd.Flags().Duration("max-age", defaultListenMaxAge, "Drop events older than this; 0 plays every event")
	return cmd
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
	token, _ := cmd.Flags().GetString("token")
	maxAge, _ := cmd.Flags().GetDuration("max-age")
	if token == "" {
		token = os.Getenv(listenTokenEnv)
	}
	if token == "" && !isLoopbackAddr(addr) {
		return fmt.Errorf("listening on %s needs a token: set %s or pass --token", addr, listenTokenEnv)
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

	netListener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s: %w", addr, err)
	}
	server := listen.New(listen.Options{Token: token, MaxAge: maxAge}, func(event sounds.Event) {
		c.playEvent(event, cfg)
	})
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
