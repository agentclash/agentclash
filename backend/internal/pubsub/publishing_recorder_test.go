package pubsub

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/runevents"
	"github.com/google/uuid"
)

type fakeRecorder struct {
	called     bool
	returnErr  error
	returnEvent repository.RunEvent
}

func (f *fakeRecorder) RecordRunEvent(_ context.Context, params repository.RecordRunEventParams) (repository.RunEvent, error) {
	f.called = true
	if f.returnErr != nil {
		return repository.RunEvent{}, f.returnErr
	}
	return f.returnEvent, nil
}

type fakePublisher struct {
	called    bool
	lastRunID uuid.UUID
	lastEvent runevents.Envelope
	returnErr error
}

func (f *fakePublisher) PublishRunEvent(_ context.Context, runID uuid.UUID, event runevents.Envelope) error {
	f.called = true
	f.lastRunID = runID
	f.lastEvent = event
	return f.returnErr
}

func (f *fakePublisher) Close() error { return nil }

func TestPublishingRecorder_PublishesAfterRecord(t *testing.T) {
	runID := uuid.New()
	runAgentID := uuid.New()

	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{
			ID:             1,
			RunID:          runID,
			RunAgentID:     runAgentID,
			SequenceNumber: 42,
		},
	}
	pub := &fakePublisher{}
	recorder := NewPublishingRecorder(inner, pub, slog.Default())

	params := repository.RecordRunEventParams{
		Event: runevents.Envelope{
			RunID:      runID,
			RunAgentID: runAgentID,
			EventType:  "system.run.started",
		},
	}

	event, err := recorder.RecordRunEvent(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.SequenceNumber != 42 {
		t.Errorf("expected sequence 42, got %d", event.SequenceNumber)
	}
	if !inner.called {
		t.Error("inner recorder was not called")
	}
	if !pub.called {
		t.Error("publisher was not called")
	}
	if pub.lastRunID != runID {
		t.Errorf("publisher received wrong runID: %v", pub.lastRunID)
	}
	if pub.lastEvent.SequenceNumber != 42 {
		t.Errorf("published event should have sequence 42, got %d", pub.lastEvent.SequenceNumber)
	}
}

func TestPublishingRecorder_RecordErrorSkipsPublish(t *testing.T) {
	inner := &fakeRecorder{returnErr: errors.New("db error")}
	pub := &fakePublisher{}
	recorder := NewPublishingRecorder(inner, pub, slog.Default())

	params := repository.RecordRunEventParams{
		Event: runevents.Envelope{RunID: uuid.New()},
	}

	_, err := recorder.RecordRunEvent(context.Background(), params)
	if err == nil {
		t.Fatal("expected error from inner recorder")
	}
	if pub.called {
		t.Error("publisher should not be called when recording fails")
	}
}

func TestPublishingRecorder_PublishErrorSwallowed(t *testing.T) {
	runID := uuid.New()
	inner := &fakeRecorder{
		returnEvent: repository.RunEvent{
			RunID:          runID,
			SequenceNumber: 1,
		},
	}
	pub := &fakePublisher{returnErr: errors.New("redis down")}
	recorder := NewPublishingRecorder(inner, pub, slog.Default())

	params := repository.RecordRunEventParams{
		Event: runevents.Envelope{RunID: runID},
	}

	event, err := recorder.RecordRunEvent(context.Background(), params)
	if err != nil {
		t.Fatalf("publish error should be swallowed, got: %v", err)
	}
	if event.SequenceNumber != 1 {
		t.Errorf("expected sequence 1, got %d", event.SequenceNumber)
	}
	if !pub.called {
		t.Error("publisher should have been called even though it errored")
	}
}

func TestNoopPublisher(t *testing.T) {
	pub := NoopPublisher{}
	err := pub.PublishRunEvent(context.Background(), uuid.New(), runevents.Envelope{})
	if err != nil {
		t.Errorf("NoopPublisher should not error: %v", err)
	}
	if err := pub.Close(); err != nil {
		t.Errorf("NoopPublisher.Close should not error: %v", err)
	}
}

func TestNoopSubscriber(t *testing.T) {
	sub := NoopSubscriber{}
	ch, err := sub.Subscribe(context.Background(), uuid.New())
	if err != nil {
		t.Errorf("NoopSubscriber should not error: %v", err)
	}
	// Channel should be closed immediately.
	if _, ok := <-ch; ok {
		t.Error("NoopSubscriber channel should be closed")
	}
	if err := sub.Close(); err != nil {
		t.Errorf("NoopSubscriber.Close should not error: %v", err)
	}
}

func TestChannelName(t *testing.T) {
	id := uuid.MustParse("12345678-1234-1234-1234-123456789abc")
	name := ChannelName(id)
	expected := "run:12345678-1234-1234-1234-123456789abc:events"
	if name != expected {
		t.Errorf("ChannelName = %q, want %q", name, expected)
	}
}
