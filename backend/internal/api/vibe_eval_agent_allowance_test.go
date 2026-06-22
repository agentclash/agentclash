package api

import (
	"context"
	"errors"
	"testing"
	"time"

	billingpkg "github.com/agentclash/agentclash/backend/internal/billing"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
)

type fakeGuideMeter struct {
	calls int
	err   error
}

func (f *fakeGuideMeter) ConsumeGuideAgentTurn(_ context.Context, _ uuid.UUID) error {
	f.calls++
	return f.err
}

func allowanceManager(meter guideTurnMeter, now time.Time) *VibeEvalAgentManager {
	return &VibeEvalAgentManager{meter: meter, now: func() time.Time { return now }}
}

func quotaGateError() error {
	return billingpkg.GateError{Decision: billingpkg.GateDecision{Allowed: false, Code: billingpkg.GateCodeQuotaExceeded, Message: "exhausted"}}
}

func TestMeterFreshTurn_BlockSurfacesGateError(t *testing.T) {
	meter := &fakeGuideMeter{err: quotaGateError()}
	err := allowanceManager(meter, time.Now()).MeterFreshTurn(context.Background(), uuid.New())
	var gateErr billingpkg.GateError
	if !errors.As(err, &gateErr) {
		t.Fatalf("err = %v, want billing GateError (handler maps it to a pre-SSE 402)", err)
	}
	if meter.calls != 1 {
		t.Fatalf("consume called %d times, want 1 (a fresh turn always meters)", meter.calls)
	}
}

func TestMeterFreshTurn_AllowedConsumesOnce(t *testing.T) {
	meter := &fakeGuideMeter{}
	if err := allowanceManager(meter, time.Now()).MeterFreshTurn(context.Background(), uuid.New()); err != nil {
		t.Fatalf("MeterFreshTurn: %v", err)
	}
	if meter.calls != 1 {
		t.Fatalf("consume called %d times, want 1", meter.calls)
	}
}

func TestMeterConfirmationResolve_GenuineApproveCounts(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	meter := &fakeGuideMeter{}
	pc := repository.VibeEvalPendingConfirmation{Status: "pending", PayloadHash: "h", ExpiresAt: now.Add(time.Hour)}
	if err := allowanceManager(meter, now).MeterConfirmationResolve(context.Background(), uuid.New(), pc, "h", true); err != nil {
		t.Fatalf("MeterConfirmationResolve: %v", err)
	}
	if meter.calls != 1 {
		t.Fatalf("consume called %d times, want 1 (genuine approve counts)", meter.calls)
	}
}

func TestMeterConfirmationResolve_DenyNeverCounts(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	meter := &fakeGuideMeter{}
	pc := repository.VibeEvalPendingConfirmation{Status: "pending", PayloadHash: "h", ExpiresAt: now.Add(time.Hour)}
	if err := allowanceManager(meter, now).MeterConfirmationResolve(context.Background(), uuid.New(), pc, "h", false); err != nil {
		t.Fatalf("deny meter: %v", err)
	}
	if meter.calls != 0 {
		t.Fatalf("consume called %d times on deny, want 0 (deny is always allowed and uncounted)", meter.calls)
	}
}

func TestMeterConfirmationResolve_InvalidApproveUncounted(t *testing.T) {
	now := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	cases := map[string]repository.VibeEvalPendingConfirmation{
		"wrong_hash":       {Status: "pending", PayloadHash: "real", ExpiresAt: now.Add(time.Hour)},
		"expired":          {Status: "pending", PayloadHash: "h", ExpiresAt: now.Add(-time.Minute)},
		"already_resolved": {Status: "succeeded", PayloadHash: "h", ExpiresAt: now.Add(time.Hour)},
		"executing":        {Status: "executing", PayloadHash: "h", ExpiresAt: now.Add(time.Hour)},
	}
	for name, pc := range cases {
		t.Run(name, func(t *testing.T) {
			meter := &fakeGuideMeter{}
			// approve=true with the presented hash "h"; only "wrong_hash" presents a non-matching pc hash.
			if err := allowanceManager(meter, now).MeterConfirmationResolve(context.Background(), uuid.New(), pc, "h", true); err != nil {
				t.Fatalf("invalid-resolve meter: %v", err)
			}
			if meter.calls != 0 {
				t.Fatalf("consume called %d times for %s, want 0 (invalid resolves never burn quota)", meter.calls, name)
			}
		})
	}
}
