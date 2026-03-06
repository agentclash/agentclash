package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/internal/engine/tools"
	"github.com/Atharva-Kanherkar/agentclash/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/internal/telemetry"
)

// RunnerConfig controls how a single agent runs.
type RunnerConfig struct {
	Name              string
	Model             string
	Provider          provider.Provider
	WorkspaceDir      string
	ChallengeDesc     string
	MaxIterations     int
	BroadcastInterval int
	Opponents         []string // "Name (model)" for each opponent
}

// Runner executes the think->act->observe loop for one agent.
type Runner struct {
	cfg         RunnerConfig
	registry    *tools.Registry
	trace       *telemetry.Trace
	broadcaster *Broadcaster
	logger      *slog.Logger
	events      *EventBus
	messages    []provider.Message
	stepCount   int
}

func NewRunner(
	cfg RunnerConfig,
	registry *tools.Registry,
	broadcaster *Broadcaster,
	trace *telemetry.Trace,
	logger *slog.Logger,
	events *EventBus,
) *Runner {
	return &Runner{
		cfg:         cfg,
		registry:    registry,
		trace:       trace,
		broadcaster: broadcaster,
		logger:      logger,
		events:      events,
	}
}

func (r *Runner) emit(e Event) {
	if r.events != nil {
		e.Agent = r.cfg.Name
		r.events.Emit(e)
	}
}

