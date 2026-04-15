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

	// DefaultRubricModel is the provider model used when a rubric or
	// reference judge omits both Model and Models. Defaults to
	// claude-sonnet-4-6: rubric/reference require structured JSON
	// output and calibrated numeric reasoning, which Haiku struggles
	// with. Pack authors can still override per-judge.
	DefaultRubricModel string

	// DefaultNWiseModel is the provider model used when an n_wise
	// judge omits both Model and Models. Defaults to claude-sonnet-4-6
	// for the same reason as rubric: n_wise ranking needs structured
	// output and cross-agent reasoning. Pack authors can override.
	DefaultNWiseModel string

	// NWiseMaxOutputChars caps the per-agent final_output character
	// count rendered inside an n_wise prompt. Defaults to 4000.
	// Outputs longer than this are truncated with a [... truncated ...]
	// marker and the evaluator pushes a warning listing the affected
	// agents. Matches the analysis doc Part 10 guidance for preventing
	// context-window blowups when a run has several verbose agents.
	NWiseMaxOutputChars int

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
	// ResolvedReferences holds the union of every judge's
	// ContextFrom + ReferenceFrom values, already resolved against
	// the run-agent's evidence via scoring.ResolveContextReferences.
	// Keyed by the evidence reference string (e.g.,
	// "challenge_input", "case.payload.foo"). Populated by the
	// Phase 4+ JudgeRunAgent activity; nil for deterministic-only
	// callers. Phase 5 rubric/reference dispatch reads from this
	// map to build CONTEXT and REFERENCE ANSWER prompt blocks.
	ResolvedReferences map[string]string
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
//   - DefaultRubricModel → claude-sonnet-4-6
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
	if cfg.DefaultRubricModel == "" {
		cfg.DefaultRubricModel = "claude-sonnet-4-6"
	}
	if cfg.DefaultNWiseModel == "" {
		cfg.DefaultNWiseModel = "claude-sonnet-4-6"
	}
	if cfg.NWiseMaxOutputChars <= 0 {
		cfg.NWiseMaxOutputChars = 4000
	}
	if cfg.DefaultTimeout <= 0 {
		cfg.DefaultTimeout = 60 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &Evaluator{router: router, cfg: cfg}
}

// Evaluate runs every PER-AGENT judge for the given agent and returns
// one scoring.JudgeResult per judge_key in the same order as
// in.Judges. Per-judge failures are captured as error-state results,
// NOT returned as errors, so the caller receives one Result per
// Evaluate call and every judge gets its own JudgeResult row in the
// Phase 4 persistence path.
//
// n_wise judges are SKIPPED here — they run at the run level via
// Evaluator.EvaluateNWise (Phase 6) because the per-agent Input has
// no cross-agent access. The workflow-side activity splits the
// spec's judges by mode and dispatches each set to the right entry
// point. Silently skipping (rather than emitting an unavailable
// stub) keeps the per-agent result list clean for persistence — the
// n_wise results come from a separate UpsertLLMJudgeResult loop in
// the JudgeRun activity.
//
// The error return is reserved for Evaluate-wide failures (ctx
// cancellation before any judge runs, internal invariants). In
// practice it is always nil; Phase 4+ may introduce Evaluate-wide
// errors tied to the activity contract.
func (e *Evaluator) Evaluate(ctx context.Context, in Input) (Result, error) {
	results := make([]scoring.JudgeResult, 0, len(in.Judges))
	warnings := make([]string, 0)

	for _, judge := range in.Judges {
		// Skip n_wise judges — they run at the run level via
		// EvaluateNWise. Leaving them out of per-agent results keeps
		// the persistence path idempotent (the n_wise rows come from
		// the JudgeRun activity, not the per-agent JudgeRunAgent).
		if judge.Mode == scoring.JudgeMethodNWise {
			continue
		}
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

// evaluateOne dispatches to the mode-specific handler. Phase 3
// shipped assertion; Phase 5 wires rubric + reference through the
// shared evaluateRubric path; n_wise is NOT dispatched here because
// it operates at run-level (not per-agent) and needs a different
// entry point — the Phase 6 JudgeRun activity calls
// Evaluator.EvaluateNWise directly with all agents' final outputs.
// Per-agent Evaluate.Input has no cross-agent access, so n_wise
// judges passed through Evaluate skip over to an unavailable stub
// with a phase-gated reason pointing at the run-level path.
func (e *Evaluator) evaluateOne(ctx context.Context, judge scoring.LLMJudgeDeclaration, in Input) scoring.JudgeResult {
	switch judge.Mode {
	case scoring.JudgeMethodAssertion:
		return e.evaluateAssertion(ctx, judge, in)
	case scoring.JudgeMethodRubric, scoring.JudgeMethodReference:
		return e.evaluateRubric(ctx, judge, in)
	case scoring.JudgeMethodNWise:
		// n_wise is run-level. Evaluate (per-agent) filters these
		// out before reaching dispatch; this stub defends against
		// callers that pass an n_wise judge through the per-agent
		// path anyway (e.g., programmatic test fixtures).
		return scoring.JudgeResult{
			Key:    judge.Key,
			Mode:   judge.Mode,
			State:  scoring.OutputStateError,
			Reason: "n_wise judges run via Evaluator.EvaluateNWise at the run level, not per-agent Evaluate",
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
