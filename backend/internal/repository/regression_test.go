package repository_test

import (
	"context"
	"errors"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/agentclash/agentclash/backend/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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

func TestRepositoryRegressionSuiteListBatchesCaseCounts(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	secondChallengeIdentityID := lookupChallengeIdentityID(t, ctx, db, "second-ticket")

	createSuite := func(name string) repository.RegressionSuite {
		t.Helper()
		suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
			WorkspaceID:           fixture.workspaceID,
			SourceChallengePackID: sourceChallengePackID,
			Name:                  name,
			Description:           "Seed suite for batched count coverage",
			Status:                domain.RegressionSuiteStatusActive,
			SourceMode:            "derived_only",
			DefaultGateSeverity:   domain.RegressionSeverityWarning,
			CreatedByUserID:       fixture.userID,
		})
		if err != nil {
			t.Fatalf("CreateRegressionSuite(%q) returned error: %v", name, err)
		}
		return suite
	}

	createCase := func(suiteID uuid.UUID, challengeIdentityID uuid.UUID, key string) {
		t.Helper()
		if _, err := repo.CreateRegressionCase(ctx, repository.CreateRegressionCaseParams{
			SuiteID:                      suiteID,
			Title:                        "Regression " + key,
			Description:                  "Case seeded for batched count coverage",
			Status:                       domain.RegressionCaseStatusActive,
			Severity:                     domain.RegressionSeverityWarning,
			PromotionMode:                domain.RegressionPromotionModeOutputOnly,
			SourceRunID:                  &fixture.runID,
			SourceRunAgentID:             &fixture.primaryRunAgentID,
			SourceChallengePackVersionID: fixture.challengePackVersionID,
			SourceChallengeInputSetID:    &fixture.challengeInputSetID,
			SourceChallengeIdentityID:    challengeIdentityID,
			SourceCaseKey:                key,
			EvidenceTier:                 "native_structured",
			FailureClass:                 "behavioral_regression",
			FailureSummary:               "Seed failure",
			PayloadSnapshot:              []byte(`{"input":"fixture"}`),
			ExpectedContract:             []byte(`{"expected":"pass"}`),
			Metadata:                     []byte(`{"source":"test"}`),
		}); err != nil {
			t.Fatalf("CreateRegressionCase(%q) returned error: %v", key, err)
		}
	}

	oneCaseSuite := createSuite("One case")
	twoCaseSuite := createSuite("Two cases")
	emptySuite := createSuite("No cases")
	createCase(oneCaseSuite.ID, fixture.firstChallengeIdentityID, "one-a")
	createCase(twoCaseSuite.ID, fixture.firstChallengeIdentityID, "two-a")
	createCase(twoCaseSuite.ID, secondChallengeIdentityID, "two-b")

	listed, err := repo.ListRegressionSuitesByWorkspaceID(ctx, fixture.workspaceID, 20, 0)
	if err != nil {
		t.Fatalf("ListRegressionSuitesByWorkspaceID returned error: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("listed suite count = %d, want 3", len(listed))
	}
	countsByName := make(map[string]int, len(listed))
	for _, suite := range listed {
		countsByName[suite.Name] = suite.CaseCount
	}
	for name, want := range map[string]int{
		oneCaseSuite.Name: 1,
		twoCaseSuite.Name: 2,
		emptySuite.Name:   0,
	} {
		if got := countsByName[name]; got != want {
			t.Fatalf("%s case_count = %d, want %d", name, got, want)
		}
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

	active := domain.RegressionCaseStatusActive
	workspaceCases, err := repo.ListRegressionCasesByWorkspaceID(ctx, repository.ListRegressionCasesByWorkspaceIDParams{
		WorkspaceID: fixture.workspaceID,
		Status:      &active,
		Limit:       10,
		Offset:      0,
	})
	if err != nil {
		t.Fatalf("ListRegressionCasesByWorkspaceID returned error: %v", err)
	}
	if len(workspaceCases) != 1 || workspaceCases[0].ID != regressionCase.ID {
		t.Fatalf("workspace cases = %+v, want active case %s", workspaceCases, regressionCase.ID)
	}
	activeCount, err := repo.CountRegressionCasesByWorkspaceID(ctx, fixture.workspaceID, &active)
	if err != nil {
		t.Fatalf("CountRegressionCasesByWorkspaceID returned error: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("active workspace case count = %d, want 1", activeCount)
	}
	proposed := domain.RegressionCaseStatusProposed
	proposedCount, err := repo.CountRegressionCasesByWorkspaceID(ctx, fixture.workspaceID, &proposed)
	if err != nil {
		t.Fatalf("CountRegressionCasesByWorkspaceID(proposed) returned error: %v", err)
	}
	if proposedCount != 0 {
		t.Fatalf("proposed workspace case count = %d, want 0", proposedCount)
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

func TestRepositoryRegressionCaseValidationStats(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	fixture := seedFixture(t, ctx, db)
	repo := repository.New(db)

	sourceChallengePackID := lookupChallengePackID(t, ctx, db, fixture.challengePackVersionID)
	suite, err := repo.CreateRegressionSuite(ctx, repository.CreateRegressionSuiteParams{
		WorkspaceID:           fixture.workspaceID,
		SourceChallengePackID: sourceChallengePackID,
		Name:                  "Validation regressions",
		Description:           "Seed suite for validation stats",
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
		Title:                        "Validation case",
		Status:                       domain.RegressionCaseStatusActive,
		Severity:                     domain.RegressionSeverityBlocking,
		PromotionMode:                domain.RegressionPromotionModeFullExecutable,
		SourceChallengePackVersionID: fixture.challengePackVersionID,
		SourceChallengeInputSetID:    &fixture.challengeInputSetID,
		SourceChallengeIdentityID:    fixture.firstChallengeIdentityID,
		SourceCaseKey:                "case-a",
		EvidenceTier:                 "native_structured",
		FailureClass:                 "policy_violation",
		FailureSummary:               "Original failure",
		PayloadSnapshot:              []byte(`{"input":"a"}`),
		ExpectedContract:             []byte(`{"validator":"exact"}`),
		Metadata:                     []byte(`{"source":"test"}`),
	})
	if err != nil {
		t.Fatalf("CreateRegressionCase returned error: %v", err)
	}

	evaluationSpecID := insertEvaluationSpecRecord(t, ctx, db, fixture.challengePackVersionID, "regression-validation", 1)
	baseTime := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	for index, verdict := range []string{"fail", "fail", "pass", "fail", "pass"} {
		insertRegressionValidationRun(t, ctx, db, repo, fixture, evaluationSpecID, regressionCase.ID, fixture.firstChallengeIdentityID, verdict, baseTime.Add(time.Duration(index)*time.Minute))
	}

	got, err := repo.GetRegressionCaseByID(ctx, regressionCase.ID)
	if err != nil {
		t.Fatalf("GetRegressionCaseByID returned error: %v", err)
	}
	assertValidationStats(t, got.ValidationStats, 5, 3, 2, 0.6, "pass", baseTime.Add(4*time.Minute))

	cases, err := repo.ListRegressionCasesBySuiteID(ctx, suite.ID)
	if err != nil {
		t.Fatalf("ListRegressionCasesBySuiteID returned error: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("cases = %d, want 1", len(cases))
	}
	assertValidationStats(t, cases[0].ValidationStats, 5, 3, 2, 0.6, "pass", baseTime.Add(4*time.Minute))
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

func insertRegressionValidationRun(
	t *testing.T,
	ctx context.Context,
	db *pgxpool.Pool,
	repo *repository.Repository,
	fixture testFixture,
	evaluationSpecID uuid.UUID,
	regressionCaseID uuid.UUID,
	challengeIdentityID uuid.UUID,
	verdict string,
	finishedAt time.Time,
) {
	t.Helper()

	run, runAgents := createTestRun(t, ctx, repo, fixture, 1, "regression-validation")
	runAgentID := runAgents[0].ID
	if _, err := db.Exec(ctx, `
		UPDATE runs
		SET status = 'completed',
		    started_at = $2,
		    finished_at = $2
		WHERE id = $1
	`, run.ID, finishedAt); err != nil {
		t.Fatalf("mark validation run completed returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO run_case_selections (
			id,
			run_id,
			challenge_identity_id,
			selection_origin,
			regression_case_id,
			selection_rank
		)
		VALUES ($1, $2, $3, 'regression_case', $4, 1)
	`, uuid.New(), run.ID, challengeIdentityID, regressionCaseID); err != nil {
		t.Fatalf("insert validation run selection returned error: %v", err)
	}

	if _, err := db.Exec(ctx, `
		INSERT INTO run_scorecards (
			id,
			run_id,
			evaluation_spec_id,
			winning_run_agent_id,
			scorecard
		)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), run.ID, evaluationSpecID, runAgentID, []byte(`{"status":"complete"}`)); err != nil {
		t.Fatalf("insert validation run scorecard returned error: %v", err)
	}

	score := 1.0
	if verdict == "fail" {
		score = 0
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO judge_results (
			id,
			run_agent_id,
			evaluation_spec_id,
			challenge_identity_id,
			regression_case_id,
			judge_key,
			verdict,
			normalized_score,
			raw_output
		)
		VALUES ($1, $2, $3, $4, $5, 'exact', $6, $7, $8)
	`, uuid.New(), runAgentID, evaluationSpecID, challengeIdentityID, regressionCaseID, verdict, score, []byte(`{"state":"available"}`)); err != nil {
		t.Fatalf("insert validation judge result returned error: %v", err)
	}
}

func assertValidationStats(
	t *testing.T,
	stats *repository.RegressionCaseValidationStats,
	runCount int,
	failureCount int,
	passCount int,
	reproductionRate float64,
	lastOutcome string,
	lastValidatedAt time.Time,
) {
	t.Helper()

	if stats == nil {
		t.Fatal("validation stats = nil, want populated")
	}
	if stats.RunCount != runCount || stats.FailureCount != failureCount || stats.PassCount != passCount {
		t.Fatalf("validation counts = %+v, want runs=%d failures=%d passes=%d", stats, runCount, failureCount, passCount)
	}
	if math.Abs(stats.ReproductionRate-reproductionRate) > 1e-9 {
		t.Fatalf("reproduction rate = %f, want %f", stats.ReproductionRate, reproductionRate)
	}
	if stats.LastOutcome != lastOutcome {
		t.Fatalf("last outcome = %q, want %q", stats.LastOutcome, lastOutcome)
	}
	if stats.LastValidatedAt == nil || !stats.LastValidatedAt.Equal(lastValidatedAt) {
		t.Fatalf("last validated at = %v, want %s", stats.LastValidatedAt, lastValidatedAt)
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
