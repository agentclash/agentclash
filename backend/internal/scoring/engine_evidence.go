package scoring

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/google/uuid"
)

// eventRef is a compact reference to a run event used to build validator/metric
// source pointers. Sequence is the addressable key (run_events are uniquely
// identified by (run_agent_id, sequence_number)); EventType is kept denormalized
// so the scorecard UI can label the link without re-fetching the event.
type eventRef struct {
	Sequence  int64
	EventType string
}

type extractedEvidence struct {
	finalOutput            *string
	finalOutputChallengeID *uuid.UUID
	// finalOutputSource is set only by the dedicated system.output.finalized
	// event (the narrow producer). It is never set from system.run.completed
	// — that event wraps every preceding event in the run and would make
	// deep-links land on the wrapper instead of the real producer.
	finalOutputSource *eventRef
	// lastModelCallSource tracks the most recent model.call.completed event
	// and is used as the final_output source when no system.output.finalized
	// event exists (i.e. native runs that don't synthesize a finalized event).
	// A nil source beats pointing at the run.completed wrapper.
	lastModelCallSource            *eventRef
	challengeInputValue            *string
	challengeInputChallengeID      *uuid.UUID
	challengeInputRegressionCaseID *uuid.UUID
	caseInput                      *EvidenceInput
	caseInputReason                string
	startedAt                      *time.Time
	firstOutputAt                  *time.Time
	terminalAt                     *time.Time
	completedSuccessfully          *bool
	failureCount                   int
	inputTokens                    *float64
	outputTokens                   *float64
	totalTokens                    *float64
	// raceContextTokens is the estimated prompt-side token spend from
	// race-context standings injections (issue #400). It is accumulated
	// from race.standings.injected events and must stay separate from
	// model-authored totals so billable-spend metrics are not inflated
	// by observational injections.
	raceContextTokens float64
	modelUsage                     []pricedUsage
	observedModels                 []modelRef
	stepDurations                  []stepDurationEvidence
	capturedFiles                  map[string]FileCaptureResult
	capturedFileSources            map[string]eventRef
	capturedDirListings            map[string]DirectoryListingResult
	capturedDirListingSources      map[string]eventRef
	codeExecutionResults           map[string]CodeExecutionResult
	codeExecutionSources           map[string]eventRef
	toolCallTrace                  []toolCallTraceEntry
	warnings                       []string
}

// eventRefFrom returns a reference to the given event, or nil when the event
// has no persisted sequence number (happens during unit tests that don't route
// through the repository, but never in production).
func eventRefFrom(event Event) *eventRef {
	if event.SequenceNumber <= 0 {
		return nil
	}
	return &eventRef{Sequence: event.SequenceNumber, EventType: event.Type}
}

func buildEvidence(challengeInputs []EvidenceInput, events []Event) extractedEvidence {
	evidence := extractedEvidence{}
	evidence.challengeInputValue, evidence.challengeInputChallengeID, evidence.challengeInputRegressionCaseID, evidence.warnings = resolveChallengeInputValue(challengeInputs)
	evidence.caseInput, evidence.caseInputReason = resolveCaseInput(challengeInputs)
	// Single-case runs should attribute final_output validators back to that case
	// so regression coverage can resolve pass/fail instead of staying pending.
	if evidence.challengeInputChallengeID != nil {
		evidence.finalOutputChallengeID = cloneUUIDPtr(evidence.challengeInputChallengeID)
	}

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
					evidence.finalOutputSource = eventRefFrom(event)
				} else if output, ok := extractLooseString(payload["output"]); ok {
					evidence.finalOutput = &output
					evidence.finalOutputSource = eventRefFrom(event)
				}
			}
		case "system.run.completed":
			occurredAt := event.OccurredAt.UTC()
			evidence.terminalAt = &occurredAt
			completed := true
			evidence.completedSuccessfully = &completed
			if output, ok := stringValue(payload, "final_output"); ok {
				evidence.finalOutput = &output
				// Deliberately NOT setting finalOutputSource here. This event
				// is a wrapper covering every preceding event; pointing a
				// source at it would make the "View in replay" link land on
				// a step that spans the entire run. resolveEvidenceSource
				// falls back to lastModelCallSource for the real producer.
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
		case "tool.call.completed", "tool.call.failed":
			if entry, ok := buildToolCallTraceEntry(payload, event.Type); ok {
				evidence.toolCallTrace = append(evidence.toolCallTrace, entry)
				if entry.Failed {
					evidence.failureCount++
				}
			} else if event.Type == "tool.call.failed" {
				evidence.failureCount++
			}
		case "sandbox.command.failed":
			evidence.failureCount++
		case "model.call.completed":
			if ref := eventRefFrom(event); ref != nil {
				evidence.lastModelCallSource = ref
			}
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
		case "race.standings.injected":
			// Aggregate race-context injection tokens separately from
			// model-authored tokens. See issue #400 slice 8.
			if value, ok := numericValue(payload, "tokens_added"); ok {
				evidence.raceContextTokens += value
			}
		case "grader.verification.file_captured":
			var capture FileCaptureResult
			if err := json.Unmarshal(event.Payload, &capture); err == nil && capture.Key != "" {
				if evidence.capturedFiles == nil {
					evidence.capturedFiles = make(map[string]FileCaptureResult)
				}
				evidence.capturedFiles[capture.Key] = capture
				if ref := eventRefFrom(event); ref != nil {
					if evidence.capturedFileSources == nil {
						evidence.capturedFileSources = make(map[string]eventRef)
					}
					evidence.capturedFileSources[capture.Key] = *ref
				}
			}
		case "grader.verification.directory_listed":
			var listing DirectoryListingResult
			if err := json.Unmarshal(event.Payload, &listing); err == nil && listing.Key != "" {
				if evidence.capturedDirListings == nil {
					evidence.capturedDirListings = make(map[string]DirectoryListingResult)
				}
				evidence.capturedDirListings[listing.Key] = listing
				if ref := eventRefFrom(event); ref != nil {
					if evidence.capturedDirListingSources == nil {
						evidence.capturedDirListingSources = make(map[string]eventRef)
					}
					evidence.capturedDirListingSources[listing.Key] = *ref
				}
			}
		case "grader.verification.code_executed":
			var result CodeExecutionResult
			if err := json.Unmarshal(event.Payload, &result); err == nil && result.ValidatorKey != "" {
				if evidence.codeExecutionResults == nil {
					evidence.codeExecutionResults = make(map[string]CodeExecutionResult)
				}
				evidence.codeExecutionResults[result.ValidatorKey] = result
				if ref := eventRefFrom(event); ref != nil {
					if evidence.codeExecutionSources == nil {
						evidence.codeExecutionSources = make(map[string]eventRef)
					}
					evidence.codeExecutionSources[result.ValidatorKey] = *ref
				}
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

func regressionCaseIDForChallenge(evidence extractedEvidence, challengeID *uuid.UUID) *uuid.UUID {
	if challengeID == nil {
		return nil
	}
	if evidence.caseInput != nil && evidence.caseInput.ChallengeIdentityID == *challengeID {
		return cloneUUIDPtr(evidence.caseInput.RegressionCaseID)
	}
	if evidence.challengeInputChallengeID != nil && *evidence.challengeInputChallengeID == *challengeID {
		return cloneUUIDPtr(evidence.challengeInputRegressionCaseID)
	}
	return nil
}
