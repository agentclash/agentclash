package vibe

import (
	"bytes"
	"testing"

	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
)

func TestVibeTargetInputRejectsUnnormalizedReferences(t *testing.T) {
	for _, ref := range []string{
		"case.payload.item.expected ", "\tcase.payload.item.expected \n",
		"case.payload ", "\tchallenge_input\n",
	} {
		t.Run(ref, func(t *testing.T) {
			c := challengepack.CaseDefinition{Payload: map[string]any{
				"prompt": "Answer", "item": map[string]any{"expected": "SECRET", "expected ": "decoy"},
			}}
			spec := scoring.EvaluationSpec{Validators: []scoring.ValidatorDeclaration{{ExpectedFrom: ref}}}
			beforeCase, beforeSpec := raw(c), raw(spec)
			// The executable compiler/runner pass the shared normalized view.
			// A caller accidentally passing the persisted raw view must fail
			// closed, never choose a different key from the shared scorer.
			if got := TargetInput(c, spec); got != nil {
				t.Errorf("unnormalized reference %q returned unsafe target input: %s", ref, raw(got))
			}
			if !bytes.Equal(beforeCase, raw(c)) || !bytes.Equal(beforeSpec, raw(spec)) {
				t.Fatal("withholding changed the original case or declarations")
			}
		})
	}
}
