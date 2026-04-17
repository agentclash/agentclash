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
		if evidence.finalOutputSource == nil {
			return nil
		}
		return &Source{
			Kind:      SourceKindFinalOutput,
			Sequence:  int64Ptr(evidence.finalOutputSource.Sequence),
			EventType: evidence.finalOutputSource.EventType,
			FieldPath: "final_output",
		}
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
