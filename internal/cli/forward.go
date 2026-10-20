package cli

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"claudio.click/internal/config"
	"claudio.click/internal/hooks"
	"claudio.click/internal/listen"
	"claudio.click/internal/sounds"
)

// forwardTimeout bounds one send. The hook has already returned to the
// agent by then; this only stops a worker waiting on a dead listener.
const forwardTimeout = 5 * time.Second

// forwardHookEvent sends the event to the configured listener instead of
// playing it here. Only the names a sound is chosen from leave the machine.
// A listener that is down or refuses is logged, never an error: the sound
// is lost, the agent's hook is not.
func forwardHookEvent(hookEvent *hooks.HookEvent, cfg *config.Config) {
	if !cfg.Enabled {
		slog.Debug("audio disabled, not forwarding the event")
		return
	}
	eventCtx := hookEvent.GetContext()
	if eventCtx.Category == hooks.Silent {
		slog.Debug("silent event, not forwarding it", "operation", eventCtx.Operation)
		return
	}

	event := sounds.EventFromContext(eventCtx)
	event.Source = hookEvent.Agent()
	event.Session = hookEvent.SessionID
	event.Time = time.Now().UTC()

	ctx, cancel := context.WithTimeout(context.Background(), forwardTimeout)
	defer cancel()
	target := redactedURL(cfg.ForwardURL())
	if err := listen.Post(ctx, http.DefaultClient, cfg.ForwardURL(), cfg.Forward.Token, event); err != nil {
		slog.Warn("event not forwarded", "url", target, "category", event.Category, "hint", event.Hint, "error", err)
		return
	}
	slog.Debug("event forwarded", "url", target, "category", event.Category, "hint", event.Hint)
}

// redactedURL is raw without any password it carries, for logs and status.
func redactedURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "(unparseable url)"
	}
	return parsed.Redacted()
}
