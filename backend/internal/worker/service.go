package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agentclash/agentclash/backend/internal/provider"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/sandbox"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	temporalsdk "go.temporal.io/sdk/client"
	sdkworker "go.temporal.io/sdk/worker"
)

type TemporalWorker interface {
	Start() error
	Stop()
}

type OrphanRunReaper interface {
	Start(ctx context.Context)
}

// executionHooks is the temporary extension seam for later hosted and native
// execution work without reshaping worker bootstrap.
func NewTemporalWorker(
	client temporalsdk.Client,
	cfg Config,
	repo *repository.Repository,
	playgroundClient provider.Client,
	sandboxProvider sandbox.Provider,
	githubClient workflowpkg.GitHubPullRequestClient,
	executionHooks workflowpkg.FakeWorkHooks,
) sdkworker.Worker {
	temporalWorker := sdkworker.New(client, cfg.TaskQueue, sdkworker.Options{
		Identity: cfg.Identity,
	})

	activities := workflowpkg.NewActivities(repo, executionHooks, playgroundClient).
		WithSandboxProvider(sandboxProvider).
		WithGitHubPullRequestClient(githubClient)
	workflowpkg.Register(temporalWorker, activities)
	workflowpkg.RegisterPlayground(temporalWorker, workflowpkg.NewPlaygroundActivities(repo, playgroundClient, repo))
	workflowpkg.RegisterDatasetGeneration(temporalWorker, workflowpkg.NewDatasetGenerationActivities(repo, playgroundClient, repo))

	return temporalWorker
}

func Run(ctx context.Context, cfg Config, temporalWorker TemporalWorker, logger *slog.Logger) error {
	return RunWithReaper(ctx, cfg, temporalWorker, logger)
}

// RunWithReaper starts the temporal worker plus any number of background
// reapers (orphan-run cleanup, anonymous tryout retention, ...), each on its
// own goroutine, and blocks until ctx is cancelled. nil reapers are ignored.
func RunWithReaper(ctx context.Context, cfg Config, temporalWorker TemporalWorker, logger *slog.Logger, reapers ...OrphanRunReaper) error {
	logger.Info("starting worker",
		"task_queue", cfg.TaskQueue,
		"identity", cfg.Identity,
		"temporal_address", cfg.TemporalAddress,
		"temporal_namespace", cfg.TemporalNamespace,
	)

	if err := temporalWorker.Start(); err != nil {
		return fmt.Errorf("start temporal worker: %w", err)
	}

	reaperDoneCh := make(chan struct{})
	active := make([]OrphanRunReaper, 0, len(reapers))
	for _, reaper := range reapers {
		if reaper != nil {
			active = append(active, reaper)
		}
	}
	if len(active) > 0 {
		var wg sync.WaitGroup
		wg.Add(len(active))
		for _, reaper := range active {
			go func(reaper OrphanRunReaper) {
				defer wg.Done()
				reaper.Start(ctx)
			}(reaper)
		}
		go func() {
			wg.Wait()
			close(reaperDoneCh)
		}()
	} else {
		close(reaperDoneCh)
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

	workerStopped := false
	reaperStopped := false
	for !workerStopped || !reaperStopped {
		select {
		case <-stoppedCh:
			workerStopped = true
			stoppedCh = nil
		case <-reaperDoneCh:
			reaperStopped = true
			reaperDoneCh = nil
		case <-timer.C:
			return fmt.Errorf("worker shutdown timed out after %s", cfg.ShutdownTimeout)
		}
	}
	return nil
}
