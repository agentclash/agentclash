package budget

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mockRepository implements Repository for testing using in-memory maps.
type mockRepository struct {
	policies     map[uuid.UUID]SpendPolicy
	windowSpends map[uuid.UUID]WindowSpend // keyed by spendPolicyID

	createdSummaries []CreateRunCostSummaryParams
	upsertedSpends   []UpsertWindowSpendParams
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		policies:     make(map[uuid.UUID]SpendPolicy),
		windowSpends: make(map[uuid.UUID]WindowSpend),
	}
}

func (m *mockRepository) GetSpendPolicyByID(_ context.Context, id uuid.UUID) (SpendPolicy, error) {
	p, ok := m.policies[id]
	if !ok {
		return SpendPolicy{}, ErrPolicyNotFound
	}
	return p, nil
}

func (m *mockRepository) GetWindowSpend(_ context.Context, spendPolicyID uuid.UUID, _, _ time.Time) (WindowSpend, error) {
	ws, ok := m.windowSpends[spendPolicyID]
	if !ok {
		return WindowSpend{}, nil // no spend yet
	}
	return ws, nil
}

func (m *mockRepository) UpsertWindowSpend(_ context.Context, params UpsertWindowSpendParams) error {
	m.upsertedSpends = append(m.upsertedSpends, params)
	return nil
}

func (m *mockRepository) CreateRunCostSummary(_ context.Context, params CreateRunCostSummaryParams) error {
	m.createdSummaries = append(m.createdSummaries, params)
	return nil
}

func ptr(f float64) *float64 { return &f }

func fixedNow() time.Time {
	return time.Date(2024, 3, 15, 14, 0, 0, 0, time.UTC)
}

func newTestChecker(repo *mockRepository) *Checker {
	c := NewChecker(repo)
	c.now = fixedNow
	return c
}

func TestCheckPreRunBudget_NoPolicyFound(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected Allowed=true when policy not found")
	}
}

func TestCheckPreRunBudget_NoLimits(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "month",
		SoftLimit:    nil,
		HardLimit:    nil,
		CurrencyCode: "USD",
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected Allowed=true when no limits set")
	}
	if result.SoftLimitHit {
		t.Error("expected SoftLimitHit=false when no limits set")
	}
	if result.RemainingBudget != nil {
		t.Error("expected RemainingBudget=nil when no hard limit")
	}
}

func TestCheckPreRunBudget_UnderHardLimit(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "month",
		HardLimit:    ptr(100.0),
		CurrencyCode: "USD",
	}
	repo.windowSpends[policyID] = WindowSpend{
		TotalCostUSD: 50.0,
		RunCount:     5,
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected Allowed=true when under hard limit")
	}
	if result.CurrentSpend != 50.0 {
		t.Errorf("CurrentSpend = %v, want 50.0", result.CurrentSpend)
	}
	if result.RemainingBudget == nil || *result.RemainingBudget != 50.0 {
		t.Errorf("RemainingBudget = %v, want 50.0", result.RemainingBudget)
	}
}

func TestCheckPreRunBudget_AtHardLimit(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "month",
		HardLimit:    ptr(100.0),
		CurrencyCode: "USD",
	}
	repo.windowSpends[policyID] = WindowSpend{
		TotalCostUSD: 100.0,
		RunCount:     10,
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected Allowed=false when at hard limit")
	}
	if result.RemainingBudget == nil || *result.RemainingBudget != 0.0 {
		t.Errorf("RemainingBudget = %v, want 0.0", result.RemainingBudget)
	}
}

func TestCheckPreRunBudget_OverHardLimit(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "day",
		HardLimit:    ptr(50.0),
		CurrencyCode: "USD",
	}
	repo.windowSpends[policyID] = WindowSpend{
		TotalCostUSD: 75.0,
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Allowed {
		t.Error("expected Allowed=false when over hard limit")
	}
}

func TestCheckPreRunBudget_SoftLimitHit(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "week",
		SoftLimit:    ptr(80.0),
		HardLimit:    ptr(100.0),
		CurrencyCode: "USD",
	}
	repo.windowSpends[policyID] = WindowSpend{
		TotalCostUSD: 85.0,
		RunCount:     8,
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected Allowed=true when only soft limit hit")
	}
	if !result.SoftLimitHit {
		t.Error("expected SoftLimitHit=true")
	}
	if result.RemainingBudget == nil || *result.RemainingBudget != 15.0 {
		t.Errorf("RemainingBudget = %v, want 15.0", result.RemainingBudget)
	}
}

