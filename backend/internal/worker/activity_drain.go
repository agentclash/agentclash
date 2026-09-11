package worker

import (
	"context"
	"sync"

	"go.temporal.io/sdk/interceptor"
	sdkworker "go.temporal.io/sdk/worker"
)

// activityDrain waits for application activity code, including deferred sandbox
// cleanup, after SDK Stop has stopped polling and cancelled activity contexts.
// It does not checkpoint activity state or alter workflow retry policies.
type activityDrain struct {
	interceptor.WorkerInterceptorBase
	mu     sync.Mutex
	active int
	sealed bool
	done   chan struct{}
}

func newActivityDrain() *activityDrain { return &activityDrain{done: make(chan struct{})} }

func (d *activityDrain) InterceptActivity(_ context.Context, next interceptor.ActivityInboundInterceptor) interceptor.ActivityInboundInterceptor {
	return &drainingActivity{ActivityInboundInterceptorBase: interceptor.ActivityInboundInterceptorBase{Next: next}, drain: d}
}

func (d *activityDrain) wait() {
	d.mu.Lock()
	if !d.sealed {
		d.sealed = true
		if d.active == 0 {
			close(d.done)
		}
	}
	d.mu.Unlock()
	<-d.done
}

type drainingActivity struct {
	interceptor.ActivityInboundInterceptorBase
	drain *activityDrain
}

func (a *drainingActivity) ExecuteActivity(ctx context.Context, in *interceptor.ExecuteActivityInput) (any, error) {
	d := a.drain
	d.mu.Lock()
	if d.sealed {
		d.mu.Unlock()
		// A task already dispatched by the SDK may only reach this interceptor
		// after Stop. Never start new business work on closed dependencies.
		return nil, sdkworker.ErrWorkerShutdown
	}
	d.active++
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.active--
		if d.sealed && d.active == 0 {
			close(d.done)
		}
	}()
	return a.Next.ExecuteActivity(ctx, in)
}
