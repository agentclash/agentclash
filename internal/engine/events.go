package engine

import (
	"sync"
	"time"
)

// Event is a real-time race event pushed to the UI.
type Event struct {
	Type      string    `json:"type"`
	Agent     string    `json:"agent,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// EventBus broadcasts events to all subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[chan Event]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan Event]struct{}),
	}
}

func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 256)
	eb.mu.Lock()
	eb.subscribers[ch] = struct{}{}
	eb.mu.Unlock()
	return ch
}

func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	delete(eb.subscribers, ch)
	eb.mu.Unlock()
	close(ch)
}

func (eb *EventBus) Emit(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	for ch := range eb.subscribers {
		select {
		case ch <- e:
		default:
			// slow subscriber, drop event
		}
	}
}
