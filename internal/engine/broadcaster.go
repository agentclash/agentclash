package engine

import (
	"fmt"
	"sync"
	"time"
)

// AgentStatus is a snapshot of one agent's progress.
type AgentStatus struct {
	Name       string
	Model      string
	StepCount  int
	LastAction string
	Finished   bool
	FinishedAt *time.Time
}

// Broadcaster manages the shared race state that gets injected into
// each agent's context so they know where they stand.
type Broadcaster struct {
	mu        sync.RWMutex
	raceID    string
	challenge string
	startedAt time.Time
	timeLimit time.Duration
	standings []AgentStatus
}

func NewBroadcaster(raceID, challenge string, timeLimit time.Duration) *Broadcaster {
	return &Broadcaster{
		raceID:    raceID,
		challenge: challenge,
		startedAt: time.Now(),
		timeLimit: timeLimit,
	}
}

func (b *Broadcaster) RegisterAgent(name, model string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.standings = append(b.standings, AgentStatus{Name: name, Model: model})
}

func (b *Broadcaster) UpdateAgent(name string, stepCount int, lastAction string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i := range b.standings {
		if b.standings[i].Name == name {
			b.standings[i].StepCount = stepCount
			b.standings[i].LastAction = lastAction
			return
		}
	}
}

func (b *Broadcaster) MarkFinished(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	for i := range b.standings {
		if b.standings[i].Name == name {
			b.standings[i].Finished = true
			b.standings[i].FinishedAt = &now
			return
		}
	}
}

// GetStandings builds the race update string injected into an agent's context.
// excludeAgent is the agent receiving the update (marked with →).
func (b *Broadcaster) GetStandings(excludeAgent string) string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	elapsed := time.Since(b.startedAt).Round(time.Second)
	remaining := (b.timeLimit - time.Since(b.startedAt)).Round(time.Second)
	if remaining < 0 {
		remaining = 0
	}

	s := fmt.Sprintf("[RACE UPDATE — %s elapsed, %s remaining]\n\nSTANDINGS:\n", elapsed, remaining)

	for i, a := range b.standings {
		marker := "  "
		if a.Name == excludeAgent {
			marker = "> "
		}
		status := a.LastAction
		if status == "" {
			status = "starting..."
		}
		if a.Finished {
			status = "FINISHED"
		}
		s += fmt.Sprintf("%s%d. %-20s  step %-3d  %s\n", marker, i+1, a.Name, a.StepCount, status)
	}

	return s
}

func (b *Broadcaster) Elapsed() time.Duration {
	return time.Since(b.startedAt)
}
