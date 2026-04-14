package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamSSEParsesEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher := w.(http.Flusher)

		fmt.Fprint(w, "id: 1\n")
		fmt.Fprint(w, "event: run_event\n")
		fmt.Fprint(w, "data: {\"type\":\"started\"}\n\n")
		flusher.Flush()

		fmt.Fprint(w, "id: 2\n")
		fmt.Fprint(w, "event: run_event\n")
		fmt.Fprint(w, "data: {\"type\":\"completed\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := client.StreamSSE(ctx, "/events", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var events []SSEEvent
	for event := range ch {
		events = append(events, event)
	}

	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if events[0].ID != "1" {
		t.Fatalf("event[0].ID = %q, want %q", events[0].ID, "1")
	}
	if events[0].Event != "run_event" {
		t.Fatalf("event[0].Event = %q, want %q", events[0].Event, "run_event")
	}
	if string(events[0].Data) != `{"type":"started"}` {
		t.Fatalf("event[0].Data = %q, want %q", string(events[0].Data), `{"type":"started"}`)
	}

	if events[1].ID != "2" {
		t.Fatalf("event[1].ID = %q, want %q", events[1].ID, "2")
	}
	if string(events[1].Data) != `{"type":"completed"}` {
		t.Fatalf("event[1].Data = %q, want %q", string(events[1].Data), `{"type":"completed"}`)
	}
}

func TestStreamSSEIgnoresComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		fmt.Fprint(w, ": this is a comment\n")
		fmt.Fprint(w, "id: 1\n")
		fmt.Fprint(w, "event: test\n")
		fmt.Fprint(w, "data: hello\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := client.StreamSSE(ctx, "/events", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var events []SSEEvent
	for event := range ch {
		events = append(events, event)
	}

	if len(events) != 1 {
		t.Fatalf("got %d events, want 1 (comments should be ignored)", len(events))
	}
	if string(events[0].Data) != "hello" {
		t.Fatalf("data = %q, want %q", string(events[0].Data), "hello")
	}
}

func TestStreamSSEHandlesMultilineData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		fmt.Fprint(w, "id: 1\n")
		fmt.Fprint(w, "event: msg\n")
		fmt.Fprint(w, "data: line1\n")
		fmt.Fprint(w, "data: line2\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := client.StreamSSE(ctx, "/events", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	event := <-ch
	if string(event.Data) != "line1\nline2" {
		t.Fatalf("data = %q, want %q", string(event.Data), "line1\nline2")
	}
}

func TestStreamSSEReturnsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"unauthorized","message":"bad token"}}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "bad-token")
	_, err := client.StreamSSE(context.Background(), "/events", nil)
	if err == nil {
		t.Fatal("expected error on 401, got nil")
	}
}

func TestStreamSSERespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)

		// Send one event then hold the connection open.
		fmt.Fprint(w, "id: 1\nevent: test\ndata: first\n\n")
		flusher.Flush()

		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "tok")
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := client.StreamSSE(ctx, "/events", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read first event.
	event := <-ch
	if string(event.Data) != "first" {
		t.Fatalf("data = %q, want %q", string(event.Data), "first")
	}

	// Cancel context — channel should close.
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to close after context cancel")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for channel to close")
	}
}
