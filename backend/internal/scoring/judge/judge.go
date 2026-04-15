// Package judge implements the LLM-as-judge evaluator service for
// issue #148. Phase 3 ships assertion mode end-to-end; rubric,
// reference, and n_wise mode dispatch arms return Phase-gated
// unavailable results until their respective phases land.
//
// Design principles (see backend/.claude/analysis/issue-148-deep-
// analysis.md Parts 3 and 5):
//
//   - The evaluator is stateless and safe to share across goroutines.
//     All per-request state flows through Input and Result.
//
//   - One Evaluator with internal dispatch, NOT five parallel code
//     paths. The 5 issue-level "methods" are points on a cube with
//     orthogonal axes (scope, output shape, fan-out). A shared fanOut
//     helper runs (models × samples) in bounded parallelism for every
//     mode.
//
//   - Per-judge errors never abort the run. A judge that fails (rate
//     limit, schema error, parse failure) produces a scoring.JudgeResult
//     with state=error or state=unavailable and a Reason string; the
//     remaining judges for the same agent still run. Dimension
//     dispatch (Phase 4) treats an error/unavailable judge as a
//     missing dim and downgrades the scorecard to partial.
//
//   - Anti-gaming clauses are always injected into the prompt envelope
//     by the evaluator, regardless of pack config. Pack authors can
//     add more via LLMJudgeDeclaration.AntiGamingClauses but cannot
//     remove the defaults.
//
// Phase 3 does NOT include: Temporal activity wiring (Phase 4),
// rubric/reference/n_wise dispatch (Phases 5/6), multi-provider
// credential overrides (Phase 7), the write path into
// repository.UpsertLLMJudgeResult (Phase 4), or removal of the
// errJudgeModeUnsupported runtime gate in engine.go (Phase 4).
package judge

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

// Config parameterises the Evaluator. The zero value is NOT usable —
// callers must at minimum supply a CredentialReference for the
// underlying provider calls. NewEvaluator fills in sensible defaults
// for the other fields so common call-sites can pass a sparse Config.
type Config struct {
	// MaxParallel bounds concurrent LLM calls across all fan-out axes
	// (models × samples) for a single Evaluate invocation. Defaults
	// to 4. Setting this too high multiplies rate-limit exposure;
	// setting it to 1 serialises all calls for deterministic tests.
	MaxParallel int

	// DefaultAssertionModel is the provider model used when a judge's
	// Mode is assertion and neither Model nor Models is set. Defaults
	// to claude-haiku-4-5-20251001: binary output, short prompt, cheap.
	// Callers can override per-judge via LLMJudgeDeclaration.Model.
	DefaultAssertionModel string

	// Providers maps model identifier → provider.Router key. Explicit
	// entries take precedence over the well-known prefix fallback in
	// resolveProviderKey. Use this when a pack wants to pin a specific
	// provider account (e.g., routing "claude-sonnet-4-6" through
	// openrouter instead of anthropic direct).
	Providers map[string]string

	// CredentialReference is the default credential reference passed
	// to every judge provider.Request. Typically a workspace-secret://
	// URI so EnvCredentialResolver resolves it from the workspace
	// secrets loaded by the Temporal activity. REQUIRED — the
	// evaluator has no fallback.
	CredentialReference string

	// DefaultTimeout caps any single LLM call. Overridden per-judge by
	// LLMJudgeDeclaration.TimeoutMS. Defaults to 60 seconds — long
	// enough for a Haiku assertion with thinking, short enough to
	// surface stuck calls before the enclosing Temporal activity
	// times out.
	DefaultTimeout time.Duration

	// Logger receives structured events about judge dispatch. Defaults
	// to slog.Default() when nil. The judge package never logs agent
	// output or prompt content; only control flow and error reasons.
	Logger *slog.Logger
}

// Input is the per-agent input to the evaluator. The Phase 4
// JudgeRunAgent activity constructs this after deterministic scoring
// completes, pulling FinalOutput from run_agent_replays and
// ChallengeInput from the evaluation spec's challenge context. The
// evaluator does NOT re-read events or touch the DB.
type Input struct {
	RunAgentID       uuid.UUID
	EvaluationSpecID uuid.UUID
	Judges           []scoring.LLMJudgeDeclaration
	FinalOutput      string
	// ChallengeInput is optional context stitched into the prompt
	// envelope when non-empty. Assertion-mode packs that want the
	// judge to see the original challenge (e.g., to check whether
	// the agent answered the actual question) supply it here.
	ChallengeInput string
}

