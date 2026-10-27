package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	sdkworkflow "go.temporal.io/sdk/workflow"
)

const (
	executeRunAgentCaseActivityName   = "workflow.execute_run_agent_case"
	runAgentCaseFanoutVersionChangeID = "run-agent-case-fanout"
	defaultMaxInFlightCases           = 8
	caseFanoutProfileConfigKey        = "case_fanout"
	maxInFlightCasesProfileConfigKey  = "max_in_flight_cases"
)

// ExecuteRunAgentCaseInput identifies a single case within a run-agent.
type ExecuteRunAgentCaseInput struct {
	RunID      uuid.UUID `json:"run_id"`
	RunAgentID uuid.UUID `json:"run_agent_id"`
	CaseKey    string    `json:"case_key"`
	CaseIndex  int       `json:"case_index"`
}

// ExecuteRunAgentCaseResult is the durable per-case outcome collected by the
// workflow. Activity-level errors that exhaust retries still surface here as
// Success=false so the run-agent can complete with a partial scorecard.
type ExecuteRunAgentCaseResult struct {
	CaseKey      string `json:"case_key"`
	CaseIndex    int    `json:"case_index"`
	Success      bool   `json:"success"`
	StopReason   string `json:"stop_reason,omitempty"`
	FinalOutput  string `json:"final_output,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type caseFanoutConfig struct {
	Enabled     bool
	MaxInFlight int
}

func parseCaseFanoutConfig(executionContext repository.RunAgentExecutionContext) caseFanoutConfig {
	cfg := caseFanoutConfig{
		Enabled:     false,
		MaxInFlight: defaultMaxInFlightCases,
	}
	raw := executionContext.Deployment.RuntimeProfile.ProfileConfig
	if len(raw) == 0 {
		return cfg
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return cfg
	}
	if enabled, ok := decoded[caseFanoutProfileConfigKey].(bool); ok {
		cfg.Enabled = enabled
	}
	switch v := decoded[maxInFlightCasesProfileConfigKey].(type) {
	case float64:
		if int(v) > 0 {
			cfg.MaxInFlight = int(v)
		}
	case int:
		if v > 0 {
			cfg.MaxInFlight = v
		}
	case json.Number:
		if n, err := v.Int64(); err == nil && n > 0 {
			cfg.MaxInFlight = int(n)
		}
	}
	return cfg
}

func caseFanoutEnabled(executionContext repository.RunAgentExecutionContext) bool {
	return parseCaseFanoutConfig(executionContext).Enabled
}

func listCaseKeysForFanout(executionContext repository.RunAgentExecutionContext) []string {
	if executionContext.ChallengeInputSet == nil {
		return nil
	}
	keys := make([]string, 0, len(executionContext.ChallengeInputSet.Cases))
	for i, c := range executionContext.ChallengeInputSet.Cases {
		key := strings.TrimSpace(c.CaseKey)
		if key == "" {
			key = strings.TrimSpace(c.ItemKey)
		}
		if key == "" {
			key = fmt.Sprintf("case-%d", i)
		}
		keys = append(keys, key)
	}
	return keys
}

func narrowExecutionContextToCase(executionContext repository.RunAgentExecutionContext, caseKey string) (repository.RunAgentExecutionContext, error) {
	if executionContext.ChallengeInputSet == nil {
		return repository.RunAgentExecutionContext{}, fmt.Errorf("challenge input set is required for case fan-out")
	}
	var matched *repository.ChallengeCaseExecutionContext
	for i := range executionContext.ChallengeInputSet.Cases {
		c := &executionContext.ChallengeInputSet.Cases[i]
		key := strings.TrimSpace(c.CaseKey)
		if key == "" {
			key = strings.TrimSpace(c.ItemKey)
		}
		if key == caseKey {
			matched = c
			break
		}
	}
	if matched == nil {
		return repository.RunAgentExecutionContext{}, fmt.Errorf("case %q not found in execution context", caseKey)
	}
	narrowed := executionContext
	narrowed.ExecutionCaseKey = caseKey
	inputSetCopy := *executionContext.ChallengeInputSet
	inputSetCopy.Cases = []repository.ChallengeCaseExecutionContext{*matched}
	// Items are the legacy parallel view; keep only the matching item when present.
	if len(inputSetCopy.Items) > 0 {
		filtered := make([]repository.ChallengeInputItemExecutionContext, 0, 1)
		for _, item := range inputSetCopy.Items {
			if strings.TrimSpace(item.ItemKey) == caseKey || item.ID == matched.ID {
				filtered = append(filtered, item)
			}
		}
		inputSetCopy.Items = filtered
	}
	narrowed.ChallengeInputSet = &inputSetCopy
	return narrowed, nil
}

func caseActivityTimeout(executionContext repository.RunAgentExecutionContext, caseKey string) time.Duration {
	timeoutSecs := executionContext.Deployment.RuntimeProfile.RunTimeoutSeconds
	if executionContext.ChallengeInputSet != nil {
		for _, c := range executionContext.ChallengeInputSet.Cases {
			key := strings.TrimSpace(c.CaseKey)
			if key == "" {
				key = strings.TrimSpace(c.ItemKey)
			}
			if key == caseKey && c.CaseTimeoutSeconds > 0 {
				timeoutSecs = c.CaseTimeoutSeconds
				break
			}
		}
	}
	if timeoutSecs <= 0 {
		return defaultActivityOptions.StartToCloseTimeout
	}
	return time.Duration(timeoutSecs)*time.Second + nativeActivityBootBuffer + nativeActivityCleanupBuffer
}

func caseActivityOptions(ctx sdkworkflow.Context, executionContext repository.RunAgentExecutionContext, caseKey string) sdkworkflow.ActivityOptions {
	opts := nativeModelActivityOptions(executionContext)
	opts.StartToCloseTimeout = caseActivityTimeout(executionContext, caseKey)
	return withActivityTaskQueue(ctx, opts, TaskQueueExecution)
}

// executeNativeCasesBounded launches per-case activities with a deterministic
// in-flight cap. Individual case failures are collected; the function returns
// nil unless the workflow is canceled.
func executeNativeCasesBounded(
	ctx sdkworkflow.Context,
	input RunAgentWorkflowInput,
	executionContext repository.RunAgentExecutionContext,
) error {
	cfg := parseCaseFanoutConfig(executionContext)
	caseKeys := listCaseKeysForFanout(executionContext)
	if len(caseKeys) == 0 {
		// No cases → fall back to the mega-activity so empty packs keep
		// today's behavior (native executor handles the empty set).
		return executeNativeModelStep(ctx, input, executionContext).Get(ctx, nil)
	}

	maxInFlight := cfg.MaxInFlight
	if maxInFlight <= 0 {
		maxInFlight = defaultMaxInFlightCases
	}
	if maxInFlight > len(caseKeys) {
		maxInFlight = len(caseKeys)
	}

	type pendingCase struct {
		index  int
		key    string
		future sdkworkflow.Future
	}

	results := make([]ExecuteRunAgentCaseResult, len(caseKeys))
	pending := make(map[sdkworkflow.Future]pendingCase, maxInFlight)
	nextIndex := 0

	launch := func(index int) {
		key := caseKeys[index]
		future := sdkworkflow.ExecuteActivity(
			sdkworkflow.WithActivityOptions(ctx, caseActivityOptions(ctx, executionContext, key)),
			executeRunAgentCaseActivityName,
			ExecuteRunAgentCaseInput{
				RunID:      input.RunID,
				RunAgentID: input.RunAgentID,
				CaseKey:    key,
				CaseIndex:  index,
			},
		)
		pending[future] = pendingCase{index: index, key: key, future: future}
	}

	for nextIndex < maxInFlight {
		launch(nextIndex)
		nextIndex++
	}

	for len(pending) > 0 {
		selector := sdkworkflow.NewSelector(ctx)
		// Snapshot futures in a stable order so the selector registration
		// order is deterministic across replay (map iteration is not).
		futures := make([]sdkworkflow.Future, 0, len(pending))
		for f := range pending {
			futures = append(futures, f)
		}
		sort.Slice(futures, func(i, j int) bool {
			left := pending[futures[i]]
			right := pending[futures[j]]
			return left.index < right.index
		})

		var (
			completed pendingCase
			cancelErr error
		)
		for _, f := range futures {
			future := f
			pc := pending[future]
			selector.AddFuture(future, func(fut sdkworkflow.Future) {
				completed = pc
				var result ExecuteRunAgentCaseResult
				err := fut.Get(ctx, &result)
				if err != nil {
					if isWorkflowCanceled(err) {
						cancelErr = err
						return
					}
					results[pc.index] = ExecuteRunAgentCaseResult{
						CaseKey:      pc.key,
						CaseIndex:    pc.index,
						Success:      false,
						ErrorMessage: err.Error(),
					}
					return
				}
				result.CaseKey = pc.key
				result.CaseIndex = pc.index
				results[pc.index] = result
			})
		}
		selector.Select(ctx)
		if cancelErr != nil {
			return cancelErr
		}
		delete(pending, completed.future)

		if nextIndex < len(caseKeys) {
			launch(nextIndex)
			nextIndex++
		}
	}

	// Partial-failure: never fail the run-agent for individual case errors.
	// Callers transition to Evaluating; scoring produces EvaluationStatusPartial
	// when evidence is incomplete.
	succeeded := 0
	for _, r := range results {
		if r.Success {
			succeeded++
		}
	}
	sdkworkflow.GetLogger(ctx).Info("case fan-out completed",
		"run_agent_id", input.RunAgentID.String(),
		"total_cases", len(results),
		"succeeded", succeeded,
		"failed", len(results)-succeeded,
	)
	return nil
}
