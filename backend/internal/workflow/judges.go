package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/Atharva-Kanherkar/agentclash/backend/internal/provider"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/repository"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/scoring"
)

const defaultJudgeTimeout = 60 * time.Second

type judgeCallRecord struct {
	Model        string   `json:"model"`
	ProviderKey  string   `json:"provider_key"`
	SampleIndex  int      `json:"sample_index"`
	Score        *float64 `json:"score,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
	Error        string   `json:"error,omitempty"`
	ResponseText string   `json:"response_text,omitempty"`
}

type nwiseCandidate struct {
	RunAgentID     string   `json:"run_agent_id"`
	Label          string   `json:"label"`
	LaneIndex      int32    `json:"lane_index"`
	ContextEntries []string `json:"context_entries"`
}

func evaluateLLMJudges(
	ctx context.Context,
	client provider.Client,
	repo RunRepository,
	executionContext repository.RunAgentExecutionContext,
	input scoring.EvaluationInput,
	spec scoring.EvaluationSpec,
) ([]scoring.LLMJudgeResult, []string) {
	if len(spec.LLMJudges) == 0 {
		return nil, nil
	}

	results := make([]scoring.LLMJudgeResult, 0, len(spec.LLMJudges))
	warnings := make([]string, 0)
	for _, judge := range spec.LLMJudges {
		result, judgeWarnings := evaluateSingleLLMJudge(ctx, client, repo, executionContext, input, judge)
		results = append(results, result)
		warnings = append(warnings, judgeWarnings...)
	}

	sort.SliceStable(results, func(i, j int) bool {
		return results[i].JudgeKey < results[j].JudgeKey
	})
	return results, warnings
}

func evaluateSingleLLMJudge(
	ctx context.Context,
	client provider.Client,
	repo RunRepository,
	executionContext repository.RunAgentExecutionContext,
	input scoring.EvaluationInput,
	judge scoring.LLMJudgeDeclaration,
) (scoring.LLMJudgeResult, []string) {
	result := scoring.LLMJudgeResult{
		JudgeKey:   judge.Key,
		Mode:       string(judge.Mode),
		ModelCount: int32(len(judgeModels(judge))),
	}

	if client == nil {
		result.Reason = "judge provider client is not configured"
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q skipped: %s", judge.Key, result.Reason)}
	}
	if judge.Mode == scoring.JudgeMethodNWise {
		return evaluateSingleNWiseJudge(ctx, client, repo, executionContext, judge)
	}

	contextValues, reason, err := resolveJudgeContextValues(input, judge)
	if err != nil {
		result.Reason = err.Error()
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q errored: %v", judge.Key, err)}
	}
	if reason != "" {
		result.Reason = reason
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q unavailable: %s", judge.Key, reason)}
	}

	models := judgeModels(judge)
	result.SampleCount = int32(judge.Samples * len(models))
	callRecords := make([]judgeCallRecord, 0, len(models)*judge.Samples)
	modelScores := make(map[string]float64, len(models))
	successfulScores := make([]float64, 0, len(models)*judge.Samples)
	warnings := make([]string, 0)

	for _, model := range models {
		providerKey, providerAccountID, credentialReference, err := resolveJudgeTarget(model, executionContext)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("llm judge %q model %q: %v", judge.Key, model, err))
			for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
				callRecords = append(callRecords, judgeCallRecord{
					Model:       model,
					ProviderKey: providerKey,
					SampleIndex: sampleIdx + 1,
					Error:       err.Error(),
				})
			}
			continue
		}

		runCtx := ctx
		if strings.HasPrefix(credentialReference, "workspace-secret://") {
			secrets, loadErr := repo.LoadWorkspaceSecrets(ctx, executionContext.Run.WorkspaceID)
			if loadErr != nil {
				err = fmt.Errorf("load workspace secrets: %w", loadErr)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q: %v", judge.Key, model, err))
				for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
					callRecords = append(callRecords, judgeCallRecord{
						Model:       model,
						ProviderKey: providerKey,
						SampleIndex: sampleIdx + 1,
						Error:       err.Error(),
					})
				}
				continue
			}
			runCtx = provider.WithWorkspaceSecrets(runCtx, secrets)
		}

		perModelScores := make([]float64, 0, judge.Samples)
		for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
			request := provider.Request{
				ProviderKey:         providerKey,
				ProviderAccountID:   providerAccountID,
				CredentialReference: credentialReference,
				Model:               model,
				StepTimeout:         judgeTimeout(judge),
				Messages:            buildJudgeMessages(judge, contextValues),
				Metadata: mustMarshalJSON(map[string]any{
					"run_id":             executionContext.Run.ID,
					"run_agent_id":       executionContext.RunAgent.ID,
					"evaluation_spec_id": input.EvaluationSpecID,
					"judge_key":          judge.Key,
					"judge_mode":         judge.Mode,
					"judge_model":        model,
					"judge_sample_index": sampleIdx + 1,
				}),
			}

			response, invokeErr := client.InvokeModel(runCtx, request)
			record := judgeCallRecord{
				Model:        model,
				ProviderKey:  providerKey,
				SampleIndex:  sampleIdx + 1,
				ResponseText: response.OutputText,
			}
			if invokeErr != nil {
				record.Error = invokeErr.Error()
				callRecords = append(callRecords, record)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q sample %d: %v", judge.Key, model, sampleIdx+1, invokeErr))
				continue
			}

			score, confidence, parseErr := parseJudgeScore(judge, response.OutputText)
			if parseErr != nil {
				record.Error = parseErr.Error()
				callRecords = append(callRecords, record)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q sample %d: %v", judge.Key, model, sampleIdx+1, parseErr))
				continue
			}

			record.Score = &score
			record.Confidence = confidence
			callRecords = append(callRecords, record)
			perModelScores = append(perModelScores, score)
			successfulScores = append(successfulScores, score)
		}

		if aggregated, ok := aggregateSamplesForMode(judge.Mode, perModelScores); ok {
			modelScores[model] = aggregated
		}
	}

	reason = ""
	if len(modelScores) == 0 {
		reason = "all judge invocations failed or returned unparsable output"
		result.Reason = reason
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":                  judge.Mode,
			"reason":                reason,
			"calls":                 callRecords,
			"unable_to_judge_count": len(callRecords),
			"warnings":              warnings,
		})
		return result, warnings
	}

	aggregatedScore := aggregateModelScores(judge, modelScores)
	result.NormalizedScore = &aggregatedScore
	if variance, ok := sampleVariance(successfulScores); ok {
		result.Variance = &variance
	}
	if confidence := deriveJudgeConfidence(judge, modelScores, successfulScores, len(warnings) > 0); confidence != "" {
		result.Confidence = &confidence
	}
	result.Payload = mustMarshalJSON(map[string]any{
		"mode":             judge.Mode,
		"calls":            callRecords,
		"model_scores":     modelScores,
		"aggregated_score": aggregatedScore,
		"warnings":         warnings,
	})
	return result, warnings
}

func evaluateSingleNWiseJudge(
	ctx context.Context,
	client provider.Client,
	repo RunRepository,
	executionContext repository.RunAgentExecutionContext,
	judge scoring.LLMJudgeDeclaration,
) (scoring.LLMJudgeResult, []string) {
	result := scoring.LLMJudgeResult{
		JudgeKey:   judge.Key,
		Mode:       string(judge.Mode),
		ModelCount: int32(len(judgeModels(judge))),
	}

	candidates, reason, err := buildNWiseCandidates(ctx, repo, executionContext, judge)
	if err != nil {
		result.Reason = err.Error()
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q errored: %v", judge.Key, err)}
	}
	if reason != "" {
		result.Reason = reason
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q unavailable: %s", judge.Key, reason)}
	}
	if len(candidates) < 2 {
		result.Reason = "n_wise judges require at least two run agents in the run"
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":   judge.Mode,
			"reason": result.Reason,
		})
		return result, []string{fmt.Sprintf("llm judge %q unavailable: %s", judge.Key, result.Reason)}
	}

	currentRunAgentID := executionContext.RunAgent.ID.String()
	models := judgeModels(judge)
	result.SampleCount = int32(judge.Samples * len(models))
	callRecords := make([]judgeCallRecord, 0, len(models)*judge.Samples)
	modelScores := make(map[string]float64, len(models))
	successfulScores := make([]float64, 0, len(models)*judge.Samples)
	warnings := make([]string, 0)

	for _, model := range models {
		providerKey, providerAccountID, credentialReference, err := resolveJudgeTarget(model, executionContext)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("llm judge %q model %q: %v", judge.Key, model, err))
			for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
				callRecords = append(callRecords, judgeCallRecord{
					Model:       model,
					ProviderKey: providerKey,
					SampleIndex: sampleIdx + 1,
					Error:       err.Error(),
				})
			}
			continue
		}

		runCtx := ctx
		if strings.HasPrefix(credentialReference, "workspace-secret://") {
			secrets, loadErr := repo.LoadWorkspaceSecrets(ctx, executionContext.Run.WorkspaceID)
			if loadErr != nil {
				err = fmt.Errorf("load workspace secrets: %w", loadErr)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q: %v", judge.Key, model, err))
				for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
					callRecords = append(callRecords, judgeCallRecord{
						Model:       model,
						ProviderKey: providerKey,
						SampleIndex: sampleIdx + 1,
						Error:       err.Error(),
					})
				}
				continue
			}
			runCtx = provider.WithWorkspaceSecrets(runCtx, secrets)
		}

		perModelScores := make([]float64, 0, judge.Samples)
		for sampleIdx := 0; sampleIdx < judge.Samples; sampleIdx++ {
			orderedCandidates := rotateNWiseCandidates(candidates, sampleIdx, judge.PositionDebiasing)
			request := provider.Request{
				ProviderKey:         providerKey,
				ProviderAccountID:   providerAccountID,
				CredentialReference: credentialReference,
				Model:               model,
				StepTimeout:         judgeTimeout(judge),
				Messages:            buildNWiseJudgeMessages(judge, orderedCandidates),
				Metadata: mustMarshalJSON(map[string]any{
					"run_id":             executionContext.Run.ID,
					"run_agent_id":       executionContext.RunAgent.ID,
					"evaluation_spec_id": executionContext.ChallengePackVersion.ID,
					"judge_key":          judge.Key,
					"judge_mode":         judge.Mode,
					"judge_model":        model,
					"judge_sample_index": sampleIdx + 1,
					"candidate_count":    len(orderedCandidates),
				}),
			}

			response, invokeErr := client.InvokeModel(runCtx, request)
			record := judgeCallRecord{
				Model:        model,
				ProviderKey:  providerKey,
				SampleIndex:  sampleIdx + 1,
				ResponseText: response.OutputText,
			}
			if invokeErr != nil {
				record.Error = invokeErr.Error()
				callRecords = append(callRecords, record)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q sample %d: %v", judge.Key, model, sampleIdx+1, invokeErr))
				continue
			}

			score, confidence, parseErr := parseNWiseScore(currentRunAgentID, orderedCandidates, response.OutputText)
			if parseErr != nil {
				record.Error = parseErr.Error()
				callRecords = append(callRecords, record)
				warnings = append(warnings, fmt.Sprintf("llm judge %q model %q sample %d: %v", judge.Key, model, sampleIdx+1, parseErr))
				continue
			}

			record.Score = &score
			record.Confidence = confidence
			callRecords = append(callRecords, record)
			perModelScores = append(perModelScores, score)
			successfulScores = append(successfulScores, score)
		}

		if aggregated, ok := aggregateSamplesForMode(judge.Mode, perModelScores); ok {
			modelScores[model] = aggregated
		}
	}

	if len(modelScores) == 0 {
		result.Reason = "all judge invocations failed or returned unparsable output"
		result.Payload = mustMarshalJSON(map[string]any{
			"mode":                  judge.Mode,
			"reason":                result.Reason,
			"calls":                 callRecords,
			"unable_to_judge_count": len(callRecords),
			"candidates":            candidates,
			"warnings":              warnings,
		})
		return result, warnings
	}

	aggregatedScore := aggregateModelScores(judge, modelScores)
	result.NormalizedScore = &aggregatedScore
	if variance, ok := sampleVariance(successfulScores); ok {
		result.Variance = &variance
	}
	if confidence := deriveJudgeConfidence(judge, modelScores, successfulScores, len(warnings) > 0); confidence != "" {
		result.Confidence = &confidence
	}
	result.Payload = mustMarshalJSON(map[string]any{
		"mode":             judge.Mode,
		"calls":            callRecords,
		"model_scores":     modelScores,
		"aggregated_score": aggregatedScore,
		"candidates":       candidates,
		"warnings":         warnings,
	})
	return result, warnings
}

func buildNWiseCandidates(
	ctx context.Context,
	repo RunRepository,
	executionContext repository.RunAgentExecutionContext,
	judge scoring.LLMJudgeDeclaration,
) ([]nwiseCandidate, string, error) {
	runAgents, err := repo.ListRunAgentsByRunID(ctx, executionContext.Run.ID)
	if err != nil {
		return nil, "", fmt.Errorf("list run agents for n_wise judge: %w", err)
	}
	sort.SliceStable(runAgents, func(i, j int) bool {
		if runAgents[i].LaneIndex == runAgents[j].LaneIndex {
			return runAgents[i].ID.String() < runAgents[j].ID.String()
		}
		return runAgents[i].LaneIndex < runAgents[j].LaneIndex
	})

	candidates := make([]nwiseCandidate, 0, len(runAgents))
	for _, runAgent := range runAgents {
		agentExecutionContext, err := repo.GetRunAgentExecutionContextByID(ctx, runAgent.ID)
		if err != nil {
			return nil, "", fmt.Errorf("load n_wise run-agent execution context %s: %w", runAgent.ID, err)
		}
		events, err := repo.ListRunEventsByRunAgentID(ctx, runAgent.ID)
		if err != nil {
			return nil, "", fmt.Errorf("list n_wise run-agent events %s: %w", runAgent.ID, err)
		}
		challengeInputs, err := mapChallengeInputs(agentExecutionContext.ChallengePackVersion.Manifest, agentExecutionContext.ChallengeInputSet)
		if err != nil {
			return nil, "", fmt.Errorf("map n_wise challenge inputs for %s: %w", runAgent.ID, err)
		}
		contextValues, reason, err := resolveJudgeContextValues(scoring.EvaluationInput{
			RunAgentID:       runAgent.ID,
			EvaluationSpecID: executionContext.ChallengePackVersion.ID,
			ChallengeInputs:  challengeInputs,
			Events:           mapRunEvents(events),
		}, judge)
		if err != nil {
			return nil, "", fmt.Errorf("resolve n_wise context for %s: %w", runAgent.ID, err)
		}
		if reason != "" {
			return nil, fmt.Sprintf("judge context unavailable for run agent %s: %s", runAgent.ID, reason), nil
		}
		candidates = append(candidates, nwiseCandidate{
			RunAgentID:     runAgent.ID.String(),
			Label:          firstNonEmpty(strings.TrimSpace(runAgent.Label), fmt.Sprintf("lane-%d", runAgent.LaneIndex)),
			LaneIndex:      runAgent.LaneIndex,
			ContextEntries: contextValues,
		})
	}
	return candidates, "", nil
}

func rotateNWiseCandidates(candidates []nwiseCandidate, sampleIdx int, enabled bool) []nwiseCandidate {
	rotated := append([]nwiseCandidate(nil), candidates...)
	if !enabled || len(rotated) == 0 {
		return rotated
	}
	shift := sampleIdx % len(rotated)
	if shift == 0 {
		return rotated
	}
	return append(rotated[shift:], rotated[:shift]...)
}

func buildNWiseJudgeMessages(judge scoring.LLMJudgeDeclaration, candidates []nwiseCandidate) []provider.Message {
	body := []string{
		"You are an evaluation judge for autonomous-agent runs.",
		"Return only valid JSON. Do not wrap the response in markdown fences.",
		`Response schema: {"ranking": ["<run_agent_id>", "..."], "confidence": "low|medium|high", "reasoning": "<brief rationale>"}`,
		"Task:",
		judge.Prompt,
		"Rank every candidate from best to worst. Include every candidate exactly once using run_agent_id values only.",
		"Candidates:",
	}
	for idx, candidate := range candidates {
		body = append(body,
			fmt.Sprintf("Candidate %d", idx+1),
			fmt.Sprintf("run_agent_id: %s", candidate.RunAgentID),
			fmt.Sprintf("label: %s", candidate.Label),
			fmt.Sprintf("lane_index: %d", candidate.LaneIndex),
			strings.Join(candidate.ContextEntries, "\n\n"),
		)
	}
	if len(judge.AntiGamingClauses) > 0 {
		body = append(body, "Additional anti-gaming clauses:")
		body = append(body, judge.AntiGamingClauses...)
	}
	return []provider.Message{{
		Role:    "user",
		Content: strings.Join(body, "\n\n"),
	}}
}

func parseNWiseScore(currentRunAgentID string, candidates []nwiseCandidate, raw string) (float64, string, error) {
	normalized := sanitizeJudgeJSON(raw)
	var parsed struct {
		Ranking    []string `json:"ranking"`
		RankedIDs  []string `json:"ranked_ids"`
		Confidence string   `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(normalized), &parsed); err != nil {
		return 0, "", fmt.Errorf("parse n_wise judge response: %w", err)
	}
	ranking := parsed.Ranking
	if len(ranking) == 0 {
		ranking = parsed.RankedIDs
	}
	if len(ranking) == 0 {
		return 0, "", fmt.Errorf("n_wise judge response did not include a ranking array")
	}

	validIDs := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		validIDs[candidate.RunAgentID] = struct{}{}
	}
	seen := make(map[string]struct{}, len(ranking))
	filtered := make([]string, 0, len(ranking))
	for _, candidateID := range ranking {
		candidateID = strings.TrimSpace(candidateID)
		if candidateID == "" {
			continue
		}
		if _, ok := validIDs[candidateID]; !ok {
			return 0, "", fmt.Errorf("n_wise judge returned unknown candidate id %q", candidateID)
		}
		if _, ok := seen[candidateID]; ok {
			return 0, "", fmt.Errorf("n_wise judge returned duplicate candidate id %q", candidateID)
		}
		seen[candidateID] = struct{}{}
		filtered = append(filtered, candidateID)
	}
	if len(filtered) != len(candidates) {
		return 0, "", fmt.Errorf("n_wise judge ranked %d candidates, want %d", len(filtered), len(candidates))
	}

	position := -1
	for idx, candidateID := range filtered {
		if candidateID == currentRunAgentID {
			position = idx
			break
		}
	}
	if position < 0 {
		return 0, "", fmt.Errorf("n_wise judge omitted current run agent %s", currentRunAgentID)
	}
	if len(filtered) == 1 {
		return 1, normalizeJudgeConfidence(parsed.Confidence), nil
	}
	borda := float64(len(filtered)-1-position) / float64(len(filtered)-1)
	return borda, normalizeJudgeConfidence(parsed.Confidence), nil
}