// Result is the aggregated output of evaluating every judge for one
// agent. The caller persists each JudgeResult via
// repository.UpsertLLMJudgeResult (Phase 4 activity).
//
// JudgeResults is the same length as Input.Judges and maintains
// order so callers can correlate results back to spec declarations
// by index or by Key.
//
// Warnings is a free-form slice of human-readable strings describing
// non-fatal issues the evaluator hit (e.g., "judge foo: rate limited").
// Phase 4 merges these into RunAgentEvaluation.Warnings.
type Result struct {
	RunAgentID   uuid.UUID
	JudgeResults []scoring.JudgeResult
	Warnings     []string
}

// Evaluator runs LLM-as-judge evaluation for a single run-agent. It
// is stateless beyond its constructor-time configuration and is safe
// to share across goroutines or Temporal activity invocations.
type Evaluator struct {
	router provider.Router
	cfg    Config
}

// NewEvaluator returns an Evaluator backed by the given provider
// router. Missing Config fields are populated with defaults:
//
//   - MaxParallel → 4
//   - DefaultAssertionModel → claude-haiku-4-5-20251001
//   - DefaultTimeout → 60 seconds
//   - Logger → slog.Default()
//
// CredentialReference is NOT defaulted — a missing credential would
// silently route to no provider, which we'd rather surface as a clear
// runtime error than a configuration default. Callers that want
// env-only dev loops can pass "env:API_KEY" explicitly.
func NewEvaluator(router provider.Router, cfg Config) *Evaluator {
	if cfg.MaxParallel <= 0 {
		cfg.MaxParallel = 4
	}
	if cfg.DefaultAssertionModel == "" {
		cfg.DefaultAssertionModel = "claude-haiku-4-5-20251001"
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Evaluator{router: router, cfg: cfg}
}

// Evaluate runs every declared judge for the given agent and returns
// one scoring.JudgeResult per judge_key in the same order as
// in.Judges. Per-judge failures are captured as error-state results,
// NOT returned as errors, so the caller receives one Result per
// Evaluate call and every judge gets its own JudgeResult row in the
// Phase 4 persistence path.
//
// The error return is reserved for Evaluate-wide failures (ctx
// cancellation before any judge runs, internal invariants). In
// Phase 3 practice it is always nil; Phase 4+ may introduce Evaluate-
// wide errors tied to the activity contract.
func (e *Evaluator) Evaluate(ctx context.Context, in Input) (Result, error) {
	results := make([]scoring.JudgeResult, 0, len(in.Judges))
	warnings := make([]string, 0)

	for _, judge := range in.Judges {
		if ctx.Err() != nil {
			// Remaining judges are marked cancelled so the caller sees
			// a clear record of what ran vs. what was skipped. The
			// fanOut helper handles this at a finer granularity for
			// samples/models within one judge.
			results = append(results, scoring.JudgeResult{
				Key:    judge.Key,
				Mode:   judge.Mode,
				State:  scoring.OutputStateError,
				Reason: fmt.Sprintf("judge %q: context cancelled before dispatch: %v", judge.Key, ctx.Err()),
			})
			warnings = append(warnings, fmt.Sprintf("judge %q: cancelled", judge.Key))
			continue
		}
		result := e.evaluateOne(ctx, judge, in)
		results = append(results, result)
		if result.State == scoring.OutputStateError && result.Reason != "" {
			warnings = append(warnings, fmt.Sprintf("judge %q: %s", judge.Key, result.Reason))
		}
	}

	return Result{
		RunAgentID:   in.RunAgentID,
		JudgeResults: results,
		Warnings:     warnings,
	}, nil
}

// evaluateOne dispatches to the mode-specific handler. Phase 3 only
// implements assertion; rubric/reference/n_wise return a placeholder
// unavailable result that identifies the phase gate for observability.
func (e *Evaluator) evaluateOne(ctx context.Context, judge scoring.LLMJudgeDeclaration, in Input) scoring.JudgeResult {
	switch judge.Mode {
	case scoring.JudgeMethodAssertion:
		return e.evaluateAssertion(ctx, judge, in)
	case scoring.JudgeMethodRubric:
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   judge.Mode,
			State:  scoring.OutputStateUnavailable,
			Reason: "rubric mode arrives in #148 phase 5",
		}
	case scoring.JudgeMethodReference:
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   judge.Mode,
			State:  scoring.OutputStateUnavailable,
			Reason: "reference mode arrives in #148 phase 5",
		}
	case scoring.JudgeMethodNWise:
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   judge.Mode,
			State:  scoring.OutputStateUnavailable,
			Reason: "n_wise mode arrives in #148 phase 6",
		}
	default:
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   judge.Mode,
			State:  scoring.OutputStateError,
			Reason: fmt.Sprintf("unsupported judge mode %q", judge.Mode),
		}
	}
}
