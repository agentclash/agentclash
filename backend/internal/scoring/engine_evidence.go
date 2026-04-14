package scoring

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
)

type extractedEvidence struct {
	finalOutput               *string
	finalOutputChallengeID    *uuid.UUID
	challengeInputValue       *string
	challengeInputChallengeID *uuid.UUID
	caseInput                 *EvidenceInput
	caseInputReason           string
	startedAt                 *time.Time
	firstOutputAt             *time.Time
	terminalAt                *time.Time
	completedSuccessfully     *bool
	failureCount              int
	inputTokens               *float64
	outputTokens              *float64
	totalTokens               *float64
	modelUsage                []pricedUsage
	observedModels            []modelRef
	stepDurations             []stepDurationEvidence
	capturedFiles             map[string]FileCaptureResult
	capturedDirListings       map[string]DirectoryListingResult
	codeExecutionResults      map[string]CodeExecutionResult
	warnings                  []string
}

func buildEvidence(challengeInputs []EvidenceInput, events []Event) extractedEvidence {
	evidence := extractedEvidence{}
	evidence.challengeInputValue, evidence.challengeInputChallengeID, evidence.warnings = resolveChallengeInputValue(challengeInputs)
	evidence.caseInput, evidence.caseInputReason = resolveCaseInput(challengeInputs)

	var (
		inputFromCalls  float64
		outputFromCalls float64
		totalFromCalls  float64
		usageFromCalls  bool
		stepStartedAt   = map[int]time.Time{}
		usageByModel    = map[string]*pricedUsage{}
		seenModels      = map[string]modelRef{}
	)

	for _, event := range events {
		payload := decodePayload(event.Payload)
		switch event.Type {
		case "system.run.started":
			if evidence.startedAt == nil {
				occurredAt := event.OccurredAt.UTC()
				evidence.startedAt = &occurredAt
			}
		case "system.step.started":
			if stepIndex, ok := intValue(payload, "step_index"); ok {
				stepStartedAt[stepIndex] = event.OccurredAt.UTC()
			}
		case "system.step.completed":
			stepIndex, ok := intValue(payload, "step_index")
			if !ok {
				break
			}
			startedAt, ok := stepStartedAt[stepIndex]
			if !ok {
				break
			}
			completedAt := event.OccurredAt.UTC()
			evidence.stepDurations = append(evidence.stepDurations, stepDurationEvidence{
				StepIndex:   stepIndex,
				DurationMS:  float64(completedAt.Sub(startedAt).Milliseconds()),
				StartedAt:   startedAt.Format(time.RFC3339Nano),
				CompletedAt: completedAt.Format(time.RFC3339Nano),
			})
		case "model.output.delta", "system.output.finalized":
			if evidence.firstOutputAt == nil {
				occurredAt := event.OccurredAt.UTC()
				evidence.firstOutputAt = &occurredAt
			}
			if event.Type == "system.output.finalized" && evidence.finalOutput == nil {
				if output, ok := stringValue(payload, "final_output"); ok {
					evidence.finalOutput = &output
				} else if output, ok := extractLooseString(payload["output"]); ok {
					evidence.finalOutput = &output
				}
			}
		case "system.run.completed":
			occurredAt := event.OccurredAt.UTC()
			evidence.terminalAt = &occurredAt
			completed := true
			evidence.completedSuccessfully = &completed
			if output, ok := stringValue(payload, "final_output"); ok {
				evidence.finalOutput = &output
			}
			if value, ok := numericValue(payload, "input_tokens"); ok {
				evidence.inputTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "output_tokens"); ok {
				evidence.outputTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "total_tokens"); ok {
				evidence.totalTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "input_tokens"); ok && evidence.inputTokens == nil {
				evidence.inputTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "output_tokens"); ok && evidence.outputTokens == nil {
				evidence.outputTokens = floatPtr(value)
			}
			if value, ok := usageValue(payload, "total_tokens"); ok && evidence.totalTokens == nil {
				evidence.totalTokens = floatPtr(value)
			}
			if value, ok := numericValue(payload, "latency_ms"); ok && evidence.startedAt != nil {
				evidence.terminalAt = timePtr(evidence.startedAt.Add(time.Duration(value) * time.Millisecond))
			}
		case "system.run.failed":
			occurredAt := event.OccurredAt.UTC()
			evidence.terminalAt = &occurredAt
			completed := false
			evidence.completedSuccessfully = &completed
			evidence.failureCount++
		case "tool.call.failed", "sandbox.command.failed":
			evidence.failureCount++
		case "model.call.completed":
			providerKey, _ := stringValue(payload, "provider_key")
			providerModelID, _ := stringValue(payload, "provider_model_id")
			if providerModelID == "" {
				providerModelID, _ = stringValue(payload, "model")
			}
			if providerKey != "" || providerModelID != "" {
				seenModels[providerKey+"\x00"+providerModelID] = modelRef{
					ProviderKey:     providerKey,
					ProviderModelID: providerModelID,
				}
			}
			if value, ok := usageValue(payload, "input_tokens"); ok {
				inputFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "input_tokens", value)
			}
			if value, ok := usageValue(payload, "output_tokens"); ok {
				outputFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "output_tokens", value)
			}
			if value, ok := usageValue(payload, "total_tokens"); ok {
				totalFromCalls += value
				usageFromCalls = true
				addModelUsage(usageByModel, providerKey, providerModelID, "total_tokens", value)
			}
		case "grader.verification.file_captured":
			var capture FileCaptureResult
			if err := json.Unmarshal(event.Payload, &capture); err == nil && capture.Key != "" {
				if evidence.capturedFiles == nil {
					evidence.capturedFiles = make(map[string]FileCaptureResult)
				}
				evidence.capturedFiles[capture.Key] = capture
			}
		case "grader.verification.directory_listed":
			var listing DirectoryListingResult
			if err := json.Unmarshal(event.Payload, &listing); err == nil && listing.Key != "" {
				if evidence.capturedDirListings == nil {
					evidence.capturedDirListings = make(map[string]DirectoryListingResult)
				}
				evidence.capturedDirListings[listing.Key] = listing
			}
		case "grader.verification.code_executed":
			var result CodeExecutionResult
			if err := json.Unmarshal(event.Payload, &result); err == nil && result.ValidatorKey != "" {
				if evidence.codeExecutionResults == nil {
					evidence.codeExecutionResults = make(map[string]CodeExecutionResult)
				}
				evidence.codeExecutionResults[result.ValidatorKey] = result
			}
		case "model.call.started":
			providerKey, _ := stringValue(payload, "provider_key")
			providerModelID, _ := stringValue(payload, "model")
			if providerModelID == "" {
				providerModelID, _ = stringValue(payload, "provider_model_id")
			}
			if providerKey != "" || providerModelID != "" {
				seenModels[providerKey+"\x00"+providerModelID] = modelRef{
					ProviderKey:     providerKey,
					ProviderModelID: providerModelID,
				}
			}
		}
	}

	if evidence.inputTokens == nil && usageFromCalls {
		evidence.inputTokens = floatPtr(inputFromCalls)
	}
	if evidence.outputTokens == nil && usageFromCalls {
		evidence.outputTokens = floatPtr(outputFromCalls)
	}
	if evidence.totalTokens == nil && usageFromCalls {
		if totalFromCalls > 0 {
			evidence.totalTokens = floatPtr(totalFromCalls)
		} else if evidence.inputTokens != nil && evidence.outputTokens != nil {
			evidence.totalTokens = floatPtr(*evidence.inputTokens + *evidence.outputTokens)
		}
	}
	for _, usage := range usageByModel {
		if usage.TotalTokens == 0 && (usage.InputTokens > 0 || usage.OutputTokens > 0) {
			usage.TotalTokens = usage.InputTokens + usage.OutputTokens
		}
		evidence.modelUsage = append(evidence.modelUsage, *usage)
	}
	for _, ref := range seenModels {
		evidence.observedModels = append(evidence.observedModels, ref)
	}
	sort.SliceStable(evidence.modelUsage, func(i, j int) bool {
		if evidence.modelUsage[i].ProviderKey == evidence.modelUsage[j].ProviderKey {
			return evidence.modelUsage[i].ProviderModelID < evidence.modelUsage[j].ProviderModelID
		}
		return evidence.modelUsage[i].ProviderKey < evidence.modelUsage[j].ProviderKey
	})
	sort.SliceStable(evidence.observedModels, func(i, j int) bool {
		if evidence.observedModels[i].ProviderKey == evidence.observedModels[j].ProviderKey {
			return evidence.observedModels[i].ProviderModelID < evidence.observedModels[j].ProviderModelID
		}
		return evidence.observedModels[i].ProviderKey < evidence.observedModels[j].ProviderKey
	})
	if evidence.finalOutput == nil {
		evidence.warnings = append(evidence.warnings, "final output evidence is unavailable")
	}
	if evidence.completedSuccessfully == nil {
		evidence.warnings = append(evidence.warnings, "terminal run evidence is unavailable")
	}

	return evidence
}