func resolveJudgeContextValues(input scoring.EvaluationInput, judge scoring.LLMJudgeDeclaration) ([]string, string, error) {
	values := make([]string, 0, len(judge.ContextFrom)+1)
	for _, ref := range judge.ContextFrom {
		value, reason, err := scoring.ResolveEvidenceValueForJudge(ref, input)
		if err != nil {
			return nil, "", err
		}
		if value == nil {
			return nil, firstNonEmpty(reason, fmt.Sprintf("judge context %q is unavailable", ref)), nil
		}
		values = append(values, fmt.Sprintf("%s:\n%s", ref, *value))
	}
	if judge.Mode == scoring.JudgeMethodReference {
		value, reason, err := scoring.ResolveEvidenceValueForJudge(judge.ReferenceFrom, input)
		if err != nil {
			return nil, "", err
		}
		if value == nil {
			return nil, firstNonEmpty(reason, fmt.Sprintf("reference evidence %q is unavailable", judge.ReferenceFrom)), nil
		}
		values = append(values, fmt.Sprintf("reference_answer:\n%s", *value))
	}
	return values, "", nil
}

func judgeModels(judge scoring.LLMJudgeDeclaration) []string {
	if strings.TrimSpace(judge.Model) != "" {
		return []string{strings.TrimSpace(judge.Model)}
	}
	models := make([]string, 0, len(judge.Models))
	for _, model := range judge.Models {
		trimmed := strings.TrimSpace(model)
		if trimmed == "" {
			continue
		}
		models = append(models, trimmed)
	}
	return models
}

