package runner

import (
	"context"
	"errors"
	"time"
)

type TerminalObserverMessages struct {
	Failure    string
	Completion string
}

// FinishWithObserver records the terminal run event and preserves the executor
// contract that observer failures become StopReasonObserverError failures.
func FinishWithObserver(ctx context.Context, observer Observer, result *Result, err *error, messages TerminalObserverMessages) {
	if observer == nil {
		observer = NoopObserver{}
	}
	if *err != nil {
		if observerErr := observer.OnRunFailure(ctx, *err); observerErr != nil {
			*err = errors.Join(*err, NewFailure(StopReasonObserverError, messages.Failure, observerErr))
		}
		return
	}
	if observerErr := observer.OnRunComplete(ctx, *result); observerErr != nil {
		*result = Result{}
		*err = NewFailure(StopReasonObserverError, messages.Completion, observerErr)
	}
}

type TimedContext struct {
	Context context.Context
	Cancel  context.CancelFunc
}

func WithRuntimeTimeout(ctx context.Context, timeout time.Duration) TimedContext {
	if timeout <= 0 {
		return TimedContext{
			Context: ctx,
			Cancel:  func() {},
		}
	}
	child, cancel := context.WithTimeout(ctx, timeout)
	return TimedContext{
		Context: child,
		Cancel:  cancel,
	}
}