func TestCheckPreRunBudget_RunWindowKind(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "run",
		HardLimit:    ptr(10.0),
		CurrencyCode: "USD",
	}

	result, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Allowed {
		t.Error("expected Allowed=true for per-run window kind")
	}
}

func TestCheckPreRunBudget_RepositoryError(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	// Use a policyID that exists but will cause GetWindowSpend to fail.
	policyID := uuid.New()
	repo.policies[policyID] = SpendPolicy{
		ID:           policyID,
		WorkspaceID:  uuid.New(),
		WindowKind:   "month",
		HardLimit:    ptr(100.0),
		CurrencyCode: "USD",
	}

	// Replace the repo with one that returns an error from GetWindowSpend.
	errRepo := &errorRepository{
		mockRepository:   repo,
		getWindowSpendErr: errors.New("db connection failed"),
	}
	c.repo = errRepo

	_, err := c.CheckPreRunBudget(context.Background(), uuid.New(), policyID)
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

func TestRecordRunCost_CreatesAndUpserts(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	runID := uuid.New()
	orgID := uuid.New()
	wsID := uuid.New()
	policyID := uuid.New()

	params := RecordRunCostParams{
		RunID:             runID,
		OrganizationID:    orgID,
		WorkspaceID:       wsID,
		SpendPolicyID:     policyID,
		WindowKind:        "month",
		TotalCostUSD:      12.50,
		TotalInputTokens:  1000,
		TotalOutputTokens: 500,
		CostBreakdown:     []byte(`{"model":"gpt-4"}`),
	}

	err := c.RecordRunCost(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary was created.
	if len(repo.createdSummaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(repo.createdSummaries))
	}
	s := repo.createdSummaries[0]
	if s.RunID != runID {
		t.Errorf("summary RunID = %v, want %v", s.RunID, runID)
	}
	if s.TotalCostUSD != 12.50 {
		t.Errorf("summary TotalCostUSD = %v, want 12.50", s.TotalCostUSD)
	}

	// Verify window spend was upserted.
	if len(repo.upsertedSpends) != 1 {
		t.Fatalf("expected 1 upserted spend, got %d", len(repo.upsertedSpends))
	}
	u := repo.upsertedSpends[0]
	if u.SpendPolicyID != policyID {
		t.Errorf("upsert SpendPolicyID = %v, want %v", u.SpendPolicyID, policyID)
	}
	if u.CostUSD != 12.50 {
		t.Errorf("upsert CostUSD = %v, want 12.50", u.CostUSD)
	}
	if u.RunID != runID {
		t.Errorf("upsert RunID = %v, want %v", u.RunID, runID)
	}

	// Verify window bounds are for March 2024.
	wantStart := time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	if !u.WindowStart.Equal(wantStart) {
		t.Errorf("upsert WindowStart = %v, want %v", u.WindowStart, wantStart)
	}
	if !u.WindowEnd.Equal(wantEnd) {
		t.Errorf("upsert WindowEnd = %v, want %v", u.WindowEnd, wantEnd)
	}
}

func TestRecordRunCost_RunWindowSkipsUpsert(t *testing.T) {
	repo := newMockRepository()
	c := newTestChecker(repo)

	params := RecordRunCostParams{
		RunID:             uuid.New(),
		OrganizationID:    uuid.New(),
		WorkspaceID:       uuid.New(),
		SpendPolicyID:     uuid.New(),
		WindowKind:        "run",
		TotalCostUSD:      5.00,
		TotalInputTokens:  500,
		TotalOutputTokens: 250,
		CostBreakdown:     []byte(`{}`),
	}

	err := c.RecordRunCost(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.createdSummaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(repo.createdSummaries))
	}
	if len(repo.upsertedSpends) != 0 {
		t.Errorf("expected 0 upserted spends for 'run' window, got %d", len(repo.upsertedSpends))
	}
}

// errorRepository wraps mockRepository but returns errors from specific methods.
type errorRepository struct {
	*mockRepository
	getWindowSpendErr error
}

func (e *errorRepository) GetWindowSpend(_ context.Context, _ uuid.UUID, _, _ time.Time) (WindowSpend, error) {
	return WindowSpend{}, e.getWindowSpendErr
}