func resolveJudgeTarget(model string, executionContext repository.RunAgentExecutionContext) (string, string, string, error) {
	providerKey := inferJudgeProviderKey(model)
	if providerKey == "" && executionContext.Deployment.ProviderAccount != nil {
		providerKey = executionContext.Deployment.ProviderAccount.ProviderKey
	}
	if providerKey == "" {
		return "", "", "", fmt.Errorf("cannot infer provider for judge model %q", model)
	}

	if account := executionContext.Deployment.ProviderAccount; account != nil && account.ProviderKey == providerKey {
		return providerKey, account.ID.String(), account.CredentialReference, nil
	}

	credentialReference, ok := defaultJudgeCredentialReference(providerKey)
	if !ok {
		return providerKey, "", "", fmt.Errorf("no default credential reference configured for judge provider %q", providerKey)
	}
	return providerKey, "", credentialReference, nil
}

func inferJudgeProviderKey(model string) string {
	trimmed := strings.ToLower(strings.TrimSpace(model))
	switch {
	case strings.HasPrefix(trimmed, "claude-"):
		return "anthropic"
	case strings.HasPrefix(trimmed, "gpt-"),
		strings.HasPrefix(trimmed, "o1"),
		strings.HasPrefix(trimmed, "o3"),
		strings.HasPrefix(trimmed, "o4"),
		strings.HasPrefix(trimmed, "text-embedding"):
		return "openai"
	case strings.HasPrefix(trimmed, "gemini-"):
		return "gemini"
	case strings.HasPrefix(trimmed, "grok-"):
		return "xai"
	default:
		return ""
	}
}

