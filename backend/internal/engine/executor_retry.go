package engine

import (
	"context"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
)

func (e NativeExecutor) invokeWithRetries(ctx context.Context, request provider.Request) (provider.Response, error) {
	backoff := e.initialRetryBackoff
	if backoff <= 0 {
		backoff = defaultRetryBackoff
	}
	attempts := e.maxRetryAttempts
	if attempts <= 0 {
		attempts = defaultRetryAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		response, err := e.invokeModel(ctx, request)
		if err == nil {
			return response, nil
		}

		failure, ok := provider.AsFailure(err)
		if !ok || !failure.Retryable || !isTransientProviderCode(failure.Code) || attempt == attempts {
			return provider.Response{}, err
		}

		lastErr = err
		wait := retryBackoff(failure, backoff)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return provider.Response{}, ctx.Err()
		case <-timer.C:
		}
		backoff *= 2
	}

	return provider.Response{}, lastErr
}

func (e NativeExecutor) invokeModel(ctx context.Context, request provider.Request) (provider.Response, error) {
	streamingClient, ok := e.client.(provider.StreamingClient)
	if !ok {
		return e.client.InvokeModel(ctx, request)
	}

	return streamingClient.StreamModel(ctx, request, func(delta provider.StreamDelta) error {
		if observerErr := e.observer.OnProviderOutput(ctx, request, delta); observerErr != nil {
			return NewFailure(StopReasonObserverError, "record native provider output event", observerErr)
		}
		return nil
	})
}

func isTransientProviderCode(code provider.FailureCode) bool {
	return code == provider.FailureCodeRateLimit ||
		code == provider.FailureCodeTimeout ||
		code == provider.FailureCodeUnavailable
}

func retryBackoff(failure provider.Failure, baseBackoff time.Duration) time.Duration {
	if failure.RetryAfter > 0 {
		return failure.RetryAfter + 1*time.Second
	}
	if failure.Code == provider.FailureCodeRateLimit && baseBackoff < rateLimitMinBackoff {
		return rateLimitMinBackoff
	}
	return baseBackoff
}
