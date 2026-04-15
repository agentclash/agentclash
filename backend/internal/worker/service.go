package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring/judge"
	workflowpkg "github.com/Atharva-Kanherkar/agentclash/backend/internal/workflow"
	temporalsdk "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

type TemporalWorker interface {
	Start() error
	Stop()
}

// executionHooks is the temporary extension seam for later hosted and native
// execution work without reshaping worker bootstrap.
//
// providerRouter is passed by value (provider.Router is a struct, not an
// interface). The playground activities take a provider.Client; Router
// satisfies Client so the same value is reused there. The judge
// evaluator wants the Router directly because Phase 7 may want
// per-judge routing decisions that go beyond the single InvokeModel
// surface — keeping the type narrow now would force a refactor later.
func NewTemporalWorker(
	client temporalsdk.Client,
	cfg Config,
	repo *repository.Repository,
	providerRouter provider.Router,
	executionHooks workflowpkg.FakeWorkHooks,
) sdkworker.Worker {
	temporalWorker := sdkworker.New(client, cfg.TaskQueue, sdkworker.Options{
		Identity: cfg.Identity,
	})

	// Wire the LLM-as-judge evaluator (#148 phase 4) onto the
	// scoring activities. JudgeCredentialReference comes from worker
	// config (env var with default env://ANTHROPIC_API_KEY) and
	// applies to every judge call until Phase 7 introduces per-pack
	// credential overrides via ScorecardDeclaration.JudgeProviderRef.
	judgeEvaluator := judge.NewEvaluator(providerRouter, judge.Config{
		MaxParallel:           4,
		DefaultAssertionModel: "claude-haiku-4-5-20251001",
		CredentialReference:   cfg.JudgeCredentialReference,
		DefaultTimeout:        60 * time.Second,
	})

	activities := workflowpkg.NewActivities(repo, executionHooks).WithJudgeEvaluator(judgeEvaluator)
	workflowpkg.Register(temporalWorker, activities)
	workflowpkg.RegisterPlayground(temporalWorker, workflowpkg.NewPlaygroundActivities(repo, providerRouter, repo))

	return temporalWorker
}

func Run(ctx context.Context, cfg Config, temporalWorker TemporalWorker, logger *slog.Logger) error {
	logger.Info("starting worker",
		"task_queue", cfg.TaskQueue,
		"identity", cfg.Identity,
		"temporal_address", cfg.TemporalAddress,
		"temporal_namespace", cfg.TemporalNamespace,
	)

	if err := temporalWorker.Start(); err != nil {
		return fmt.Errorf("start temporal worker: %w", err)
	}

	<-ctx.Done()

	logger.Info("stopping worker", "shutdown_timeout", cfg.ShutdownTimeout.String())

	stoppedCh := make(chan struct{}, 1)
	go func() {
		temporalWorker.Stop()
		stoppedCh <- struct{}{}
	}()

	timer := time.NewTimer(cfg.ShutdownTimeout)
	defer timer.Stop()

	select {
	case <-stoppedCh:
		return nil
	case <-timer.C:
		return fmt.Errorf("worker shutdown timed out after %s", cfg.ShutdownTimeout)
	}
}