// Run executes the agent loop until it submits, times out, or hits max iterations.
func (r *Runner) Run(ctx context.Context) error {
	r.messages = []provider.Message{
		{Role: "system", Content: r.buildSystemPrompt()},
		{Role: "user", Content: r.cfg.ChallengeDesc},
	}

	r.emit(Event{Type: "agent_started", Data: map[string]any{
		"model": r.cfg.Model,
	}})

	for i := 0; i < r.cfg.MaxIterations; i++ {
		if err := ctx.Err(); err != nil {
			r.logger.Info("agent timed out", "agent", r.cfg.Name, "step", i)
			r.trace.Error = "timed out"
			r.emit(Event{Type: "agent_error", Data: map[string]any{"error": "timed out"}})
			return nil
		}

		// Inject race state periodically
		if r.cfg.BroadcastInterval > 0 && i > 0 && i%r.cfg.BroadcastInterval == 0 {
			standings := r.broadcaster.GetStandings(r.cfg.Name)
			r.messages = append(r.messages, provider.Message{
				Role:    "user",
				Content: standings,
			})
			r.trace.AddStep(telemetry.Step{
				Type:              telemetry.StepObserve,
				Timestamp:         time.Now(),
				Observation:       standings,
				RaceStateInjected: true,
			})
			r.stepCount++
			r.emit(Event{Type: "agent_observed", Data: map[string]any{
				"standings": standings,
				"step":      r.stepCount,
			}})
		}

		// THINK: call LLM (with 2 minute per-call timeout)
		r.emit(Event{Type: "agent_thinking", Data: map[string]any{
			"step": r.stepCount,
		}})

		thinkStart := time.Now()
		callCtx, callCancel := context.WithTimeout(ctx, 2*time.Minute)
		resp, err := r.cfg.Provider.ChatCompletion(callCtx, &provider.ChatRequest{
			Model:       r.cfg.Model,
			Messages:    r.messages,
			Tools:       r.registry.Defs(),
			MaxTokens:   4096,
			Temperature: 0.2,
		})
		callCancel()
		if err != nil {
			r.logger.Error("llm call failed", "agent", r.cfg.Name, "error", err)
			r.trace.Error = fmt.Sprintf("llm call failed: %v", err)
			r.emit(Event{Type: "agent_error", Data: map[string]any{"error": err.Error()}})
			return nil
		}

		thinkDur := time.Since(thinkStart)
		r.trace.AddStep(telemetry.Step{
			Type:        telemetry.StepThink,
			Timestamp:   thinkStart,
			Duration:    thinkDur,
			LLMResponse: resp.Content,
			TokensUsed:  resp.Usage.TotalTokens,
		})
		r.stepCount++

		// Collect planned tool names for the UI
		plannedTools := make([]string, len(resp.ToolCalls))
		for ti, tc := range resp.ToolCalls {
			plannedTools[ti] = tc.Name
		}

		r.emit(Event{Type: "agent_thought", Data: map[string]any{
			"step":          r.stepCount,
			"tokens":        resp.Usage.TotalTokens,
			"duration_ms":   thinkDur.Milliseconds(),
			"tool_calls":    len(resp.ToolCalls),
			"planned_tools": plannedTools,
			"content":       truncate(resp.Content, 500),
		}})

		r.logger.Info("agent thinking",
			"agent", r.cfg.Name,
			"step", i,
			"tokens", resp.Usage.TotalTokens,
			"tool_calls", len(resp.ToolCalls),
		)

		// No tool calls — model responded with text only
		if len(resp.ToolCalls) == 0 {
			r.messages = append(r.messages, provider.Message{
				Role:    "assistant",
				Content: resp.Content,
			})
			if resp.StopReason == "stop" {
				r.broadcaster.MarkFinished(r.cfg.Name)
				r.emit(Event{Type: "agent_finished", Data: map[string]any{
					"submitted": false,
					"reason":    "stop",
				}})
				return nil
			}
			continue
		}

		// Add assistant message with tool calls
		r.messages = append(r.messages, provider.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// ACT: execute each tool call
		for _, tc := range resp.ToolCalls {
			result := r.registry.Execute(r.cfg.WorkspaceDir, tc.Name, tc.Arguments)

			r.trace.AddStep(telemetry.Step{
				Type:       telemetry.StepAct,
				Timestamp:  time.Now(),
				Duration:   result.Duration,
				ToolName:   tc.Name,
				ToolInput:  tc.Arguments,
				ToolOutput: result.Output,
				ToolError:  result.Error,
				Success:    result.Success,
			})
			r.stepCount++

			r.broadcaster.UpdateAgent(r.cfg.Name, r.stepCount, tc.Name)

			r.emit(Event{Type: "agent_acted", Data: map[string]any{
				"step":        r.stepCount,
				"tool":        tc.Name,
				"input":       truncate(tc.Arguments, 500),
				"category":    string(result.Category),
				"success":     result.Success,
				"duration_ms": result.Duration.Milliseconds(),
				"output":      truncate(result.Output, 1000),
				"error":       result.Error,
			}})

			r.logger.Info("agent acted",
				"agent", r.cfg.Name,
				"tool", tc.Name,
				"category", result.Category,
				"success", result.Success,
			)

			// Send tool result back to conversation
			toolOutput := result.Output
			if result.Error != "" && toolOutput == "" {
				toolOutput = "Error: " + result.Error
			}
			r.messages = append(r.messages, provider.Message{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})

			// Check for solution submission
			if tc.Name == "submit_solution" && result.Success {
				r.trace.Submitted = true
				r.trace.SubmitExplanation = result.Output
				r.broadcaster.MarkFinished(r.cfg.Name)
				r.emit(Event{Type: "agent_finished", Data: map[string]any{
					"submitted":   true,
					"explanation": result.Output,
				}})
				return nil
			}
		}
	}

	r.logger.Info("agent hit max iterations", "agent", r.cfg.Name)
	r.trace.Error = "max iterations reached"
	r.emit(Event{Type: "agent_error", Data: map[string]any{"error": "max iterations reached"}})
	return nil
}

func (r *Runner) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString("You are ")
	sb.WriteString(r.cfg.Name)
	sb.WriteString(", competing in AgentClash — a real-time AI race.\n\n")

	sb.WriteString("RULES:\n")
	sb.WriteString("- You are racing against other AI models to solve the same challenge\n")
	sb.WriteString("- You will periodically receive RACE UPDATES showing standings\n")
	sb.WriteString("- Be fast AND accurate — both speed and correctness matter\n")
	sb.WriteString("- When done, call submit_solution immediately\n\n")

	sb.WriteString("TOOL STRATEGY:\n")
	sb.WriteString("- Prefer structured tools (read_file, search_text, build, run_tests) over bash\n")
	sb.WriteString("- Using structured tools scores higher than using bash for the same operation\n")
	sb.WriteString("- The bash tool is available as an escape hatch for things structured tools can't do\n\n")

	sb.WriteString("AVAILABLE STRUCTURED TOOLS:\n")
	sb.WriteString("- read_file: read a file\n")
	sb.WriteString("- write_file: create or overwrite a file\n")
	sb.WriteString("- list_files: list directory contents\n")
	sb.WriteString("- search_text: search for text patterns across files (like grep)\n")
	sb.WriteString("- search_files: find files by name pattern (like find)\n")
	sb.WriteString("- build: compile the project\n")
	sb.WriteString("- run_tests: run the test suite\n")
	sb.WriteString("- bash: escape hatch — run any shell command (scores lower)\n")
	sb.WriteString("- submit_solution: declare you're done\n\n")

	if len(r.cfg.Opponents) > 0 {
		sb.WriteString("YOUR OPPONENTS (racing right now):\n")
		for _, opp := range r.cfg.Opponents {
			sb.WriteString("- ")
			sb.WriteString(opp)
			sb.WriteString("\n")
		}
		sb.WriteString("\nThey are solving the exact same problem in parallel. Every second counts.\n")
	} else {
		sb.WriteString("You know your opponents are working on the same problem right now.\n")
	}
	sb.WriteString("Work smart. Work fast. Win.\n")

	return sb.String()
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
