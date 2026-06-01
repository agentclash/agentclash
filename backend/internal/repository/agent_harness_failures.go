package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentclash/agentclash/backend/internal/failurereview"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	AgentHarnessFailureClassSetup                    = "setup"
	AgentHarnessFailureClassAuth                     = "auth"
	AgentHarnessFailureClassToolMisuse               = "tool_misuse"
	AgentHarnessFailureClassIncompleteImplementation = "incomplete_implementation"
	AgentHarnessFailureClassNoOpDiff                 = "no_op_diff"
	AgentHarnessFailureClassTestFailure              = "test_failure"
	AgentHarnessFailureClassOverbroadDiff            = "overbroad_diff"
	AgentHarnessFailureClassNoPR                     = "no_pr"
	AgentHarnessFailureClassJudgeFailure             = "judge_failure"
	AgentHarnessFailureClassTimeout                  = "timeout"
	AgentHarnessFailureClassPolicyPrivacy            = "policy_privacy"
	AgentHarnessFailureClassInfrastructure           = "infrastructure"
	AgentHarnessFailureClassNone                     = "none"
	AgentHarnessFailureClassUnknown                  = "unknown"
)

func ValidAgentHarnessFailureClass(class string) bool {
	switch strings.TrimSpace(class) {
	case AgentHarnessFailureClassSetup,
		AgentHarnessFailureClassAuth,
		AgentHarnessFailureClassToolMisuse,
		AgentHarnessFailureClassIncompleteImplementation,
		AgentHarnessFailureClassNoOpDiff,
		AgentHarnessFailureClassTestFailure,
		AgentHarnessFailureClassOverbroadDiff,
		AgentHarnessFailureClassNoPR,
		AgentHarnessFailureClassJudgeFailure,
		AgentHarnessFailureClassTimeout,
		AgentHarnessFailureClassPolicyPrivacy,
		AgentHarnessFailureClassInfrastructure,
		AgentHarnessFailureClassNone,
		AgentHarnessFailureClassUnknown:
		return true
	default:
		return false
	}
}

type AgentHarnessFailureAnnotation struct {
	ID                      uuid.UUID
	AgentHarnessExecutionID uuid.UUID
	SuggestedClass          *string
	SuggestedSummary        string
	SuggestedSource         string
	SuggestedConfidence     *float64
	SuggestedPayload        json.RawMessage
	HumanClass              *string
	HumanSummary            string
	HumanPayload            json.RawMessage
	EditedByUserID          *uuid.UUID
	CreatedAt               time.Time
	UpdatedAt               time.Time
}

type UpsertAgentHarnessFailureAnnotationParams struct {
	ExecutionID         uuid.UUID
	SuggestedClass      *string
	SuggestedSummary    string
	SuggestedSource     string
	SuggestedConfidence *float64
	SuggestedPayload    json.RawMessage
	HumanClass          *string
	HumanSummary        string
	HumanPayload        json.RawMessage
	EditedByUserID      *uuid.UUID
}

type AgentHarnessFailureReview struct {
	ExecutionID         uuid.UUID                     `json:"execution_id"`
	AgentHarnessID      uuid.UUID                     `json:"agent_harness_id"`
	WorkspaceID         uuid.UUID                     `json:"workspace_id"`
	RunID               *uuid.UUID                    `json:"run_id,omitempty"`
	RunAgentID          *uuid.UUID                    `json:"run_agent_id,omitempty"`
	Status              string                        `json:"status"`
	SuggestedClass      string                        `json:"suggested_class"`
	SuggestedSummary    string                        `json:"suggested_summary"`
	SuggestedSource     string                        `json:"suggested_source"`
	SuggestedConfidence float64                       `json:"suggested_confidence"`
	HumanClass          *string                       `json:"human_class,omitempty"`
	HumanSummary        string                        `json:"human_summary,omitempty"`
	EffectiveClass      string                        `json:"effective_class"`
	EffectiveSummary    string                        `json:"effective_summary"`
	Taxonomy            failurereview.FailureTaxonomy `json:"taxonomy"`
	RepositoryURL       string                        `json:"repository_url,omitempty"`
	BaseBranch          string                        `json:"base_branch,omitempty"`
	TaskType            string                        `json:"task_type,omitempty"`
	HarnessName         string                        `json:"harness_name,omitempty"`
	HarnessKind         string                        `json:"harness_kind,omitempty"`
	Model               string                        `json:"model,omitempty"`
	Template            string                        `json:"template,omitempty"`
	SuiteID             *uuid.UUID                    `json:"suite_id,omitempty"`
	SuiteVersionID      *uuid.UUID                    `json:"suite_version_id,omitempty"`
	SuiteTaskID         *uuid.UUID                    `json:"suite_task_id,omitempty"`
	ScorecardPassed     *bool                         `json:"scorecard_passed,omitempty"`
	OverallScore        *float64                      `json:"overall_score,omitempty"`
	EventRefs           []AgentHarnessFailureEventRef `json:"event_refs,omitempty"`
	CreatedAt           time.Time                     `json:"created_at"`
	UpdatedAt           time.Time                     `json:"updated_at"`
}

