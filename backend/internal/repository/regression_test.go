package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestRepositoryRegressionSuiteCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)

	created, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Seed suite for CRUD coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	got, err := repo.GetRegressionSuiteByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetRegressionSuiteByID returned error: %v", err)
	}
	if got.Name != "Critical regressions" {
		t.Fatalf("suite name = %q, want Critical regressions", got.Name)
	}

	listed, err := repo.ListRegressionSuitesByWorkspaceID(ctx, fixture.workspaceID, 20, 0)
	if err != nil {
		t.Fatalf("ListRegressionSuitesByWorkspaceID returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("suite count = %d, want 1", len(listed))
	}

	description := "Renamed suite"
	severity := domain.RegressionSeverityBlocking
	archived := domain.RegressionSuiteStatusArchived
	updated, err := repo.PatchRegressionSuite(ctx, repository.PatchRegressionSuiteParams{
		ID:                  created.ID,
		Description:         &description,
		Status:              &archived,
		DefaultGateSeverity: &severity,
	})
	if err != nil {
		t.Fatalf("PatchRegressionSuite returned error: %v", err)
	}
	if updated.Status != domain.RegressionSuiteStatusArchived {
		t.Fatalf("suite status = %s, want archived", updated.Status)
	}
	if updated.DefaultGateSeverity != domain.RegressionSeverityBlocking {
		t.Fatalf("default gate severity = %s, want blocking", updated.DefaultGateSeverity)
	}
}

func TestRepositoryRegressionCaseAndPromotionCRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Seed suite for case coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	regressionCase, err := repo.CreateRegressionCase(ctx, repository.CreateRegressionCaseParams{
		SuiteID:                      suite.ID,
		Title:                        "Support case regression",
		Description:                  "Case seeded from a failed run",
		Status:                       domain.RegressionCaseStatusActive,
		Severity:                     domain.RegressionSeverityBlocking,
		PromotionMode:                domain.RegressionPromotionModeFullExecutable,
		SourceRunID:                  &fixture.runID,
		SourceRunAgentID:             &fixture.primaryRunAgentID,
		SourceChallengePackVersionID: fixture.challengePackVersionID,
		SourceChallengeInputSetID:    &fixture.challengeInputSetID,
		SourceChallengeIdentityID:    fixture.firstChallengeIdentityID,
		SourceCaseKey:                "case-1",
		EvidenceTier:                 "replay",
		FailureClass:                 "behavioral_regression",
		FailureSummary:               "Model regressed on reply quality",
		PayloadSnapshot:              []byte(`{"replay_id":"snapshot"}`),
		ExpectedContract:             []byte(`{"outcome":"pass"}`),
		Metadata:                     []byte(`{"source":"test"}`),
	})
	if err != nil {
		t.Fatalf("CreateRegressionCase returned error: %v", err)
	}
	if regressionCase.WorkspaceID != fixture.workspaceID {
		t.Fatalf("case workspace id = %s, want %s", regressionCase.WorkspaceID, fixture.workspaceID)
	}

	got, err := repo.GetRegressionCaseByID(ctx, regressionCase.ID)
	if err != nil {
		t.Fatalf("GetRegressionCaseByID returned error: %v", err)
	}
	if got.Title != "Support case regression" {
		t.Fatalf("case title = %q, want Support case regression", got.Title)
	}

	cases, err := repo.ListRegressionCasesBySuiteID(ctx, suite.ID)
	if err != nil {
		t.Fatalf("ListRegressionCasesBySuiteID returned error: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("case count = %d, want 1", len(cases))
	}

	title := "Muted support case regression"
	muted := domain.RegressionCaseStatusMuted
	severity := domain.RegressionSeverityWarning
	updated, err := repo.PatchRegressionCase(ctx, repository.PatchRegressionCaseParams{
		ID:       regressionCase.ID,
		Title:    &title,
		Status:   &muted,
		Severity: &severity,
	})
	if err != nil {
		t.Fatalf("PatchRegressionCase returned error: %v", err)
	}
	if updated.Status != domain.RegressionCaseStatusMuted {
		t.Fatalf("case status = %s, want muted", updated.Status)
	}
	if updated.Severity != domain.RegressionSeverityWarning {
		t.Fatalf("case severity = %s, want warning", updated.Severity)
	}

	promotion, err := repo.CreateRegressionPromotion(ctx, repository.CreateRegressionPromotionParams{
		WorkspaceRegressionCaseID: regressionCase.ID,
		SourceRunID:               fixture.runID,
		SourceRunAgentID:          fixture.primaryRunAgentID,
		SourceEventRefs:           []byte(`["event-1"]`),
		PromotedByUserID:          fixture.userID,
		PromotionReason:           "Carry regression into suite",
		PromotionSnapshot:         []byte(`{"from":"test"}`),
	})
	if err != nil {
		t.Fatalf("CreateRegressionPromotion returned error: %v", err)
	}
	if promotion.WorkspaceRegressionCaseID != regressionCase.ID {
		t.Fatalf("promotion case id = %s, want %s", promotion.WorkspaceRegressionCaseID, regressionCase.ID)
	}
}