func defaultJudgeCredentialReference(providerKey string) (string, bool) {
	switch strings.TrimSpace(providerKey) {
	case "anthropic":
		return "env://ANTHROPIC_API_KEY", true
	case "openai":
		return "env://OPENAI_API_KEY", true
	case "gemini":
		return "env://GEMINI_API_KEY", true
	case "xai":
		return "env://XAI_API_KEY", true
	default:
		return "", false
	}
}

func judgeTimeout(judge scoring.LLMJudgeDeclaration) time.Duration {
	if judge.TimeoutMS != nil && *judge.TimeoutMS > 0 {
		return time.Duration(*judge.TimeoutMS) * time.Millisecond
	}
	return defaultJudgeTimeout
}

func buildJudgeMessages(judge scoring.LLMJudgeDeclaration, contextValues []string) []provider.Message {
	responseFormat := `{"score": <number>, "confidence": "low|medium|high", "reasoning": "<brief rationale>"}`
	switch judge.Mode {
	case scoring.JudgeMethodAssertion:
		responseFormat = `{"pass": true|false, "confidence": "low|medium|high", "reasoning": "<brief rationale>"}`
	}

	instructions := []string{
		"You are an evaluation judge for autonomous-agent runs.",
		"Return only valid JSON. Do not wrap the response in markdown fences.",
		fmt.Sprintf("Response schema: %s", responseFormat),
	}

	body := make([]string, 0, len(contextValues)+4)
	body = append(body, instructions...)
	switch judge.Mode {
	case scoring.JudgeMethodAssertion:
		body = append(body, "Assertion:")
		body = append(body, judge.Assertion)
	case scoring.JudgeMethodRubric:
		body = append(body, "Rubric:")
		body = append(body, judge.Rubric)
	case scoring.JudgeMethodReference:
		body = append(body, "Reference-scoring rubric:")
		body = append(body, judge.Rubric)
		body = append(body, fmt.Sprintf("Score using the configured range %.2f to %.2f.", judgeScoreScale(judge).Min, judgeScoreScale(judge).Max))
	}
	body = append(body, "Evaluation context:")
	body = append(body, contextValues...)
	if len(judge.AntiGamingClauses) > 0 {
		body = append(body, "Additional anti-gaming clauses:")
		body = append(body, judge.AntiGamingClauses...)
	}

	return []provider.Message{
		{
			Role:    "user",
			Content: strings.Join(body, "\n\n"),
		},
	}
}

