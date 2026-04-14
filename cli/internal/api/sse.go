package api

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SSEEvent represents a Server-Sent Event.
type SSEEvent struct {
	ID    string
	Event string
	Data  []byte
}

// StreamSSE opens an SSE connection and sends events to a channel.
// The channel is closed when the connection ends or the context is cancelled.
func (c *Client) StreamSSE(ctx context.Context, path string, query url.Values) (<-chan SSEEvent, error) {
	fullURL := c.baseURL + path
	if len(query) > 0 {
		fullURL += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	c.setAuth(req)

	// Use a client without timeout for streaming.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE connection failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body := make([]byte, 4096)
		n, _ := resp.Body.Read(body)
		return nil, fmt.Errorf("SSE error (HTTP %d): %s", resp.StatusCode, string(body[:n]))
	}

	ch := make(chan SSEEvent, 64)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var event SSEEvent
		var dataLines []string

		for scanner.Scan() {
			line := scanner.Text()

			if line == "" {
				// Empty line = event boundary.
				if len(dataLines) > 0 {
					event.Data = []byte(strings.Join(dataLines, "\n"))
					select {
					case ch <- event:
					case <-ctx.Done():
						return
					}
					event = SSEEvent{}
					dataLines = nil
				}
				continue
			}

			if strings.HasPrefix(line, "id: ") || strings.HasPrefix(line, "id:") {
				event.ID = strings.TrimPrefix(line, "id: ")
				event.ID = strings.TrimPrefix(event.ID, "id:")
				event.ID = strings.TrimSpace(event.ID)
			} else if strings.HasPrefix(line, "event: ") || strings.HasPrefix(line, "event:") {
				event.Event = strings.TrimPrefix(line, "event: ")
				event.Event = strings.TrimPrefix(event.Event, "event:")
				event.Event = strings.TrimSpace(event.Event)
			} else if strings.HasPrefix(line, "data: ") || strings.HasPrefix(line, "data:") {
				data := strings.TrimPrefix(line, "data: ")
				data = strings.TrimPrefix(data, "data:")
				dataLines = append(dataLines, data)
			}
			// Ignore comments (lines starting with :) and unknown fields.
		}
	}()

	return ch, nil
}