func TestRepositoryPatchRegressionSuiteRejectsInvalidTransition(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Seed suite for invalid transition coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	active := domain.RegressionSuiteStatusActive
	_, err = repo.PatchRegressionSuite(ctx, repository.PatchRegressionSuiteParams{
		ID:     suite.ID,
		Status: &active,
	})
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("PatchRegressionSuite error = %v, want ErrInvalidTransition", err)
	}
}

func TestRepositoryRegressionSuiteNameCanBeReusedAfterArchive(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	original, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Original suite",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	archived := domain.RegressionSuiteStatusArchived
	if _, err := repo.PatchRegressionSuite(ctx, repository.PatchRegressionSuiteParams{
		ID:     original.ID,
		Status: &archived,
	}); err != nil {
		t.Fatalf("PatchRegressionSuite archived transition returned error: %v", err)
	}

	recreated, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Replacement active suite",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite reuse-after-archive returned error: %v", err)
	}
	if recreated.ID == original.ID {
		t.Fatal("expected a new suite record when reusing the archived name")
	}
}

func TestRepositoryPatchRegressionCaseRejectsInvalidTransition(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Seed suite for invalid case transition coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	regressionCase, err := repo.CreateRegressionCase(ctx, repository.CreateRegressionCaseParams{
		SuiteID:                      suite.ID,
		Title:                        "Support case regression",
		Description:                  "Case seeded from a failed run",
		Status:                       domain.RegressionCaseStatusActive,
		Severity:                     domain.RegressionSeverityBlocking,
		PromotionMode:                domain.RegressionPromotionModeFullExecutable,
		SourceRunID:                  &fixture.runID,
		SourceRunAgentID:             &fixture.primaryRunAgentID,
		SourceChallengePackVersionID: fixture.challengePackVersionID,
		SourceChallengeInputSetID:    &fixture.challengeInputSetID,
		SourceChallengeIdentityID:    fixture.firstChallengeIdentityID,
		SourceCaseKey:                "case-1",
		EvidenceTier:                 "replay",
		FailureClass:                 "behavioral_regression",
		FailureSummary:               "Model regressed on reply quality",
	})
	if err != nil {
		t.Fatalf("CreateRegressionCase returned error: %v", err)
	}

	archived := domain.RegressionCaseStatusArchived
	if _, err := repo.PatchRegressionCase(ctx, repository.PatchRegressionCaseParams{
		ID:     regressionCase.ID,
		Status: &archived,
	}); err != nil {
		t.Fatalf("PatchRegressionCase archived transition returned error: %v", err)
	}

	active := domain.RegressionCaseStatusActive
	_, err = repo.PatchRegressionCase(ctx, repository.PatchRegressionCaseParams{
		ID:     regressionCase.ID,
		Status: &active,
	})
	if err == nil {
		t.Fatal("expected invalid transition error")
	}
	if !errors.Is(err, repository.ErrInvalidTransition) {
		t.Fatalf("PatchRegressionCase error = %v, want ErrInvalidTransition", err)
	}
}

func TestRepositoryPromoteFailureFreezesContextAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Critical regressions",
		Description:           "Seed suite for promotion coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "promote-failure", 1)
	insertFailureReviewScorecard(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, map[string]any{
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": "available", "score": 0.2},
		},
	})

	result, err := repo.PromoteFailure(ctx, repository.PromoteFailureParams{
		SuiteID:             suite.ID,
		RunID:               fixture.runID,
		RunAgentID:          fixture.primaryRunAgentID,
		ChallengeIdentityID: fixture.firstChallengeIdentityID,
		Title:               "Filesystem policy regression",
		FailureSummary:      "Policy guard tripped",
		Severity:            domain.RegressionSeverityBlocking,
		PromotionMode:       domain.RegressionPromotionModeFullExecutable,
		FailureClass:        "policy_violation",
		EvidenceTier:        "native_structured",
		SourceCaseKey:       "prompt.txt",
		SourceItemKey:       stringPtr("prompt.txt"),
		ExpectedContract:    []byte(`{"scorecard":{"dimensions":["correctness"]}}`),
		ValidatorOverrides:  []byte(`{"judge_threshold_overrides":{"policy.filesystem":1}}`),
		Metadata:            []byte(`{"source":"test"}`),
		SourceEventRefs:     []byte(`[{"sequence_number":2,"event_type":"system.output.finalized","kind":"run_event"}]`),
		PromotionSnapshot:   []byte(`{"from":"failure_review"}`),
		PromotedByUserID:    fixture.userID,
	})
	if err != nil {
		t.Fatalf("PromoteFailure returned error: %v", err)
	}
	if !result.Created {
		t.Fatal("Created = false, want true on first promotion")
	}
	if result.Case.SourceChallengeInputSetID == nil || *result.Case.SourceChallengeInputSetID != fixture.challengeInputSetID {
		t.Fatalf("source challenge input set id = %v, want %s", result.Case.SourceChallengeInputSetID, fixture.challengeInputSetID)
	}
	if string(result.Case.PayloadSnapshot) != `{"content":"Customer one is blocked"}` {
		t.Fatalf("payload snapshot = %s, want frozen case payload", result.Case.PayloadSnapshot)
	}

	if string(result.Case.ExpectedContract) != `{"scorecard":{"dimensions":["correctness"]}}` {
		t.Fatalf("expected contract = %s, want manager-supplied frozen contract", result.Case.ExpectedContract)
	}

	second, err := repo.PromoteFailure(ctx, repository.PromoteFailureParams{
		SuiteID:             suite.ID,
		RunID:               fixture.runID,
		RunAgentID:          fixture.primaryRunAgentID,
		ChallengeIdentityID: fixture.firstChallengeIdentityID,
		Title:               "Filesystem policy regression",
		FailureSummary:      "Policy guard tripped",
		Severity:            domain.RegressionSeverityBlocking,
		PromotionMode:       domain.RegressionPromotionModeFullExecutable,
		FailureClass:        "policy_violation",
		EvidenceTier:        "native_structured",
		SourceCaseKey:       "prompt.txt",
		SourceItemKey:       stringPtr("prompt.txt"),
		ExpectedContract:    []byte(`{"scorecard":{"dimensions":["correctness"]}}`),
		PromotedByUserID:    fixture.userID,
	})
	if err != nil {
		t.Fatalf("second PromoteFailure returned error: %v", err)
	}
	if second.Created {
		t.Fatal("Created = true, want false on idempotent second promotion")
	}
	if second.Case.ID != result.Case.ID {
		t.Fatalf("second case id = %s, want %s", second.Case.ID, result.Case.ID)
	}

	var caseCount, promotionCount int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM workspace_regression_cases WHERE suite_id = $1
	`, suite.ID).Scan(&caseCount); err != nil {
		t.Fatalf("count regression cases returned error: %v", err)
	}
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM workspace_regression_promotions WHERE workspace_regression_case_id = $1
	`, result.Case.ID).Scan(&promotionCount); err != nil {
		t.Fatalf("count regression promotions returned error: %v", err)
	}
	if caseCount != 1 {
		t.Fatalf("case count = %d, want 1", caseCount)
	}
	if promotionCount != 1 {
		t.Fatalf("promotion count = %d, want 1", promotionCount)
	}
}

