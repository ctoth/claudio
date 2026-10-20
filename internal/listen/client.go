package listen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"claudio.click/internal/sounds"
)

// Post sends event as the body of one POST to url, used exactly as given,
// with token as a bearer token when there is one. url is whatever takes the
// event onward: a Server's /events, or a relay that something else reads
// from. Any answer outside 2xx is an error.
func Post(ctx context.Context, client *http.Client, url, token string, event sounds.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send event: %w", err)
	}
	defer response.Body.Close()
	// A refusal is worth one short line in the log; cap what a stranger at
	// that address could make this process read.
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 256))
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return fmt.Errorf("listener answered %d: %s", response.StatusCode, strings.TrimSpace(string(answer)))
	}
	return nil
}
