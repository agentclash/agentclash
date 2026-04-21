package failurereview

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/domain"
	"github.com/google/uuid"
)

type FailureState string

const (
	FailureStateFailed             FailureState = "failed"
	FailureStateWarning            FailureState = "warning"
	FailureStateFlaky              FailureState = "flaky"
	FailureStateIncompleteEvidence FailureState = "incomplete_evidence"
)

type EvidenceTier string

const (
	EvidenceTierNone             EvidenceTier = "none"
	EvidenceTierNativeStructured EvidenceTier = "native_structured"
	EvidenceTierHostedStructured EvidenceTier = "hosted_structured"
	EvidenceTierHostedBlackBox   EvidenceTier = "hosted_black_box"
	EvidenceTierDerivedSummary   EvidenceTier = "derived_summary"
)

type PromotionMode string

const (
	PromotionModeFullExecutable PromotionMode = "full_executable"
	PromotionModeOutputOnly     PromotionMode = "output_only"
)

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityBlocking Severity = "blocking"
)

type Item struct {
	RunID                  uuid.UUID       `json:"run_id"`
	RunAgentID             uuid.UUID       `json:"run_agent_id"`
	ChallengeIdentityID    *uuid.UUID      `json:"challenge_identity_id,omitempty"`
	ChallengeKey           string          `json:"challenge_key"`
	CaseKey                string          `json:"case_key"`
	ItemKey                string          `json:"item_key"`
	FailureState           FailureState    `json:"failure_state"`
	FailedDimensions       []string        `json:"failed_dimensions"`
	FailedChecks           []string        `json:"failed_checks"`
	FailureClass           FailureClass    `json:"failure_class"`
	Headline               string          `json:"headline"`
	Detail                 string          `json:"detail"`
	RecommendedAction      string          `json:"recommended_action"`
	Promotable             bool            `json:"promotable"`
	PromotionModeAvailable []PromotionMode `json:"promotion_mode_available"`
	ReplayStepRefs         []ReplayStepRef `json:"replay_step_refs"`
	ArtifactRefs           []ArtifactRef   `json:"artifact_refs"`
	JudgeRefs              []JudgeRef      `json:"judge_refs"`
	MetricRefs             []MetricRef     `json:"metric_refs"`
	EvidenceTier           EvidenceTier    `json:"evidence_tier"`
	Severity               Severity        `json:"severity"`
	sortKey                CursorKey       `json:"-"`
}

type ReplayStepRef struct {
	SequenceNumber int64  `json:"sequence_number"`
	EventType      string `json:"event_type"`
	Kind           string `json:"kind"`
}

type ArtifactRef struct {
	Key       string `json:"key"`
	Kind      string `json:"kind,omitempty"`
	Path      string `json:"path,omitempty"`
	MediaType string `json:"media_type,omitempty"`
}

type JudgeRef struct {
	Key             string   `json:"key"`
	Kind            string   `json:"kind"`
	Verdict         string   `json:"verdict,omitempty"`
	State           string   `json:"state,omitempty"`
	NormalizedScore *float64 `json:"normalized_score,omitempty"`
	Reason          string   `json:"reason,omitempty"`
	SequenceNumber  *int64   `json:"sequence_number,omitempty"`
	EventType       string   `json:"event_type,omitempty"`
}

