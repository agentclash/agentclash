package scoring

import "strings"

// resolveEvidenceSource maps a validator `target` string (e.g. "final_output",
// "file:generated_code", "case.payload.answer") to the run event that produced
// the evidence, when one exists. It returns nil for evidence that is aggregated
// across events or sourced from outside the trace (challenge inputs, literals).
//
// The mapping intentionally mirrors resolveEvidenceValue so the source keeps
// pace with the value the validator actually evaluated.
func resolveEvidenceSource(target string, evidence extractedEvidence) *Source {
	switch {
	case target == "final_output", target == "run.final_output":
		// Prefer the dedicated finalized event when present (prompt_eval runs
		// emit it). Otherwise fall back to the last model.call.completed —
		// that's the actual producer of the agent's text. If neither exists
		// we return nil; a nil source beats a link that lands on the
		// system.run.completed wrapper covering the whole run.
		if evidence.finalOutputSource != nil {
			return &Source{
				Kind:      SourceKindFinalOutput,
				Sequence:  int64Ptr(evidence.finalOutputSource.Sequence),
				EventType: evidence.finalOutputSource.EventType,
				FieldPath: "final_output",
			}
		}
		if evidence.lastModelCallSource != nil {
			return &Source{
				Kind:      SourceKindModelCall,
				Sequence:  int64Ptr(evidence.lastModelCallSource.Sequence),
				EventType: evidence.lastModelCallSource.EventType,
				FieldPath: "final_output",
			}
		}
		return nil
	case strings.HasPrefix(target, "file:"):
		checkKey := strings.TrimPrefix(target, "file:")
		if ref, ok := evidence.capturedFileSources[checkKey]; ok {
			return &Source{
				Kind:      SourceKindToolCall,
				Sequence:  int64Ptr(ref.Sequence),
				EventType: ref.EventType,
				FieldPath: target,
			}
		}
		if ref, ok := evidence.capturedDirListingSources[checkKey]; ok {
			return &Source{
				Kind:      SourceKindToolCall,
				Sequence:  int64Ptr(ref.Sequence),
				EventType: ref.EventType,
				FieldPath: target,
			}
		}
		return nil
	default:
		// challenge_input, case.*, artifact.*, literal:* have no originating
		// run event — they come from challenge inputs, not the trace.
		return nil
	}
}
