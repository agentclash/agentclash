package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/agentclash/agentclash/backend/internal/productanalytics"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/agentclash/agentclash/backend/internal/storage"
	workflowpkg "github.com/agentclash/agentclash/backend/internal/workflow"
	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/sandbox"
	temporalsdk "go.temporal.io/sdk/client"
	"go.temporal.io/sdk/interceptor"
	sdkworker "go.temporal.io/sdk/worker"
)

type TemporalWorker interface {
	Start() error
	Stop()
}

type OrphanRunReaper interface {
	Start(ctx context.Context)
}

// multiQueueWorker hosts one Temporal worker per configured task queue class.
type multiQueueWorker struct {
	workers    []TemporalWorker
	started    int
	activities *activityDrain
}

func (m *multiQueueWorker) Start() error {
	for _, w := range m.workers {
		if err := w.Start(); err != nil {
			// RunWithReaper performs bounded cleanup of successfully started queues.
			return err
		}
		m.started++
	}
	return nil
}

func (m *multiQueueWorker) Stop() {
	var wg sync.WaitGroup
	for _, w := range m.workers[:m.started] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Stop()
		}()
	}
	wg.Wait()
	// SDK Stop cancels activities at grace expiry but need not await their
	// goroutines. Keep DB, sandbox and provider clients alive for cleanup.
	if m.activities != nil {
		m.activities.wait()
	}
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
	artifactStore storage.Store,
	productAnalytics ...productanalytics.ProductAnalytics,
) TemporalWorker {
	queues := cfg.TaskQueues
	if len(queues) == 0 {
		if cfg.TaskQueue != "" {
			queues = workflowpkg.ExpandTaskQueues([]string{cfg.TaskQueue})
		} else {
			queues = workflowpkg.AllTaskQueues()
		}
	}

	activities := workflowpkg.NewActivities(repo, executionHooks, playgroundClient).
		WithSandboxProvider(sandboxProvider).
		WithGitHubPullRequestClient(githubClient).
		WithPublicAgentTryoutConfig(cfg.AgentTryoutHosted).
		WithArtifactStore(artifactStore).
		WithEvalSetBudgetRepository(repo).
		WithWorkspaceRunCounter(repo).
		WithScanFindingRepository(repo)
	if len(productAnalytics) > 0 {
		activities.WithProductAnalytics(productAnalytics[0])
	}
	datasetActivities := workflowpkg.NewDatasetGenerationActivities(repo, playgroundClient, repo)

	drain := newActivityDrain()
	workers := make([]TemporalWorker, 0, len(queues))
	for _, queue := range queues {
		opts := temporalWorkerOptions(cfg, queue, drain)
		w := sdkworker.New(client, queue, opts)
		workflowpkg.RegisterForTaskQueue(w, activities, queue)
		workflowpkg.RegisterDatasetGenerationForTaskQueue(w, datasetActivities, queue)
		workers = append(workers, w)
	}

	return &multiQueueWorker{workers: workers, activities: drain}
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
		"task_queues", cfg.TaskQueues,
		"identity", cfg.Identity,
		"max_concurrent_activities", cfg.MaxConcurrentActivities,
		"max_concurrent_workflow_tasks", cfg.MaxConcurrentWorkflowTasks,
		"temporal_address", cfg.TemporalAddress,
		"temporal_namespace", cfg.TemporalNamespace,
	)

	if err := temporalWorker.Start(); err != nil {
		done := make(chan struct{})
		close(done)
		return errors.Join(fmt.Errorf("start temporal worker: %w", err), stopWorker(cfg, temporalWorker, done))
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
	return stopWorker(cfg, temporalWorker, reaperDoneCh)
}

func stopWorker(cfg Config, temporalWorker TemporalWorker, reaperDoneCh <-chan struct{}) error {
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

func temporalWorkerOptions(cfg Config, queue string, drain *activityDrain) sdkworker.Options {
	maxActs := cfg.MaxConcurrentActivities
	if maxActs <= 0 {
		maxActs = defaultMaxConcurrentActivities
	}
	maxWFT := cfg.MaxConcurrentWorkflowTasks
	if maxWFT <= 0 {
		maxWFT = defaultMaxConcurrentWorkflowTasks
	}

	opts := sdkworker.Options{
		Identity:                               fmt.Sprintf("%s/%s", cfg.Identity, queue),
		MaxConcurrentActivityExecutionSize:     maxActs,
		MaxConcurrentWorkflowTaskExecutionSize: maxWFT,
		WorkerStopTimeout:                      cfg.WorkerStopTimeout,
		Interceptors:                           []interceptor.WorkerInterceptor{drain},
	}
	if cfg.WorkerActivitiesPerSecond > 0 {
		opts.WorkerActivitiesPerSecond = cfg.WorkerActivitiesPerSecond
	}
	if cfg.TaskQueueActivitiesPerSecond > 0 {
		opts.TaskQueueActivitiesPerSecond = cfg.TaskQueueActivitiesPerSecond
	}
	return opts
}