type MetricRef struct {
	Key          string   `json:"key"`
	MetricType   string   `json:"metric_type"`
	State        string   `json:"state,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	NumericValue *float64 `json:"numeric_value,omitempty"`
	TextValue    *string  `json:"text_value,omitempty"`
	BooleanValue *bool    `json:"boolean_value,omitempty"`
	Unit         *string  `json:"unit,omitempty"`
}

type RunAgentInput struct {
	RunID                uuid.UUID
	RunStatus            domain.RunStatus
	RunAgentID           uuid.UUID
	RunAgentLabel        string
	DeploymentType       string
	ChallengePackStatus  string
	HasChallengeInputSet bool
	ToolPolicy           json.RawMessage
	Cases                []CaseContext
	Scorecard            json.RawMessage
	JudgeResults         []JudgeResult
	MetricResults        []MetricResult
	LLMJudgeResults      []LLMJudgeResult
	Events               []Event
}

type CaseContext struct {
	ChallengeIdentityID uuid.UUID
	ChallengeKey        string
	CaseKey             string
	ItemKey             string
	Payload             json.RawMessage
	Artifacts           []ArtifactContext
}

type ArtifactContext struct {
	Key       string
	Kind      string
	Path      string
	MediaType string
}

type JudgeResult struct {
	ChallengeIdentityID *uuid.UUID
	Key                 string
	Verdict             *string
	NormalizedScore     *float64
	Reason              string
}

type MetricResult struct {
	ChallengeIdentityID *uuid.UUID
	Key                 string
	MetricType          string
	NumericValue        *float64
	TextValue           *string
	BooleanValue        *bool
	Unit                *string
}

type LLMJudgeResult struct {
	Key             string
	Mode            string
	NormalizedScore *float64
	Reason          string
	State           string
	Verdict         string
	Passed          *bool
}

type Event struct {
	SequenceNumber int64
	EventType      string
	Source         string
	Payload        json.RawMessage
}

type CursorKey struct {
	RunAgentID   string
	ChallengeID  string
	ChallengeKey string
	CaseKey      string
	ItemKey      string
}

func BuildRunAgentItems(input RunAgentInput) ([]Item, error) {
	scorecard, err := decodeScorecardDocument(input.Scorecard)
	if err != nil {
		return nil, err
	}

	caseByID := make(map[string]CaseContext, len(input.Cases))
	for _, item := range input.Cases {
		caseByID[item.ChallengeIdentityID.String()] = item
	}

	validatorByKey := make(map[string]validatorDetail, len(scorecard.ValidatorDetails))
	for _, detail := range scorecard.ValidatorDetails {
		validatorByKey[detail.Key] = detail
	}
	metricByKey := make(map[string]metricDetail, len(scorecard.MetricDetails))
	for _, detail := range scorecard.MetricDetails {
		metricByKey[detail.Key] = detail
	}

	groups := make(map[string]*itemGroup)
	for _, judge := range input.JudgeResults {
		if judge.ChallengeIdentityID == nil {
			continue
		}
		group := ensureGroup(groups, caseByID, input, *judge.ChallengeIdentityID)
		detail := validatorByKey[judge.Key]
		group.JudgeRefs = append(group.JudgeRefs, buildJudgeRef(judge, detail))
		if isFailedJudge(judge, detail) {
			group.FailedChecks = append(group.FailedChecks, judge.Key)
			if detail.Source != nil && detail.Source.Sequence != nil {
				group.ReplayStepRefs = append(group.ReplayStepRefs, ReplayStepRef{
					SequenceNumber: *detail.Source.Sequence,
					EventType:      detail.Source.EventType,
					Kind:           firstNonEmpty(detail.Source.Kind, "run_event"),
				})
			}
		}
	}
	for _, metric := range input.MetricResults {
		if metric.ChallengeIdentityID == nil {
			continue
		}
		group := ensureGroup(groups, caseByID, input, *metric.ChallengeIdentityID)
		group.MetricRefs = append(group.MetricRefs, buildMetricRef(metric, metricByKey[metric.Key]))
	}

	if len(groups) == 0 && len(input.Cases) == 1 && hasFailingLLMJudge(input.LLMJudgeResults) {
		group := ensureGroup(groups, caseByID, input, input.Cases[0].ChallengeIdentityID)
		group.OnlyLLMJudges = true
	}

	items := make([]Item, 0, len(groups))
	for _, group := range groups {
		item, ok := finalizeGroup(group, input, scorecard)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return compareSortKeys(items[i].sortKey, items[j].sortKey) < 0
	})

	return items, nil
}

func FilterItems(items []Item, agentID *uuid.UUID, severity *Severity, failureClass *FailureClass, evidenceTier *EvidenceTier, challengeKey, caseKey *string) []Item {
	filtered := make([]Item, 0, len(items))
	for _, item := range items {
		if agentID != nil && item.RunAgentID != *agentID {
			continue
		}
		if severity != nil && item.Severity != *severity {
			continue
		}
		if failureClass != nil && item.FailureClass != *failureClass {
			continue
		}
		if evidenceTier != nil && item.EvidenceTier != *evidenceTier {
			continue
		}
		if challengeKey != nil && item.ChallengeKey != *challengeKey {
			continue
		}
		if caseKey != nil && item.CaseKey != *caseKey {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func PaginateItems(items []Item, after *CursorKey, limit int) ([]Item, *CursorKey) {
	ordered := append([]Item(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		return compareSortKeys(ordered[i].sortKey, ordered[j].sortKey) < 0
	})

	start := 0
	if after != nil {
		for i := range ordered {
			if compareSortKeys(ordered[i].sortKey, *after) > 0 {
				start = i
				break
			}
			start = len(ordered)
		}
	}
	if start >= len(ordered) {
		return []Item{}, nil
	}

	end := start + limit
	if end > len(ordered) {
		end = len(ordered)
	}
	page := append([]Item(nil), ordered[start:end]...)
	if end >= len(ordered) {
		return page, nil
	}
	next := ordered[end-1].sortKey
	return page, &next
}

func EncodeCursor(key CursorKey) (string, error) {
	encoded, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func DecodeCursor(raw string) (CursorKey, error) {
	var key CursorKey
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		decoded = []byte(raw)
	}
	if err := json.Unmarshal(decoded, &key); err != nil {
		return CursorKey{}, fmt.Errorf("decode cursor: %w", err)
	}
	return key, nil
}

type itemGroup struct {
	RunID          uuid.UUID
	RunAgentID     uuid.UUID
	Case           CaseContext
	ChallengeID    *uuid.UUID
	JudgeRefs      []JudgeRef
	MetricRefs     []MetricRef
	FailedChecks   []string
	ReplayStepRefs []ReplayStepRef
	OnlyLLMJudges  bool
}

type scorecardDocument struct {
	Dimensions       map[string]dimensionSummary `json:"dimensions"`
	ValidatorDetails []validatorDetail           `json:"validator_details"`
	MetricDetails    []metricDetail              `json:"metric_details"`
}

type dimensionSummary struct {
	State  string   `json:"state"`
	Score  *float64 `json:"score,omitempty"`
	Reason string   `json:"reason,omitempty"`
}

type validatorDetail struct {
	Key             string           `json:"key"`
	Type            string           `json:"type"`
	Verdict         string           `json:"verdict"`
	State           string           `json:"state"`
	Reason          string           `json:"reason,omitempty"`
	NormalizedScore *float64         `json:"normalized_score,omitempty"`
	Source          *scorecardSource `json:"source,omitempty"`
}

type scorecardSource struct {
	Kind      string `json:"kind"`
	Sequence  *int64 `json:"sequence,omitempty"`
	EventType string `json:"event_type,omitempty"`
	FieldPath string `json:"field_path,omitempty"`
}

type metricDetail struct {
	Key          string   `json:"key"`
	State        string   `json:"state"`
	Reason       string   `json:"reason,omitempty"`
	NumericValue *float64 `json:"numeric_value,omitempty"`
	TextValue    *string  `json:"text_value,omitempty"`
	BooleanValue *bool    `json:"boolean_value,omitempty"`
}

func decodeScorecardDocument(payload json.RawMessage) (scorecardDocument, error) {
	document := scorecardDocument{
		Dimensions:       map[string]dimensionSummary{},
		ValidatorDetails: []validatorDetail{},
		MetricDetails:    []metricDetail{},
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return document, nil
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		return scorecardDocument{}, fmt.Errorf("decode failure review scorecard: %w", err)
	}
	if document.Dimensions == nil {
		document.Dimensions = map[string]dimensionSummary{}
	}
	return document, nil
}

func ensureGroup(groups map[string]*itemGroup, caseByID map[string]CaseContext, input RunAgentInput, challengeIdentityID uuid.UUID) *itemGroup {
	key := challengeIdentityID.String()
	if existing, ok := groups[key]; ok {
		return existing
	}
	item := caseByID[key]
	group := &itemGroup{
		RunID:       input.RunID,
		RunAgentID:  input.RunAgentID,
		Case:        item,
		ChallengeID: uuidPtr(challengeIdentityID),
	}
	groups[key] = group
	return group
}

func finalizeGroup(group *itemGroup, input RunAgentInput, scorecard scorecardDocument) (Item, bool) {
	failedDimensions := failedDimensions(scorecard.Dimensions)
	evidenceTier := inferEvidenceTier(input.DeploymentType, input.Events)
	finalOutputRef := finalOutputReplayRef(input.Events)

	judgeRefs := append([]JudgeRef(nil), group.JudgeRefs...)
	if len(input.Cases) == 1 || group.OnlyLLMJudges {
		for _, judge := range input.LLMJudgeResults {
			ref := JudgeRef{
				Key:             judge.Key,
				Kind:            "llm_judge",
				State:           llmJudgeState(judge),
				NormalizedScore: cloneFloat64(judge.NormalizedScore),
				Reason:          strings.TrimSpace(judge.Reason),
			}
			if finalOutputRef != nil {
				ref.SequenceNumber = &finalOutputRef.SequenceNumber
				ref.EventType = finalOutputRef.EventType
			}
			judgeRefs = append(judgeRefs, ref)
			if ref.State == "fail" {
				group.FailedChecks = append(group.FailedChecks, judge.Key)
			}
		}
	}

	dedupStrings(&group.FailedChecks)
	dedupReplayRefs(&group.ReplayStepRefs)

	hasStructuredSignal := len(group.ReplayStepRefs) > 0 || len(group.JudgeRefs) > 0 || len(group.MetricRefs) > 0
	classification := ClassificationInput{
		EvidenceTier:               evidenceTier,
		FailedChecks:               group.FailedChecks,
		HasStructuredFailureSignal: hasStructuredSignal,
		HasTimeoutOrBudgetSignal:   hasTimeoutOrBudgetSignal(input.Events),
		HasSandboxFailure:          hasSandboxFailure(input.Events),
		HasMalformedOutput:         hasMalformedOutput(scorecard, group.FailedChecks),
		HasLLMFinalAnswerFailure:   hasFailingLLMJudge(input.LLMJudgeResults),
	}
	failureClass := Classify(classification)
	failureState := deriveFailureState(group.FailedChecks, failedDimensions, evidenceTier)

	if len(group.FailedChecks) == 0 && len(failedDimensions) == 0 && failureState == FailureStateWarning {
		return Item{}, false
	}

	promotable := input.RunStatus == domain.RunStatusCompleted && group.ChallengeID != nil && evidenceTier != EvidenceTierNone
	promotionModes := make([]PromotionMode, 0, 2)
	if promotable && input.HasChallengeInputSet && (evidenceTier == EvidenceTierNativeStructured || evidenceTier == EvidenceTierHostedStructured) && input.ChallengePackStatus == "runnable" {
		promotionModes = append(promotionModes, PromotionModeFullExecutable)
	}
	if promotable && input.HasChallengeInputSet && finalOutputRef != nil {
		promotionModes = append(promotionModes, PromotionModeOutputOnly)
		group.ReplayStepRefs = append(group.ReplayStepRefs, *finalOutputRef)
		dedupReplayRefs(&group.ReplayStepRefs)
	}

	headline := buildHeadline(group.Case, failureClass, failedDimensions)
	detail := buildDetail(group.Case, group.FailedChecks, evidenceTier)
	recommendedAction := recommendedActionForClass(failureClass)

	item := Item{
		RunID:                  input.RunID,
		RunAgentID:             input.RunAgentID,
		ChallengeIdentityID:    cloneUUID(group.ChallengeID),
		ChallengeKey:           group.Case.ChallengeKey,
		CaseKey:                group.Case.CaseKey,
		ItemKey:                group.Case.ItemKey,
		FailureState:           failureState,
		FailedDimensions:       append([]string{}, failedDimensions...),
		FailedChecks:           append([]string{}, group.FailedChecks...),
		FailureClass:           failureClass,
		Headline:               headline,
		Detail:                 detail,
		RecommendedAction:      recommendedAction,
		Promotable:             promotable,
		PromotionModeAvailable: append([]PromotionMode{}, promotionModes...),
		ReplayStepRefs:         append([]ReplayStepRef{}, group.ReplayStepRefs...),
		ArtifactRefs:           buildArtifactRefs(group.Case.Artifacts),
		JudgeRefs:              append([]JudgeRef{}, judgeRefs...),
		MetricRefs:             append([]MetricRef{}, group.MetricRefs...),
		EvidenceTier:           evidenceTier,
		Severity:               severityFor(failureClass, failureState, evidenceTier),
		sortKey: CursorKey{
			RunAgentID:   input.RunAgentID.String(),
			ChallengeID:  uuidString(group.ChallengeID),
			ChallengeKey: group.Case.ChallengeKey,
			CaseKey:      group.Case.CaseKey,
			ItemKey:      group.Case.ItemKey,
		},
	}
	return item, true
}

func failedDimensions(dimensions map[string]dimensionSummary) []string {
	keys := make([]string, 0, len(dimensions))
	for key, detail := range dimensions {
		switch {
		case detail.State == "error", detail.State == "unavailable":
			keys = append(keys, key)
		case detail.Score != nil && *detail.Score < 1:
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func buildJudgeRef(result JudgeResult, detail validatorDetail) JudgeRef {
	ref := JudgeRef{
		Key:             result.Key,
		Kind:            firstNonEmpty(detail.Type, "validator"),
		Verdict:         valueOrEmpty(result.Verdict),
		State:           firstNonEmpty(detail.State, judgeState(result)),
		NormalizedScore: cloneFloat64(result.NormalizedScore),
		Reason:          firstNonEmpty(detail.Reason, result.Reason),
	}
	if detail.Source != nil {
		ref.SequenceNumber = cloneInt64(detail.Source.Sequence)
		ref.EventType = detail.Source.EventType
	}
	return ref
}

func buildMetricRef(result MetricResult, detail metricDetail) MetricRef {
	return MetricRef{
		Key:          result.Key,
		MetricType:   result.MetricType,
		State:        detail.State,
		Reason:       detail.Reason,
		NumericValue: cloneFloat64(result.NumericValue),
		TextValue:    cloneString(result.TextValue),
		BooleanValue: cloneBool(result.BooleanValue),
		Unit:         cloneString(result.Unit),
	}
}

func isFailedJudge(result JudgeResult, detail validatorDetail) bool {
	if detail.State == "error" || detail.State == "unavailable" {
		return true
	}
	if result.Verdict == nil {
		return false
	}
	return strings.TrimSpace(strings.ToLower(*result.Verdict)) != "pass"
}

func deriveFailureState(failedChecks []string, failedDimensions []string, evidenceTier EvidenceTier) FailureState {
	if evidenceTier == EvidenceTierHostedBlackBox && len(failedChecks) == 0 {
		return FailureStateIncompleteEvidence
	}
	if len(failedChecks) > 0 || len(failedDimensions) > 0 {
		return FailureStateFailed
	}
	return FailureStateWarning
}

func inferEvidenceTier(deploymentType string, events []Event) EvidenceTier {
	switch deploymentType {
	case "native":
		return EvidenceTierNativeStructured
	case "hosted_external":
		for _, event := range events {
			switch event.EventType {
			case "model.call.started", "model.call.completed", "tool.call.started", "tool.call.completed", "tool.call.failed", "sandbox.command.started", "sandbox.command.completed", "sandbox.command.failed", "system.output.finalized":
				return EvidenceTierHostedStructured
			}
		}
		if len(events) > 0 {
			return EvidenceTierHostedBlackBox
		}
	}
	if len(events) > 0 {
		return EvidenceTierDerivedSummary
	}
	return EvidenceTierNone
}

func hasTimeoutOrBudgetSignal(events []Event) bool {
	for _, event := range events {
		payload := lowerPayload(event.Payload)
		if event.EventType == "system.run.failed" && containsAny(payload, "timeout", "budget", "max_duration", "max tool", "max iteration") {
			return true
		}
	}
	return false
}

func hasSandboxFailure(events []Event) bool {
	for _, event := range events {
		payload := lowerPayload(event.Payload)
		if event.EventType == "sandbox.command.failed" {
			return true
		}
		if event.EventType == "system.run.failed" && containsAny(payload, "sandbox", "exit_code") {
			return true
		}
	}
	return false
}

func hasMalformedOutput(scorecard scorecardDocument, failedChecks []string) bool {
	for _, key := range failedChecks {
		for _, detail := range scorecard.ValidatorDetails {
			if detail.Key != key {
				continue
			}
			if detail.Type == "json_schema" || detail.Type == "json_path_match" {
				return true
			}
			if containsAny(strings.ToLower(detail.Reason), "parse actual json", "json schema", "jsonpath", "schema") {
				return true
			}
		}
	}
	return false
}

func hasFailingLLMJudge(results []LLMJudgeResult) bool {
	for _, result := range results {
		if result.NormalizedScore != nil && *result.NormalizedScore < 1 {
			return true
		}
		if strings.EqualFold(strings.TrimSpace(result.State), "fail") || strings.EqualFold(strings.TrimSpace(result.Verdict), "fail") {
			return true
		}
		if result.Passed != nil && !*result.Passed {
			return true
		}
	}
	return false
}

func llmJudgeState(result LLMJudgeResult) string {
	if result.NormalizedScore != nil && *result.NormalizedScore < 1 {
		return "fail"
	}
	if strings.EqualFold(strings.TrimSpace(result.State), "fail") || strings.EqualFold(strings.TrimSpace(result.Verdict), "fail") {
		return "fail"
	}
	if result.Passed != nil && !*result.Passed {
		return "fail"
	}
	if strings.TrimSpace(result.Reason) != "" {
		return "warning"
	}
	return "pass"
}

func judgeState(result JudgeResult) string {
	if result.Verdict == nil {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(*result.Verdict), "pass") {
		return "pass"
	}
	return "fail"
}

func severityFor(class FailureClass, state FailureState, evidenceTier EvidenceTier) Severity {
	if state == FailureStateIncompleteEvidence {
		return SeverityInfo
	}
	if evidenceTier == EvidenceTierHostedBlackBox || evidenceTier == EvidenceTierDerivedSummary || evidenceTier == EvidenceTierNone {
		switch class {
		case FailureClassPolicyViolation, FailureClassTimeoutOrBudget, FailureClassSandboxFailure, FailureClassMalformedOutput:
			return SeverityWarning
		}
	}
	switch class {
	case FailureClassPolicyViolation, FailureClassTimeoutOrBudget, FailureClassSandboxFailure, FailureClassMalformedOutput:
		return SeverityBlocking
	case FailureClassIncorrectFinalOutput, FailureClassToolArgumentError, FailureClassToolSelectionError, FailureClassRetrievalGrounding, FailureClassOther:
		return SeverityWarning
	case FailureClassInsufficientEvidence:
		return SeverityInfo
	case FailureClassFlakyNonDeterministic:
		return SeverityWarning
	}
	return SeverityWarning
}

func buildHeadline(caseCtx CaseContext, class FailureClass, failedDimensions []string) string {
	if len(failedDimensions) > 0 {
		return fmt.Sprintf("%s regressed on %s", caseCtx.ChallengeKey, strings.Join(failedDimensions, ", "))
	}
	return fmt.Sprintf("%s triggered %s", caseCtx.ChallengeKey, class)
}

func buildDetail(caseCtx CaseContext, failedChecks []string, evidenceTier EvidenceTier) string {
	if len(failedChecks) == 0 {
		return fmt.Sprintf("Case %s was flagged with %s evidence.", caseCtx.CaseKey, evidenceTier)
	}
	return fmt.Sprintf("Case %s failed checks %s with %s evidence.", caseCtx.CaseKey, strings.Join(failedChecks, ", "), evidenceTier)
}

func recommendedActionForClass(class FailureClass) string {
	switch class {
	case FailureClassPolicyViolation:
		return "Tighten the agent or tool policy before promoting this failure."
	case FailureClassToolArgumentError, FailureClassToolSelectionError:
		return "Inspect the replay around the failing tool call and correct the selection or arguments."
	case FailureClassTimeoutOrBudget:
		return "Review runtime limits, tool-call counts, and long-running steps."
	case FailureClassSandboxFailure:
		return "Inspect sandbox command events and environment assumptions before retrying."
	case FailureClassMalformedOutput:
		return "Validate the final-output contract and any JSON/schema formatting logic."
	case FailureClassIncorrectFinalOutput:
		return "Review the final answer and supporting evidence before promoting."
	case FailureClassInsufficientEvidence:
		return "Capture stronger structured evidence before promoting this case."
	default:
		return "Inspect the replay and scorecard evidence to decide whether this should become a regression case."
	}
}

func finalOutputReplayRef(events []Event) *ReplayStepRef {
	for _, event := range events {
		if event.EventType == "system.output.finalized" {
			return &ReplayStepRef{SequenceNumber: event.SequenceNumber, EventType: event.EventType, Kind: "final_output"}
		}
	}
	for _, event := range events {
		if event.EventType == "system.run.completed" && containsAny(lowerPayload(event.Payload), "final_output") {
			return &ReplayStepRef{SequenceNumber: event.SequenceNumber, EventType: event.EventType, Kind: "run_event"}
		}
	}
	return nil
}

func buildArtifactRefs(artifacts []ArtifactContext) []ArtifactRef {
	if len(artifacts) == 0 {
		return []ArtifactRef{}
	}
	refs := make([]ArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		refs = append(refs, ArtifactRef{
			Key:       artifact.Key,
			Kind:      artifact.Kind,
			Path:      artifact.Path,
			MediaType: artifact.MediaType,
		})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		return refs[i].Key < refs[j].Key
	})
	return refs
}

func dedupStrings(values *[]string) {
	if len(*values) == 0 {
		return
	}
	sort.Strings(*values)
	filtered := (*values)[:0]
	var last string
	for i, value := range *values {
		if i == 0 || value != last {
			filtered = append(filtered, value)
			last = value
		}
	}
	*values = filtered
}

func dedupReplayRefs(values *[]ReplayStepRef) {
	if len(*values) == 0 {
		return
	}
	sort.SliceStable(*values, func(i, j int) bool {
		if (*values)[i].SequenceNumber == (*values)[j].SequenceNumber {
			return (*values)[i].EventType < (*values)[j].EventType
		}
		return (*values)[i].SequenceNumber < (*values)[j].SequenceNumber
	})
	filtered := (*values)[:0]
	var last ReplayStepRef
	for i, value := range *values {
		if i == 0 || value.SequenceNumber != last.SequenceNumber || value.EventType != last.EventType {
			filtered = append(filtered, value)
			last = value
		}
	}
	*values = filtered
}

func compareSortKeys(left, right CursorKey) int {
	switch {
	case left.RunAgentID != right.RunAgentID:
		return strings.Compare(left.RunAgentID, right.RunAgentID)
	case left.ChallengeID != right.ChallengeID:
		return strings.Compare(left.ChallengeID, right.ChallengeID)
	case left.ChallengeKey != right.ChallengeKey:
		return strings.Compare(left.ChallengeKey, right.ChallengeKey)
	case left.CaseKey != right.CaseKey:
		return strings.Compare(left.CaseKey, right.CaseKey)
	default:
		return strings.Compare(left.ItemKey, right.ItemKey)
	}
}

func lowerPayload(payload json.RawMessage) string {
	return strings.ToLower(strings.TrimSpace(string(payload)))
}

func CursorForItem(item Item) CursorKey {
	return item.sortKey
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func cloneUUID(value *uuid.UUID) *uuid.UUID {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}

func uuidString(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
