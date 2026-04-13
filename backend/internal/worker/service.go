package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
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
func NewTemporalWorker(
	client temporalsdk.Client,
	cfg Config,
	repo *repository.Repository,
	playgroundClient provider.Client,
	executionHooks workflowpkg.FakeWorkHooks,
) sdkworker.Worker {
	temporalWorker := sdkworker.New(client, cfg.TaskQueue, sdkworker.Options{
		Identity: cfg.Identity,
	})

	activities := workflowpkg.NewActivities(repo, executionHooks)
	workflowpkg.Register(temporalWorker, activities)
	workflowpkg.RegisterPlayground(temporalWorker, workflowpkg.NewPlaygroundActivities(repo, playgroundClient))

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
