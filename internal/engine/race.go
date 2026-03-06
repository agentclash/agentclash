package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/internal/engine/tools"
	"github.com/Atharva-Kanherkar/agentclash/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/internal/scoring"
	"github.com/Atharva-Kanherkar/agentclash/internal/telemetry"
)

// Contestant is one AI model in the race.
type Contestant struct {
	Name     string
	Provider provider.Provider
	Model    string
}

// RaceConfig controls the race parameters.
type RaceConfig struct {
	ChallengeDir      string
	ChallengeDesc     string // set by web server after reading file
	TimeLimit         time.Duration
	MaxIterations     int
	BroadcastInterval int
	Contestants       []Contestant
	OutputDir         string
	Events            *EventBus // optional: if set, events are emitted for the UI
}

// Race orchestrates a single competition between multiple AI agents.
type Race struct {
	id        string
	cfg       RaceConfig
	collector *telemetry.Collector
	logger    *slog.Logger
}

func NewRace(cfg RaceConfig, logger *slog.Logger) *Race {
	id := fmt.Sprintf("%d", time.Now().Unix())
	outDir := cfg.OutputDir
	if outDir == "" {
		outDir = filepath.Join("results", id)
	}
	return &Race{
		id:        id,
		cfg:       cfg,
		collector: telemetry.NewCollector(outDir),
		logger:    logger,
	}
}

func (r *Race) emit(e Event) {
	if r.cfg.Events != nil {
		r.cfg.Events.Emit(e)
	}
}

// Run executes the full race: provision -> run all agents -> score -> report.
func (r *Race) Run(ctx context.Context) error {
	// Read challenge description
	challengeDesc := r.cfg.ChallengeDesc
	if challengeDesc == "" {
		challengePath := filepath.Join(r.cfg.ChallengeDir, "challenge.yaml")
		data, err := os.ReadFile(challengePath)
		if err != nil {
			return fmt.Errorf("read challenge: %w", err)
		}
		challengeDesc = string(data)
	}

	r.logger.Info("race starting",
		"race_id", r.id,
		"contestants", len(r.cfg.Contestants),
		"time_limit", r.cfg.TimeLimit,
	)

	// Emit race_started
	names := make([]string, len(r.cfg.Contestants))
	models := make([]string, len(r.cfg.Contestants))
	for i, c := range r.cfg.Contestants {
		names[i] = c.Name
		models[i] = c.Model
	}
	r.emit(Event{Type: "race_started", Data: map[string]any{
		"race_id":    r.id,
		"agents":     names,
		"models":     models,
		"challenge":  challengeDesc,
		"time_limit": r.cfg.TimeLimit.Seconds(),
	}})

	// Set up race context with timeout
	raceCtx, cancel := context.WithTimeout(ctx, r.cfg.TimeLimit)
	defer cancel()

	broadcaster := NewBroadcaster(r.id, "race", r.cfg.TimeLimit)
	registry := tools.DefaultRegistry()

	// Build opponent lists for each contestant (everyone except themselves)
	opponentMap := make(map[string][]string)
	for _, c := range r.cfg.Contestants {
		for _, other := range r.cfg.Contestants {
			if other.Name != c.Name {
				opponentMap[c.Name] = append(opponentMap[c.Name],
					fmt.Sprintf("%s (%s)", other.Name, other.Model))
			}
		}
	}

	// Launch all agents in parallel
	var wg sync.WaitGroup
	for _, c := range r.cfg.Contestants {
		broadcaster.RegisterAgent(c.Name, c.Model)

		workDir, cpErr := r.copyWorkspace(c.Name)
		if cpErr != nil {
			return fmt.Errorf("copy workspace for %s: %w", c.Name, cpErr)
		}

		trace := r.collector.RegisterAgent(r.id, c.Name, c.Model)

		runner := NewRunner(RunnerConfig{
			Name:              c.Name,
			Model:             c.Model,
			Provider:          c.Provider,
			WorkspaceDir:      workDir,
			ChallengeDesc:     challengeDesc,
			MaxIterations:     r.cfg.MaxIterations,
			BroadcastInterval: r.cfg.BroadcastInterval,
			Opponents:         opponentMap[c.Name],
		}, registry, broadcaster, trace, r.logger, r.cfg.Events)

		wg.Add(1)
		go func(runner *Runner, name string) {
			defer wg.Done()
			if runErr := runner.Run(raceCtx); runErr != nil {
				r.logger.Error("agent error", "agent", name, "error", runErr)
			}
		}(runner, c.Name)
	}

	wg.Wait()
	r.logger.Info("all agents finished", "race_id", r.id)

	// Finalize traces
	for _, t := range r.collector.AllTraces() {
		t.Finish()
	}

	// Score and rank
	results := scoring.Rank(r.collector.AllTraces())
	scoring.PrintResults(results)

	// Emit results
	r.emit(Event{Type: "race_completed", Data: results})

	// Save traces to disk
	if err := r.collector.SaveResults(); err != nil {
		return fmt.Errorf("save results: %w", err)
	}
	r.logger.Info("results saved", "dir", r.collector.OutputDir())

	return nil
}

// copyWorkspace creates an isolated copy of the challenge workspace for one agent.
func (r *Race) copyWorkspace(agentName string) (string, error) {
	src := filepath.Join(r.cfg.ChallengeDir, "workspace")
	dst := filepath.Join(os.TempDir(), "agentclash", r.id, agentName)

	if err := os.MkdirAll(dst, 0o755); err != nil {
		return "", err
	}

	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})

	return dst, err
}
