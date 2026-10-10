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

// Post sends event to the listener at baseURL (its address without
// "/events"), with token when the listener requires one. Anything but an
// accepted event is an error.
func Post(ctx context.Context, client *http.Client, baseURL, token string, event sounds.Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode event: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/events", bytes.NewReader(body))
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
	// The listener's answers are one short line; cap what a stranger at
	// that address could make this process read.
	answer, _ := io.ReadAll(io.LimitReader(response.Body, 256))
	if response.StatusCode != http.StatusAccepted {
		return fmt.Errorf("listener answered %d: %s", response.StatusCode, strings.TrimSpace(string(answer)))
	}
	return nil
}
