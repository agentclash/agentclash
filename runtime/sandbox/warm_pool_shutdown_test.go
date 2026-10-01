package sandbox

import (
	"context"
	"errors"
	"testing"
	"time"
)

type shutdownFillProvider struct {
	started, release chan struct{}
	ignoreCancel     bool
	session          *FakeSession
}

func (p *shutdownFillProvider) Create(ctx context.Context, _ CreateRequest) (Session, error) {
	close(p.started)
	if p.ignoreCancel {
		<-p.release
	} else {
		<-ctx.Done()
	}
	if p.session != nil {
		return p.session, nil
	}
	return nil, context.Canceled
}

func TestWarmPoolCloseWaitsForLateSessionCleanupAndRetainsError(t *testing.T) {
	destroyErr := errors.New("local cleanup failure")
	session := NewFakeSession("late-fill")
	session.SetDestroyError(destroyErr)
	p := &shutdownFillProvider{started: make(chan struct{}), session: session}
	pool := WrapWarmPool(p, WarmPoolConfig{Size: 1})
	pool.scheduleFill("test", CreateRequest{})
	<-p.started
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := pool.Close(ctx); !errors.Is(err, destroyErr) {
		t.Fatalf("late cleanup error lost: %v", err)
	}
	if session.DestroyCalls() != 1 {
		t.Fatal("late-created session leaked")
	}
	if err := pool.Close(ctx); !errors.Is(err, destroyErr) {
		t.Fatalf("repeated Close forgot failure: %v", err)
	}
}

func TestWarmPoolCloseCancelsFillAndHonorsDeadline(t *testing.T) {
	for _, ignoresCancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "cooperative", true: "stuck"}[ignoresCancel], func(t *testing.T) {
			p := &shutdownFillProvider{started: make(chan struct{}), release: make(chan struct{}), ignoreCancel: ignoresCancel}
			defer close(p.release)
			pool := WrapWarmPool(p, WarmPoolConfig{Size: 1, FillTimeout: time.Hour})
			pool.Start()
			pool.scheduleFill("test", CreateRequest{})
			<-p.started
			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			err := pool.Close(ctx)
			if ignoresCancel && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected deadline, got %v", err)
			}
			if !ignoresCancel && err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Create(context.Background(), CreateRequest{}); err == nil {
				t.Fatal("closed pool accepted create")
			}
		})
	}
}
