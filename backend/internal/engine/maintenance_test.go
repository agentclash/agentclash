package engine

import (
	"context"
	"testing"
	"time"

	"github.com/agentclash/agentclash/runtime/provider"
	"github.com/agentclash/agentclash/runtime/sandbox"
)

type cancelledModelClient struct{ started chan struct{} }

func (c cancelledModelClient) InvokeModel(ctx context.Context, _ provider.Request) (provider.Response, error) {
	close(c.started)
	<-ctx.Done()
	return provider.Response{}, ctx.Err()
}

type cleanupContextSession struct {
	*sandbox.FakeSession
	cleanupErr error
	bounded    bool
}

func (s *cleanupContextSession) Destroy(ctx context.Context) error {
	s.cleanupErr = ctx.Err()
	_, s.bounded = ctx.Deadline()
	return s.FakeSession.Destroy(ctx)
}

type cleanupContextProvider struct{ session sandbox.Session }

func (p cleanupContextProvider) Create(context.Context, sandbox.CreateRequest) (sandbox.Session, error) {
	return p.session, nil
}

func TestCancelledNativeExecutionDestroysSandboxWithFreshDeadline(t *testing.T) {
	session := &cleanupContextSession{FakeSession: sandbox.NewFakeSession("local-cancellation-test")}
	client := cancelledModelClient{started: make(chan struct{})}
	executor := NewNativeExecutor(client, cleanupContextProvider{session}, NoopObserver{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := executor.Execute(ctx, nativeExecutionContext()); done <- err }()
	select {
	case <-client.started:
	case <-time.After(time.Second):
		t.Fatal("model invocation did not start")
	}
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled invocation succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("execution did not cancel")
	}
	if session.DestroyCalls() != 1 || session.cleanupErr != nil || !session.bounded {
		t.Fatalf("sandbox cleanup: calls=%d cancelled=%v bounded=%v", session.DestroyCalls(), session.cleanupErr, session.bounded)
	}
}
