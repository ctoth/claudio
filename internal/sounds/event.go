package sounds

import (
	"time"

	"claudio.click/internal/hooks"
)

// Event is what one machine sends another to have a sound played: the
// fields the fallback chains read, plus where the event came from. It
// carries no prompt, tool input or tool output.
//
// The JSON keys are the wire contract with every emitter.
type Event struct {
	// Source names what produced the event ("claude", "github").
	Source string `json:"source,omitempty"`
	// ID identifies the event at its source.
	ID string `json:"id,omitempty"`
	// Time is when the event happened. A listener drops an event that is
	// too old to be worth hearing; the zero time is never too old.
	Time time.Time `json:"time,omitzero"`
	// Session groups events in the tracking database.
	Session string `json:"session,omitempty"`

	// Category is an hooks.EventCategory name ("loading", "success", ...)
	// and the only required field.
	Category     string `json:"category"`
	Hint         string `json:"hint,omitempty"`
	Command      string `json:"command,omitempty"`
	Subcommand   string `json:"subcommand,omitempty"`
	Phase        string `json:"phase,omitempty"`
	OriginalTool string `json:"original_tool,omitempty"`
	Operation    string `json:"operation,omitempty"`

	// Attributes carries source-specific values (an amount, a count). The
	// mapper does not read them.
	Attributes map[string]any `json:"attributes,omitempty"`
}

// EventFromContext returns the event that maps to the same sound as eventCtx.
func EventFromContext(eventCtx *hooks.EventContext) Event {
	command, subcommand := commandOf(eventCtx)
	// Only an event about a command has a phase; a tool-less event's
	// category implies none.
	phase := eventCtx.Phase
	if command != "" {
		phase = phaseOf(eventCtx)
	}
	return Event{
		Category:     eventCtx.Category.String(),
		Hint:         eventCtx.SoundHint,
		Command:      command,
		Subcommand:   subcommand,
		Phase:        phase,
		OriginalTool: eventCtx.OriginalTool,
		Operation:    eventCtx.Operation,
	}
}

// Context returns the context the mapper and the tracking database read.
// An unknown category is an error.
func (e Event) Context() (*hooks.EventContext, error) {
	category, err := hooks.ParseEventCategory(e.Category)
	if err != nil {
		return nil, err
	}
	return &hooks.EventContext{
		Category:     category,
		ToolName:     e.Command,
		OriginalTool: e.OriginalTool,
		IsSuccess:    e.Phase == "success",
		HasError:     category == hooks.Error,
		SoundHint:    e.Hint,
		Operation:    e.Operation,
		Command:      e.Command,
		Subcommand:   e.Subcommand,
		Phase:        e.Phase,
	}, nil
}
