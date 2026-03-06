package telemetry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Collector gathers traces from all agents in a race.
type Collector struct {
	mu     sync.Mutex
	traces map[string]*Trace // agentName -> trace
	outDir string
}

func NewCollector(outDir string) *Collector {
	return &Collector{
		traces: make(map[string]*Trace),
		outDir: outDir,
	}
}

func (c *Collector) OutputDir() string { return c.outDir }

func (c *Collector) RegisterAgent(raceID, agentName, model string) *Trace {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := NewTrace(raceID, agentName, model)
	c.traces[agentName] = t
	return t
}

func (c *Collector) GetTrace(agentName string) *Trace {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.traces[agentName]
}

func (c *Collector) AllTraces() []*Trace {
	c.mu.Lock()
	defer c.mu.Unlock()
	all := make([]*Trace, 0, len(c.traces))
	for _, t := range c.traces {
		all = append(all, t)
	}
	return all
}

// SaveResults writes each agent's trace as a JSON file.
func (c *Collector) SaveResults() error {
	if err := os.MkdirAll(c.outDir, 0o755); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// Save individual traces
	for name, trace := range c.traces {
		data, err := json.MarshalIndent(trace, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal trace for %s: %w", name, err)
		}
		path := filepath.Join(c.outDir, name+".json")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return fmt.Errorf("write trace for %s: %w", name, err)
		}
	}

	// Save summary
	summary := make(map[string]any)
	for name, trace := range c.traces {
		summary[name] = map[string]any{
			"model":         trace.Model,
			"submitted":     trace.Submitted,
			"total_tokens":  trace.TotalTokens,
			"total_steps":   len(trace.Steps),
			"llm_calls":     trace.TotalLLMCalls,
			"tool_calls":    trace.TotalToolCalls,
			"failed_tools":  trace.FailedToolCalls,
			"unique_tools":  trace.UniqueToolsUsed,
		}
	}
	data, _ := json.MarshalIndent(summary, "", "  ")
	return os.WriteFile(filepath.Join(c.outDir, "summary.json"), data, 0o644)
}
