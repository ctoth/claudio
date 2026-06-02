package tracking

import (
	"context"

	"claudio.click/internal/hooks"
)

// Lookup is one candidate path the resolver considered for a single
// MapSound invocation. Sequence is 1-based and only comparable within
// a single ChainType (see review finding #20 / v2 schema migration).
type Lookup struct {
	Path     string
	Found    bool
	Sequence int
}

// EventRecorder records a complete tracking event in a single atomic
// transaction. Implementations are stateless and goroutine-safe (callers
// don't need to serialize). Errors are returned, not latched — best-effort
// callers (e.g. the sound mapper) log and continue.
//
// The caller passes the full resolved chain (lookups) and the winner
// (selectedPath) it already chose. The recorder does not infer event
// boundaries from successive calls — every RecordEvent call is one event.
type EventRecorder interface {
	RecordEvent(
		ctx context.Context,
		eventCtx *hooks.EventContext,
		chainType string,
		lookups []Lookup,
		selectedPath string,
	) error
}
