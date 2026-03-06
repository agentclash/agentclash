package scoring

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/internal/telemetry"
)

// Result is the final score for one agent.
type Result struct {
	Rank            int
	AgentName       string
	Model           string
	Submitted       bool
	Duration        time.Duration
	TotalTokens     int
	TotalSteps      int
	ToolCalls       int
	UniqueTools     int
	FailedTools     int
	BashUses        int
	StructuredUses  int
	FinalScore      float64
	SubmitExplanation string
}

// Rank scores and sorts all agents from a race.
func Rank(traces []*telemetry.Trace) []Result {
	var results []Result

	for _, t := range traces {
		duration := t.EndedAt.Sub(t.StartedAt)

		var bashUses, structuredUses int
		for _, s := range t.Steps {
			if s.Type == telemetry.StepAct {
				if s.ToolName == "bash" {
					bashUses++
				} else if s.ToolName != "" {
					structuredUses++
				}
			}
		}

		score := computeScore(t, duration, bashUses, structuredUses)

		results = append(results, Result{
			AgentName:         t.AgentName,
			Model:             t.Model,
			Submitted:         t.Submitted,
			Duration:          duration,
			TotalTokens:       t.TotalTokens,
			TotalSteps:        len(t.Steps),
			ToolCalls:         t.TotalToolCalls,
			UniqueTools:       t.UniqueToolsUsed,
			FailedTools:       t.FailedToolCalls,
			BashUses:          bashUses,
			StructuredUses:    structuredUses,
			FinalScore:        score,
			SubmitExplanation: t.SubmitExplanation,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})

	for i := range results {
		results[i].Rank = i + 1
	}

	return results
}

func computeScore(t *telemetry.Trace, duration time.Duration, bashUses, structuredUses int) float64 {
	score := 0.0

	// Completion: 40 points for submitting a solution
	if t.Submitted {
		score += 40
	}

	// Speed: up to 25 points (faster = more points, 10min baseline)
	maxTime := 10 * time.Minute
	if duration > 0 && duration < maxTime {
		speedFactor := 1.0 - (float64(duration) / float64(maxTime))
		score += speedFactor * 25
	}

	// Token efficiency: up to 20 points (fewer tokens = more points, 100k baseline)
	maxTokens := 100000.0
	if t.TotalTokens > 0 && float64(t.TotalTokens) < maxTokens {
		effFactor := 1.0 - (float64(t.TotalTokens) / maxTokens)
		score += effFactor * 20
	}

	// Tool strategy: up to 15 points
	totalToolUses := bashUses + structuredUses
	if totalToolUses > 0 {
		// Ratio of structured tool usage
		structuredRatio := float64(structuredUses) / float64(totalToolUses)
		score += structuredRatio * 10

		// Bonus for tool diversity (using different tools effectively)
		if t.UniqueToolsUsed >= 4 {
			score += 5
		} else if t.UniqueToolsUsed >= 2 {
			score += 3
		}
	}

	// Penalty for failed tool calls (wasted effort)
	if t.FailedToolCalls > 3 {
		score -= float64(t.FailedToolCalls-3) * 0.5
	}

	if score < 0 {
		score = 0
	}
	return score
}

// PrintResults outputs a formatted race results table.
func PrintResults(results []Result) {
	sep := strings.Repeat("=", 80)
	fmt.Printf("\n%s\n", sep)
	fmt.Println("                       RACE RESULTS")
	fmt.Println(sep)

	for _, r := range results {
		medal := "  "
		switch r.Rank {
		case 1:
			medal = " 1"
		case 2:
			medal = " 2"
		case 3:
			medal = " 3"
		}

		submitted := "NO "
		if r.Submitted {
			submitted = "YES"
		}

		fmt.Printf("\n%s #%d  %s (%s)\n", medal, r.Rank, r.AgentName, r.Model)
		fmt.Printf("     Score: %.1f  |  Solved: %s  |  Time: %s\n",
			r.FinalScore, submitted, r.Duration.Round(time.Second))
		fmt.Printf("     Tokens: %d  |  Steps: %d  |  Tool calls: %d (structured: %d, bash: %d)\n",
			r.TotalTokens, r.TotalSteps, r.ToolCalls, r.StructuredUses, r.BashUses)
		if r.SubmitExplanation != "" {
			fmt.Printf("     Explanation: %s\n", r.SubmitExplanation)
		}
	}

	fmt.Printf("\n%s\n", sep)
}