type AgentHarnessFailureEventRef struct {
	SequenceNumber int64           `json:"sequence_number"`
	EventType      string          `json:"event_type"`
	ActorType      string          `json:"actor_type"`
	ArtifactID     *uuid.UUID      `json:"artifact_id,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
}

type AgentHarnessFailureSummaryGroup struct {
	GroupBy           string     `json:"group_by"`
	Key               string     `json:"key"`
	Label             string     `json:"label"`
	FailureClass      string     `json:"failure_class"`
	Count             int        `json:"count"`
	LatestExecutionID uuid.UUID  `json:"latest_execution_id"`
	LatestAt          time.Time  `json:"latest_at"`
	SuiteID           *uuid.UUID `json:"suite_id,omitempty"`
}

type PromoteAgentHarnessExecutionToSuiteParams struct {
	ExecutionID     uuid.UUID
	SuiteID         uuid.UUID
	CreatedByUserID *uuid.UUID
	Title           string
	PublicPrompt    string
	FailureClass    string
	FailureSummary  string
	Metadata        json.RawMessage
}

type PromoteAgentHarnessExecutionToSuiteResult struct {
	Suite AgentHarnessSuite     `json:"suite"`
	Task  AgentHarnessSuiteTask `json:"task"`
}

func (r *Repository) GetAgentHarnessFailureReview(ctx context.Context, executionID uuid.UUID) (AgentHarnessFailureReview, error) {
	execution, err := r.GetAgentHarnessExecutionByID(ctx, executionID)
	if err != nil {
		return AgentHarnessFailureReview{}, err
	}
	events, err := r.ListAgentHarnessExecutionEvents(ctx, executionID)
	if err != nil {
		return AgentHarnessFailureReview{}, err
	}
	annotation, err := r.GetAgentHarnessFailureAnnotationByExecutionID(ctx, executionID)
	if err != nil && !errors.Is(err, ErrAgentHarnessFailureAnnotationNotFound) {
		return AgentHarnessFailureReview{}, err
	}
	scorecard, scorecardErr := r.agentHarnessFailureScorecard(ctx, execution)
	if scorecardErr != nil {
		return AgentHarnessFailureReview{}, scorecardErr
	}
	return buildAgentHarnessFailureReview(execution, events, annotation, scorecard), nil
}

func (r *Repository) ListAgentHarnessFailureReviewsByWorkspaceID(ctx context.Context, workspaceID uuid.UUID) ([]AgentHarnessFailureReview, error) {
	executions, err := r.ListAgentHarnessExecutions(ctx, ListAgentHarnessExecutionsParams{WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	reviews := make([]AgentHarnessFailureReview, 0)
	for _, execution := range executions {
		if !agentHarnessExecutionHasFailureSignal(execution) {
			scorecard, err := r.agentHarnessFailureScorecard(ctx, execution)
			if err != nil {
				return nil, err
			}
			if scorecard == nil || scorecard.Passed == nil || *scorecard.Passed {
				continue
			}
		}
		review, err := r.GetAgentHarnessFailureReview(ctx, execution.ID)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, review)
	}
	return reviews, nil
}

func BuildAgentHarnessFailureSummaryGroups(reviews []AgentHarnessFailureReview) []AgentHarnessFailureSummaryGroup {
	acc := map[string]*AgentHarnessFailureSummaryGroup{}
	add := func(groupBy, key, label, class string, review AgentHarnessFailureReview) {
		if strings.TrimSpace(key) == "" {
			key = "(unset)"
		}
		if strings.TrimSpace(label) == "" {
			label = key
		}
		mapKey := strings.Join([]string{groupBy, key, class}, "\x00")
		group, ok := acc[mapKey]
		if !ok {
			group = &AgentHarnessFailureSummaryGroup{
				GroupBy:           groupBy,
				Key:               key,
				Label:             label,
				FailureClass:      class,
				LatestExecutionID: review.ExecutionID,
				LatestAt:          review.UpdatedAt,
				SuiteID:           cloneUUIDPtr(review.SuiteID),
			}
			acc[mapKey] = group
		}
		group.Count++
		if review.UpdatedAt.After(group.LatestAt) {
			group.LatestAt = review.UpdatedAt
			group.LatestExecutionID = review.ExecutionID
		}
	}
	for _, review := range reviews {
		class := review.EffectiveClass
		add("repository", review.RepositoryURL, review.RepositoryURL, class, review)
		add("task_type", review.TaskType, review.TaskType, class, review)
		add("harness", review.AgentHarnessID.String(), review.HarnessName, class, review)
		add("model", review.Model, review.Model, class, review)
		add("template", review.Template, review.Template, class, review)
		if review.SuiteID != nil {
			add("suite", review.SuiteID.String(), review.SuiteID.String(), class, review)
		}
	}
	keys := make([]string, 0, len(acc))
	for key := range acc {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	groups := make([]AgentHarnessFailureSummaryGroup, 0, len(keys))
	for _, key := range keys {
		groups = append(groups, *acc[key])
	}
	return groups
}

func agentHarnessExecutionHasFailureSignal(execution AgentHarnessExecution) bool {
	switch execution.Status {
	case string(AgentHarnessExecutionStatusFailed), string(AgentHarnessExecutionStatusCancelled):
		return true
	default:
		return false
	}
}

func (r *Repository) agentHarnessFailureScorecard(ctx context.Context, execution AgentHarnessExecution) (*RunAgentScorecard, error) {
	if execution.RunAgentID == nil {
		return nil, nil
	}
	scorecard, err := r.GetRunAgentScorecardByRunAgentID(ctx, *execution.RunAgentID)
	if err != nil {
		if errors.Is(err, ErrRunAgentScorecardNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("load agent harness scorecard: %w", err)
	}
	return &scorecard, nil
}

func buildAgentHarnessFailureReview(execution AgentHarnessExecution, events []AgentHarnessExecutionEvent, annotation AgentHarnessFailureAnnotation, scorecard *RunAgentScorecard) AgentHarnessFailureReview {
	snapshot := decodeAgentHarnessFailureHarnessSnapshot(execution.HarnessSnapshot)
	eval := decodeAgentHarnessFailureEvaluationConfig(execution.EvaluationConfigSnapshot)
	suggestedClass, suggestedSummary, confidence := classifyAgentHarnessFailure(execution, events, scorecard)
	source := "rules"
	if annotation.SuggestedClass != nil && strings.TrimSpace(*annotation.SuggestedClass) != "" {
		suggestedClass = strings.TrimSpace(*annotation.SuggestedClass)
		suggestedSummary = firstNonEmpty(strings.TrimSpace(annotation.SuggestedSummary), suggestedSummary)
		source = firstNonEmpty(strings.TrimSpace(annotation.SuggestedSource), "llm")
		if annotation.SuggestedConfidence != nil {
			confidence = *annotation.SuggestedConfidence
		}
	}
	effectiveClass := suggestedClass
	effectiveSummary := suggestedSummary
	if annotation.HumanClass != nil && strings.TrimSpace(*annotation.HumanClass) != "" {
		effectiveClass = strings.TrimSpace(*annotation.HumanClass)
		effectiveSummary = firstNonEmpty(strings.TrimSpace(annotation.HumanSummary), suggestedSummary)
	}
	review := AgentHarnessFailureReview{
		ExecutionID:         execution.ID,
		AgentHarnessID:      execution.AgentHarnessID,
		WorkspaceID:         execution.WorkspaceID,
		RunID:               cloneUUIDPtr(execution.RunID),
		RunAgentID:          cloneUUIDPtr(execution.RunAgentID),
		Status:              execution.Status,
		SuggestedClass:      suggestedClass,
		SuggestedSummary:    suggestedSummary,
		SuggestedSource:     source,
		SuggestedConfidence: confidence,
		HumanClass:          cloneStringPtr(annotation.HumanClass),
		HumanSummary:        annotation.HumanSummary,
		EffectiveClass:      effectiveClass,
		EffectiveSummary:    effectiveSummary,
		Taxonomy:            agentHarnessFailureTaxonomy(effectiveClass),
		RepositoryURL:       snapshot.RepositoryURL,
		BaseBranch:          snapshot.BaseBranch,
		TaskType:            firstNonEmpty(eval.Suite.TaskSource, "manual"),
		HarnessName:         snapshot.Name,
		HarnessKind:         snapshot.HarnessKind,
		Model:               snapshot.CodexModel,
		Template:            snapshot.CodexTemplate,
		SuiteID:             cloneUUIDPtr(&eval.Suite.SuiteID),
		SuiteVersionID:      cloneUUIDPtr(&eval.Suite.SuiteVersionID),
		SuiteTaskID:         cloneUUIDPtr(&eval.Suite.TaskID),
		CreatedAt:           execution.CreatedAt,
		UpdatedAt:           execution.UpdatedAt,
		EventRefs:           agentHarnessFailureEventRefs(events),
	}
	if review.SuiteID != nil && *review.SuiteID == uuid.Nil {
		review.SuiteID = nil
	}
	if review.SuiteVersionID != nil && *review.SuiteVersionID == uuid.Nil {
		review.SuiteVersionID = nil
	}
	if review.SuiteTaskID != nil && *review.SuiteTaskID == uuid.Nil {
		review.SuiteTaskID = nil
	}
	if scorecard != nil {
		review.ScorecardPassed = cloneBoolPtr(scorecard.Passed)
		review.OverallScore = cloneFloat64Ptr(scorecard.OverallScore)
	}
	return review
}

func classifyAgentHarnessFailure(execution AgentHarnessExecution, events []AgentHarnessExecutionEvent, scorecard *RunAgentScorecard) (string, string, float64) {
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		eventType := strings.ToLower(strings.TrimSpace(event.EventType))
		payload := strings.ToLower(string(event.Payload))
		joined := eventType + " " + payload + " " + strings.ToLower(derefString(execution.ErrorMessage))
		switch {
		case containsAny(joined, "timeout", "deadline exceeded", "timed out"):
			return AgentHarnessFailureClassTimeout, "Execution exceeded the configured time or budget limit.", 0.92
		case strings.HasPrefix(eventType, "setup.") || containsAny(joined, "setup command", "bootstrap"):
			return AgentHarnessFailureClassSetup, "Setup or bootstrap failed before the coding agent could complete the task.", 0.88
		case containsAny(joined, "auth", "credential", "permission denied", "api key", "github.git_auth", "repository_access_revoked", "403", "401"):
			return AgentHarnessFailureClassAuth, "Repository, provider, or secret authentication failed.", 0.9
		case strings.HasPrefix(eventType, "validator.") || containsAny(joined, "test failed", "tests failed", "exit code", "validator.command.failed"):
			return AgentHarnessFailureClassTestFailure, "Validation or test commands failed after the agent ran.", 0.84
		case containsAny(joined, "policy", "privacy", "redact", "forbidden"):
			return AgentHarnessFailureClassPolicyPrivacy, "Policy or privacy guardrails blocked the execution.", 0.82
		case containsAny(joined, "no pull request", "no pr", "pull request not found"):
			return AgentHarnessFailureClassNoPR, "The agent did not create the expected pull request.", 0.83
		case containsAny(joined, "no-op diff", "empty diff", "no changes"):
			return AgentHarnessFailureClassNoOpDiff, "The agent produced no meaningful code changes.", 0.83
		case containsAny(joined, "overbroad", "too many files", "large diff"):
			return AgentHarnessFailureClassOverbroadDiff, "The agent changed more code than the task called for.", 0.76
		case strings.HasPrefix(eventType, "llm_judges.") || strings.HasPrefix(eventType, "scoring.") || containsAny(joined, "judge"):
			return AgentHarnessFailureClassJudgeFailure, "Evaluation or LLM judge scoring failed or rejected the result.", 0.8
		case containsAny(joined, "tool misuse", "wrong tool", "invalid tool"):
			return AgentHarnessFailureClassToolMisuse, "The agent misused a tool or selected the wrong tool path.", 0.76
		case strings.HasPrefix(eventType, "codex.") || strings.HasPrefix(eventType, "claude."):
			return AgentHarnessFailureClassIncompleteImplementation, "The coding agent failed before producing a complete implementation.", 0.7
		}
	}
	if scorecard != nil {
		if scorecard.Passed != nil && !*scorecard.Passed {
			if scorecard.CorrectnessScore != nil && *scorecard.CorrectnessScore < evalSessionDefaultSuccessThreshold {
				return AgentHarnessFailureClassIncompleteImplementation, "Scorecard correctness did not meet the success threshold.", 0.72
			}
			return AgentHarnessFailureClassTestFailure, "Scorecard marked the harness execution as failed.", 0.68
		}
		if scorecard.Passed != nil && *scorecard.Passed {
			return AgentHarnessFailureClassNone, "Execution passed its configured evaluation.", 0.95
		}
	}
	if execution.Status == string(AgentHarnessExecutionStatusCancelled) {
		return AgentHarnessFailureClassTimeout, "Execution was cancelled before completion.", 0.55
	}
	if execution.Status == string(AgentHarnessExecutionStatusFailed) {
		return AgentHarnessFailureClassInfrastructure, "Execution failed without more specific structured evidence.", 0.45
	}
	return AgentHarnessFailureClassUnknown, "No structured failure classification evidence is available.", 0.25
}

func agentHarnessFailureTaxonomy(class string) failurereview.FailureTaxonomy {
	switch class {
	case AgentHarnessFailureClassSetup:
		return failurereview.FailureTaxonomy{Family: "workflow", Code: "harness.setup", Label: "Setup failure", AgentFault: false}
	case AgentHarnessFailureClassAuth:
		return failurereview.FailureTaxonomy{Family: "platform", Code: "harness.auth", Label: "Authentication failure", AgentFault: false}
	case AgentHarnessFailureClassToolMisuse:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.tool_misuse", Label: "Tool misuse", AgentFault: true}
	case AgentHarnessFailureClassIncompleteImplementation:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.incomplete_implementation", Label: "Incomplete implementation", AgentFault: true}
	case AgentHarnessFailureClassNoOpDiff:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.no_op_diff", Label: "No-op diff", AgentFault: true}
	case AgentHarnessFailureClassTestFailure:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.test_failure", Label: "Test failure", AgentFault: true}
	case AgentHarnessFailureClassOverbroadDiff:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.overbroad_diff", Label: "Overbroad diff", AgentFault: true}
	case AgentHarnessFailureClassNoPR:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.no_pr", Label: "No pull request", AgentFault: true}
	case AgentHarnessFailureClassJudgeFailure:
		return failurereview.FailureTaxonomy{Family: "evidence", Code: "harness.judge_failure", Label: "Judge failure", AgentFault: false}
	case AgentHarnessFailureClassTimeout:
		return failurereview.FailureTaxonomy{Family: "workflow", Code: "harness.timeout", Label: "Timeout", AgentFault: true}
	case AgentHarnessFailureClassPolicyPrivacy:
		return failurereview.FailureTaxonomy{Family: "agent", Code: "harness.policy_privacy", Label: "Policy or privacy failure", AgentFault: true}
	case AgentHarnessFailureClassNone:
		return failurereview.FailureTaxonomy{Family: "evidence", Code: "harness.passed", Label: "Passed", AgentFault: false}
	default:
		return failurereview.FailureTaxonomy{Family: "platform", Code: "harness.infrastructure", Label: "Infrastructure or unknown failure", AgentFault: false}
	}
}

func agentHarnessFailureEventRefs(events []AgentHarnessExecutionEvent) []AgentHarnessFailureEventRef {
	refs := make([]AgentHarnessFailureEventRef, 0, len(events))
	for _, event := range events {
		if strings.HasSuffix(event.EventType, ".failed") ||
			event.EventType == "github.repository_access_revoked" ||
			strings.Contains(event.EventType, "policy") ||
			strings.Contains(event.EventType, "timeout") {
			refs = append(refs, AgentHarnessFailureEventRef{
				SequenceNumber: event.SequenceNumber,
				EventType:      event.EventType,
				ActorType:      event.ActorType,
				ArtifactID:     cloneUUIDPtr(event.ArtifactID),
				Payload:        cloneJSON(event.Payload),
			})
		}
	}
	return refs
}

func (r *Repository) GetAgentHarnessFailureAnnotationByExecutionID(ctx context.Context, executionID uuid.UUID) (AgentHarnessFailureAnnotation, error) {
	row := r.db.QueryRow(ctx, `
SELECT id, agent_harness_execution_id, suggested_class, suggested_summary, suggested_source,
    suggested_confidence, suggested_payload, human_class, human_summary, human_payload,
    edited_by_user_id, created_at, updated_at
FROM agent_harness_failure_annotations
WHERE agent_harness_execution_id = $1`, executionID)
	return scanAgentHarnessFailureAnnotation(row)
}

func (r *Repository) UpsertAgentHarnessFailureAnnotation(ctx context.Context, p UpsertAgentHarnessFailureAnnotationParams) (AgentHarnessFailureAnnotation, error) {
	source := strings.TrimSpace(p.SuggestedSource)
	if source == "" {
		source = "rules"
	}
	confidence, err := numericFromFloat(p.SuggestedConfidence)
	if err != nil {
		return AgentHarnessFailureAnnotation{}, err
	}
	row := r.db.QueryRow(ctx, `
INSERT INTO agent_harness_failure_annotations (
    agent_harness_execution_id, suggested_class, suggested_summary, suggested_source, suggested_confidence,
    suggested_payload, human_class, human_summary, human_payload, edited_by_user_id
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (agent_harness_execution_id) DO UPDATE SET
    suggested_class = COALESCE(EXCLUDED.suggested_class, agent_harness_failure_annotations.suggested_class),
    suggested_summary = CASE WHEN EXCLUDED.suggested_summary <> '' THEN EXCLUDED.suggested_summary ELSE agent_harness_failure_annotations.suggested_summary END,
    suggested_source = CASE
        WHEN EXCLUDED.suggested_class IS NOT NULL
            OR EXCLUDED.suggested_summary <> ''
            OR EXCLUDED.suggested_confidence IS NOT NULL
            OR EXCLUDED.suggested_payload <> '{}'::jsonb
        THEN EXCLUDED.suggested_source
        ELSE agent_harness_failure_annotations.suggested_source
    END,
    suggested_confidence = COALESCE(EXCLUDED.suggested_confidence, agent_harness_failure_annotations.suggested_confidence),
    suggested_payload = CASE WHEN EXCLUDED.suggested_payload <> '{}'::jsonb THEN EXCLUDED.suggested_payload ELSE agent_harness_failure_annotations.suggested_payload END,
    human_class = COALESCE(EXCLUDED.human_class, agent_harness_failure_annotations.human_class),
    human_summary = CASE WHEN EXCLUDED.human_summary <> '' THEN EXCLUDED.human_summary ELSE agent_harness_failure_annotations.human_summary END,
    human_payload = CASE WHEN EXCLUDED.human_payload <> '{}'::jsonb THEN EXCLUDED.human_payload ELSE agent_harness_failure_annotations.human_payload END,
    edited_by_user_id = COALESCE(EXCLUDED.edited_by_user_id, agent_harness_failure_annotations.edited_by_user_id),
    updated_at = now()
RETURNING id, agent_harness_execution_id, suggested_class, suggested_summary, suggested_source,
    suggested_confidence, suggested_payload, human_class, human_summary, human_payload,
    edited_by_user_id, created_at, updated_at`,
		p.ExecutionID,
		p.SuggestedClass,
		strings.TrimSpace(p.SuggestedSummary),
		source,
		confidence,
		defaultRepositoryJSON(p.SuggestedPayload),
		p.HumanClass,
		strings.TrimSpace(p.HumanSummary),
		defaultRepositoryJSON(p.HumanPayload),
		p.EditedByUserID,
	)
	return scanAgentHarnessFailureAnnotation(row)
}

func scanAgentHarnessFailureAnnotation(scanner agentHarnessExecutionScanner) (AgentHarnessFailureAnnotation, error) {
	var annotation AgentHarnessFailureAnnotation
	var confidence pgtype.Numeric
	err := scanner.Scan(
		&annotation.ID,
		&annotation.AgentHarnessExecutionID,
		&annotation.SuggestedClass,
		&annotation.SuggestedSummary,
		&annotation.SuggestedSource,
		&confidence,
		&annotation.SuggestedPayload,
		&annotation.HumanClass,
		&annotation.HumanSummary,
		&annotation.HumanPayload,
		&annotation.EditedByUserID,
		&annotation.CreatedAt,
		&annotation.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return AgentHarnessFailureAnnotation{}, ErrAgentHarnessFailureAnnotationNotFound
		}
		return AgentHarnessFailureAnnotation{}, err
	}
	annotation.SuggestedConfidence = numericPtr(confidence)
	return annotation, nil
}

func (r *Repository) PromoteAgentHarnessExecutionToSuite(ctx context.Context, p PromoteAgentHarnessExecutionToSuiteParams) (PromoteAgentHarnessExecutionToSuiteResult, error) {
	execution, err := r.GetAgentHarnessExecutionByID(ctx, p.ExecutionID)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	suite, err := r.GetAgentHarnessSuiteByID(ctx, p.SuiteID)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	if suite.WorkspaceID != execution.WorkspaceID || suite.OrganizationID != execution.OrganizationID || suite.Status != "active" {
		return PromoteAgentHarnessExecutionToSuiteResult{}, ErrAgentHarnessSuiteNotFound
	}
	existingTasks, err := r.ListAgentHarnessSuiteTasksByVersionID(ctx, suite.CurrentVersionID)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}

	harness := decodeAgentHarnessFailureHarnessSnapshot(execution.HarnessSnapshot)
	eval := decodeAgentHarnessFailureEvaluationConfig(execution.EvaluationConfigSnapshot)
	events, err := r.ListAgentHarnessExecutionEvents(ctx, execution.ID)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	sourceSnapshot, err := agentHarnessPromotionSourceSnapshot(execution, harness, events)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	metadata, err := agentHarnessPromotionMetadata(p, execution, harness, eval)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	title := firstNonEmpty(strings.TrimSpace(p.Title), "Prior harness run "+execution.ID.String())
	publicPrompt := firstNonEmpty(strings.TrimSpace(p.PublicPrompt), eval.Suite.PublicPrompt, "Prior harness run task")
	taskPrompt := firstNonEmpty(harness.TaskPrompt, publicPrompt)

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	defer rollback(ctx, tx)

	var versionID uuid.UUID
	versionNumber := suite.CurrentVersionNumber + 1
	if err := tx.QueryRow(ctx, `
INSERT INTO agent_harness_suite_versions (
    organization_id, workspace_id, agent_harness_suite_id, version_number, metadata, created_by_user_id
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`,
		suite.OrganizationID,
		suite.WorkspaceID,
		suite.ID,
		versionNumber,
		defaultRepositoryJSON(suite.Metadata),
		p.CreatedByUserID,
	).Scan(&versionID); err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, fmt.Errorf("create promoted agent harness suite version: %w", err)
	}
	for index, task := range existingTasks {
		if _, err := tx.Exec(ctx, `
INSERT INTO agent_harness_suite_tasks (
    organization_id, workspace_id, agent_harness_suite_version_id, task_order,
    title, public_prompt, task_prompt, source_type, source_snapshot,
    repository_url, base_branch, execution_config, evaluation_config, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			task.OrganizationID, task.WorkspaceID, versionID, int32(index),
			task.Title, task.PublicPrompt, task.TaskPrompt, task.SourceType, defaultRepositoryJSON(task.SourceSnapshot),
			task.RepositoryURL, task.BaseBranch, defaultRepositoryJSON(task.ExecutionConfig), defaultRepositoryJSON(task.EvaluationConfig), defaultRepositoryJSON(task.Metadata),
		); err != nil {
			return PromoteAgentHarnessExecutionToSuiteResult{}, fmt.Errorf("copy agent harness suite task %s: %w", task.ID, err)
		}
	}
	var promotedTask AgentHarnessSuiteTask
	row := tx.QueryRow(ctx, `
INSERT INTO agent_harness_suite_tasks (
    organization_id, workspace_id, agent_harness_suite_version_id, task_order,
    title, public_prompt, task_prompt, source_type, source_snapshot,
    repository_url, base_branch, execution_config, evaluation_config, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, 'prior_harness_run', $8, $9, $10, $11, $12, $13)
RETURNING id, organization_id, workspace_id, agent_harness_suite_version_id, task_order,
    title, public_prompt, task_prompt, source_type, source_snapshot,
    repository_url, base_branch, execution_config, evaluation_config, metadata, created_at, updated_at`,
		suite.OrganizationID,
		suite.WorkspaceID,
		versionID,
		int32(len(existingTasks)),
		title,
		publicPrompt,
		taskPrompt,
		sourceSnapshot,
		optionalRepositoryStringPtr(harness.RepositoryURL),
		optionalRepositoryStringPtr(harness.BaseBranch),
		defaultRepositoryJSON(execution.ExecutionConfigSnapshot),
		defaultRepositoryJSON(execution.EvaluationConfigSnapshot),
		metadata,
	)
	promotedTask, err = scanAgentHarnessSuiteTask(row)
	if err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE agent_harness_suites
SET current_version_number = $2,
    updated_at = now()
WHERE id = $1`, suite.ID, versionNumber); err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, fmt.Errorf("advance promoted agent harness suite version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return PromoteAgentHarnessExecutionToSuiteResult{}, fmt.Errorf("commit promoted agent harness suite version: %w", err)
	}
	suite.CurrentVersionNumber = versionNumber
	suite.CurrentVersionID = versionID
	suite.TaskCount = len(existingTasks) + 1
	return PromoteAgentHarnessExecutionToSuiteResult{Suite: suite, Task: promotedTask}, nil
}

func agentHarnessPromotionSourceSnapshot(execution AgentHarnessExecution, harness agentHarnessFailureHarnessSnapshot, events []AgentHarnessExecutionEvent) (json.RawMessage, error) {
	artifactIDs := make([]string, 0)
	for _, event := range events {
		if event.ArtifactID != nil {
			artifactIDs = append(artifactIDs, event.ArtifactID.String())
		}
	}
	sort.Strings(artifactIDs)
	payload := map[string]any{
		"origin":             "agent_harness_execution",
		"execution_id":       execution.ID.String(),
		"agent_harness_id":   execution.AgentHarnessID.String(),
		"status":             execution.Status,
		"repository_url":     harness.RepositoryURL,
		"base_branch":        harness.BaseBranch,
		"artifact_ids":       artifactIDs,
		"source_fingerprint": agentHarnessPromotionFingerprint(execution, harness),
	}
	if execution.RunID != nil {
		payload["run_id"] = execution.RunID.String()
	}
	if execution.RunAgentID != nil {
		payload["run_agent_id"] = execution.RunAgentID.String()
	}
	return json.Marshal(payload)
}

func agentHarnessPromotionMetadata(p PromoteAgentHarnessExecutionToSuiteParams, execution AgentHarnessExecution, harness agentHarnessFailureHarnessSnapshot, eval agentHarnessFailureEvaluationConfig) (json.RawMessage, error) {
	var metadata map[string]any
	if len(p.Metadata) > 0 && string(p.Metadata) != "null" {
		if err := json.Unmarshal(p.Metadata, &metadata); err != nil {
			return nil, fmt.Errorf("decode promotion metadata: %w", err)
		}
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["origin"] = "agent_harness_execution"
	metadata["source_execution_id"] = execution.ID.String()
	metadata["source_agent_harness_id"] = execution.AgentHarnessID.String()
	metadata["source_status"] = execution.Status
	metadata["failure_class"] = strings.TrimSpace(p.FailureClass)
	metadata["failure_summary"] = strings.TrimSpace(p.FailureSummary)
	metadata["repository_url"] = harness.RepositoryURL
	metadata["base_branch"] = harness.BaseBranch
	metadata["harness_kind"] = harness.HarnessKind
	metadata["codex_template"] = harness.CodexTemplate
	metadata["codex_model"] = harness.CodexModel
	if eval.Suite.SuiteID != uuid.Nil {
		metadata["source_suite_id"] = eval.Suite.SuiteID.String()
	}
	return json.Marshal(metadata)
}

func agentHarnessPromotionFingerprint(execution AgentHarnessExecution, harness agentHarnessFailureHarnessSnapshot) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		execution.ID.String(),
		execution.AgentHarnessID.String(),
		harness.RepositoryURL,
		harness.BaseBranch,
		harness.TaskPrompt,
	}, "\x00")))
	return "agent-harness:" + hex.EncodeToString(sum[:16])
}

type agentHarnessFailureHarnessSnapshot struct {
	Name          string
	HarnessKind   string
	TaskPrompt    string
	CodexTemplate string
	CodexModel    string
	RepositoryURL string
	BaseBranch    string
}

func decodeAgentHarnessFailureHarnessSnapshot(raw json.RawMessage) agentHarnessFailureHarnessSnapshot {
	var snapshot struct {
		Name          string  `json:"name"`
		HarnessKind   string  `json:"harness_kind"`
		TaskPrompt    string  `json:"task_prompt"`
		CodexTemplate string  `json:"codex_template"`
		CodexModel    *string `json:"codex_model"`
		RepositoryURL *string `json:"repository_url"`
		BaseBranch    *string `json:"base_branch"`
	}
	_ = json.Unmarshal(raw, &snapshot)
	return agentHarnessFailureHarnessSnapshot{
		Name:          strings.TrimSpace(snapshot.Name),
		HarnessKind:   strings.TrimSpace(snapshot.HarnessKind),
		TaskPrompt:    strings.TrimSpace(snapshot.TaskPrompt),
		CodexTemplate: strings.TrimSpace(snapshot.CodexTemplate),
		CodexModel:    derefString(snapshot.CodexModel),
		RepositoryURL: derefString(snapshot.RepositoryURL),
		BaseBranch:    derefString(snapshot.BaseBranch),
	}
}

type agentHarnessFailureEvaluationConfig struct {
	Suite struct {
		SuiteID        uuid.UUID       `json:"suite_id"`
		SuiteVersionID uuid.UUID       `json:"suite_version_id"`
		TaskID         uuid.UUID       `json:"task_id"`
		TaskSource     string          `json:"task_source"`
		PublicPrompt   string          `json:"public_prompt"`
		TaskMetadata   json.RawMessage `json:"task_metadata"`
	} `json:"suite"`
}

func decodeAgentHarnessFailureEvaluationConfig(raw json.RawMessage) agentHarnessFailureEvaluationConfig {
	var config agentHarnessFailureEvaluationConfig
	_ = json.Unmarshal(raw, &config)
	config.Suite.TaskSource = strings.TrimSpace(config.Suite.TaskSource)
	config.Suite.PublicPrompt = strings.TrimSpace(config.Suite.PublicPrompt)
	return config
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func optionalRepositoryStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