func TestRepositoryPromoteFailureConcurrentRequestsStayIdempotent(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Concurrent regressions",
		Description:           "Seed suite for duplicate promotion coverage",
		Status:                domain.RegressionSuiteStatusActive,
		SourceMode:            "derived_only",
		DefaultGateSeverity:   domain.RegressionSeverityWarning,
		CreatedByUserID:       fixture.userID,
	})
	if err != nil {
		t.Fatalf("CreateRegressionSuite returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "promote-failure-concurrent", 1)
	insertFailureReviewScorecard(t, ctx, db, fixture.primaryRunAgentID, evaluationSpecID, map[string]any{
		"dimensions": map[string]any{
			"correctness": map[string]any{"state": "available", "score": 0.2},
		},
	})

	params := repository.PromoteFailureParams{
		SuiteID:             suite.ID,
		RunID:               fixture.runID,
		RunAgentID:          fixture.primaryRunAgentID,
		ChallengeIdentityID: fixture.firstChallengeIdentityID,
		Title:               "Filesystem policy regression",
		FailureSummary:      "Policy guard tripped",
		Severity:            domain.RegressionSeverityBlocking,
		PromotionMode:       domain.RegressionPromotionModeFullExecutable,
		FailureClass:        "policy_violation",
		EvidenceTier:        "native_structured",
		SourceCaseKey:       "prompt.txt",
		SourceItemKey:       stringPtr("prompt.txt"),
		ExpectedContract:    []byte(`{"scorecard":{"dimensions":["correctness"]}}`),
		PromotedByUserID:    fixture.userID,
	}

	results := make([]repository.PromoteFailureResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = repo.PromoteFailure(ctx, params)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("PromoteFailure[%d] returned error: %v", i, err)
		}
	}
	if results[0].Case.ID != results[1].Case.ID {
		t.Fatalf("case ids = %s and %s, want the same id", results[0].Case.ID, results[1].Case.ID)
	}
	createdCount := 0
	for _, result := range results {
		if result.Created {
			createdCount++
		}
	}
	if createdCount != 1 {
		t.Fatalf("created count = %d, want 1", createdCount)
	}

	var caseCount, promotionCount int
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM workspace_regression_cases WHERE suite_id = $1
	`, suite.ID).Scan(&caseCount); err != nil {
		t.Fatalf("count regression cases returned error: %v", err)
	}
	if err := db.QueryRow(ctx, `
		SELECT count(*) FROM workspace_regression_promotions WHERE workspace_regression_case_id = $1
	`, results[0].Case.ID).Scan(&promotionCount); err != nil {
		t.Fatalf("count regression promotions returned error: %v", err)
	}
	if caseCount != 1 {
		t.Fatalf("case count = %d, want 1", caseCount)
	}
	if promotionCount != 1 {
		t.Fatalf("promotion count = %d, want 1", promotionCount)
	}
}

func lookupChallengePackID(t *testing.T, ctx context.Context, db testQuerier, challengePackVersionID uuid.UUID) uuid.UUID {
	t.Helper()

	var challengePackID uuid.UUID
	if err := db.QueryRow(ctx, `
		SELECT challenge_pack_id
		FROM challenge_pack_versions
		WHERE id = $1
	`, challengePackVersionID).Scan(&challengePackID); err != nil {
		t.Fatalf("lookup challenge_pack_id returned error: %v", err)
	}
	return challengePackID
}

type testQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
