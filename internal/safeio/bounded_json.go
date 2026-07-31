package safeio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// DefaultHookReadDeadline bounds how long ReadJSONBounded waits for a hook
// payload that never completes. Hook payloads arrive in one write from the
// spawning agent, so 2s is generous; the deadline exists only for the
// pathological pipe (see ReadJSONBounded).
const DefaultHookReadDeadline = 2 * time.Second

// ReadJSONBounded reads one JSON payload from r, returning as soon as any of
// these is reached:
//
//   - a complete JSON value has arrived (json.Valid),
//   - EOF,
//   - max bytes (error, as ReadAllCapped),
//   - deadline (returns whatever arrived, possibly nothing).
//
// It exists because a hook stdin pipe is not guaranteed to deliver EOF: under
// Windows git-bash a spawning agent (observed with Claude Code Stop hooks,
// 2026-07-31) can hold the write end open indefinitely — most reliably while
// an unrelated background child process holds an inherited handle to it. A
// plain ReadAllCapped then blocks forever and the agent reports itself stuck
// "running stop hooks". In that failure mode the payload has already fully
// arrived; only the EOF is missing — which is why a complete JSON value is
// treated as end-of-payload and the deadline is a last resort, not the normal
// exit path.
//
// The internal reader goroutine may outlive the call when the deadline fires
// (it stays blocked in r.Read). That is deliberate and safe for claudio's
// short-lived CLI process; do not reuse this for long-lived daemons without
// adding cancellation.
func ReadJSONBounded(r io.Reader, max int64, deadline time.Duration, kind string) ([]byte, error) {
	type chunk struct {
		data []byte
		err  error
	}
	// Buffered so an abandoned goroutine's final send never blocks it forever
	// holding buffers; a second send after abandonment parks the goroutine,
	// which process exit reaps.
	ch := make(chan chunk, 1)
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			c := chunk{}
			if n > 0 {
				c.data = append([]byte(nil), buf[:n]...)
			}
			c.err = err
			ch <- c
			if err != nil {
				return
			}
		}
	}()

	timer := time.NewTimer(deadline)
	defer timer.Stop()
	var out []byte
	for {
		select {
		case c := <-ch:
			out = append(out, c.data...)
			if int64(len(out)) > max {
				return nil, fmt.Errorf("%s exceeds %d byte limit", kind, max)
			}
			if trimmed := bytes.TrimSpace(out); len(trimmed) > 0 && json.Valid(trimmed) {
				return out, nil
			}
			if c.err != nil {
				if c.err == io.EOF {
					return out, nil
				}
				return nil, fmt.Errorf("read %s: %w", kind, c.err)
			}
		case <-timer.C:
			// Pipe held open with an incomplete (or absent) payload. Return
			// what arrived: the caller treats empty/garbage input as
			// config-test mode / a parse error, both of which exit cleanly —
			// unlike blocking here, which wedges the spawning agent.
			return out, nil
		}
	}
}