func parseJudgeScore(judge scoring.LLMJudgeDeclaration, raw string) (float64, string, error) {
	normalized := sanitizeJudgeJSON(raw)
	switch judge.Mode {
	case scoring.JudgeMethodAssertion:
		var parsed struct {
			Pass       bool   `json:"pass"`
			Verdict    string `json:"verdict"`
			Confidence string `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(normalized), &parsed); err != nil {
			return 0, "", fmt.Errorf("parse assertion judge response: %w", err)
		}
		pass := parsed.Pass
		switch strings.ToLower(strings.TrimSpace(parsed.Verdict)) {
		case "pass", "true", "yes":
			pass = true
		case "fail", "false", "no":
			pass = false
		}
		if judge.Expect != nil && !*judge.Expect {
			pass = !pass
		}
		if pass {
			return 1, normalizeJudgeConfidence(parsed.Confidence), nil
		}
		return 0, normalizeJudgeConfidence(parsed.Confidence), nil
	case scoring.JudgeMethodRubric, scoring.JudgeMethodReference:
		var parsed struct {
			Score      float64 `json:"score"`
			Confidence string  `json:"confidence"`
		}
		if err := json.Unmarshal([]byte(normalized), &parsed); err != nil {
			return 0, "", fmt.Errorf("parse rubric judge response: %w", err)
		}
		scale := judgeScoreScale(judge)
		score := parsed.Score
		if math.IsNaN(score) || math.IsInf(score, 0) {
			return 0, "", fmt.Errorf("judge score is not finite")
		}
		if score < scale.Min {
			score = scale.Min
		}
		if score > scale.Max {
			score = scale.Max
		}
		normalizedScore := (score - scale.Min) / (scale.Max - scale.Min)
		return normalizedScore, normalizeJudgeConfidence(parsed.Confidence), nil
	default:
		return 0, "", fmt.Errorf("unsupported judge mode %q", judge.Mode)
	}
}

func judgeScoreScale(judge scoring.LLMJudgeDeclaration) scoring.ScoreScale {
	if judge.ScoreScale != nil {
		return *judge.ScoreScale
	}
	return scoring.ScoreScale{Min: 1, Max: 5}
}

func sanitizeJudgeJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```JSON")
		trimmed = strings.TrimPrefix(trimmed, "```")
		if idx := strings.LastIndex(trimmed, "```"); idx >= 0 {
			trimmed = trimmed[:idx]
		}
		trimmed = strings.TrimSpace(trimmed)
	}
	if json.Valid([]byte(trimmed)) {
		return trimmed
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		candidate := strings.TrimSpace(trimmed[start : end+1])
		if json.Valid([]byte(candidate)) {
			return candidate
		}
	}
	return trimmed
}

func aggregateSamplesForMode(mode scoring.JudgeMethodMode, scores []float64) (float64, bool) {
	if len(scores) == 0 {
		return 0, false
	}
	switch mode {
	case scoring.JudgeMethodAssertion:
		passCount := 0
		for _, score := range scores {
			if score >= 0.5 {
				passCount++
			}
		}
		if passCount*2 >= len(scores) {
			return 1, true
		}
		return 0, true
	default:
		return median(scores), true
	}
}

func aggregateModelScores(judge scoring.LLMJudgeDeclaration, modelScores map[string]float64) float64 {
	values := make([]float64, 0, len(modelScores))
	for _, value := range modelScores {
		values = append(values, value)
	}
	if len(values) == 1 {
		return values[0]
	}

	aggregation := scoring.ConsensusAggMean
	if judge.Mode.IsBooleanScope() {
		aggregation = scoring.ConsensusAggMajorityVote
	}
	if judge.Consensus != nil {
		aggregation = judge.Consensus.Aggregation
	}

	switch aggregation {
	case scoring.ConsensusAggMedian:
		return median(values)
	case scoring.ConsensusAggMajorityVote:
		passCount := 0
		for _, value := range values {
			if value >= 0.5 {
				passCount++
			}
		}
		if passCount*2 >= len(values) {
			return 1
		}
		return 0
	case scoring.ConsensusAggUnanimous:
		for _, value := range values {
			if value < 0.5 {
				return 0
			}
		}
		return 1
	default:
		return mean(values)
	}
}

func deriveJudgeConfidence(judge scoring.LLMJudgeDeclaration, modelScores map[string]float64, allScores []float64, hadWarnings bool) string {
	values := make([]float64, 0, len(modelScores))
	for _, value := range modelScores {
		values = append(values, value)
	}
	if len(values) == 0 {
		return ""
	}
	spread := max(values) - min(values)
	if len(values) == 1 && len(allScores) > 1 {
		spread = max(allScores) - min(allScores)
	}
	threshold := 0.25
	if judge.Consensus != nil && judge.Consensus.MinAgreementThreshold > 0 {
		threshold = judge.Consensus.MinAgreementThreshold
	}
	switch {
	case hadWarnings:
		return "low"
	case spread == 0:
		return "high"
	case spread > threshold:
		return "low"
	default:
		return "medium"
	}
}

func normalizeJudgeConfidence(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "high", "medium", "low":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return ""
	}
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func median(values []float64) float64 {
	cloned := append([]float64(nil), values...)
	sort.Float64s(cloned)
	mid := len(cloned) / 2
	if len(cloned)%2 == 1 {
		return cloned[mid]
	}
	return (cloned[mid-1] + cloned[mid]) / 2
}

func min(values []float64) float64 {
	minValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}

func max(values []float64) float64 {
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func sampleVariance(values []float64) (float64, bool) {
	if len(values) < 2 {
		return 0, false
	}
	avg := mean(values)
	total := 0.0
	for _, value := range values {
		diff := value - avg
		total += diff * diff
	}
	return total / float64(len(values)), true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
