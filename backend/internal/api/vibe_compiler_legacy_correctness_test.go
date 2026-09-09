package api

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/vibe"
	"github.com/agentclash/agentclash/runtime/challengepack"
	"github.com/agentclash/agentclash/runtime/scoring"
	"github.com/google/uuid"
)

func TestVibeCompilerLegacyCorrectnessCoverage(t *testing.T) {
	for _, tt := range []struct {
		name, dimension string
	}{
		{"string", `"correctness"`},
		{"empty_source", `{"key":"correctness","source":"","gate":true,"pass_threshold":1}`},
		{"trimmed_key", `{"key":" correctness ","gate":true,"pass_threshold":1}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			b := vibeReliabilityBundle(t)
			var dimension scoring.DimensionDeclaration
			if err := json.Unmarshal([]byte(tt.dimension), &dimension); err != nil {
				t.Fatal(err)
			}
			b.Version.EvaluationSpec.Scorecard.Dimensions = []scoring.DimensionDeclaration{dimension}
			// Prove that the actual shared scorer expands this legacy form to
			// validator coverage, rather than assuming every empty source is safe.
			fixtureComposition, err := challengepack.BundleToComposition(b)
			if err != nil {
				t.Fatal(err)
			}
			fixture, err := challengepack.ComposeBundle(fixtureComposition, challengepack.ResolvedPieces{})
			if err != nil {
				t.Fatal(err)
			}
			want, err := scoring.DecodeDefinition(vibeReliabilityJSON(t, fixture.Version.EvaluationSpec))
			if err != nil || len(want.Scorecard.Dimensions) != 1 || want.Scorecard.Dimensions[0].Source != scoring.DimensionSourceValidators {
				t.Fatalf("invalid legacy validator fixture: %+v %v", want.Scorecard, err)
			}
			// Exercise the original wire representation, not just a string
			// decoded and then re-encoded as an object-form declaration.
			var wire map[string]any
			if err := json.Unmarshal(vibeReliabilityJSON(t, b), &wire); err != nil {
				t.Fatal(err)
			}
			wire["version"].(map[string]any)["evaluation_spec"].(map[string]any)["scorecard"].(map[string]any)["dimensions"] = []json.RawMessage{json.RawMessage(tt.dimension)}
			compiled, err := (VibePackCompiler{}).Compile(vibeReliabilityJSON(t, wire), "openai/gpt-4.1-mini", uuid.New(), vibe.LimitsFor(true))
			if err != nil {
				t.Fatalf("supported legacy correctness rejected: %v", err)
			}
			if !reflect.DeepEqual(compiled.Cases, b.InputSets[0].Cases) {
				t.Fatal("legacy compile changed case coverage")
			}
			var composition challengepack.Composition
			if err := json.Unmarshal(compiled.Composition, &composition); err != nil {
				t.Fatal(err)
			}
			rebuilt, err := challengepack.ComposeBundle(composition, challengepack.ResolvedPieces{})
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range []scoring.EvaluationSpec{compiled.Bundle.Version.EvaluationSpec, rebuilt.Version.EvaluationSpec} {
				got, err := scoring.DecodeDefinition(vibeReliabilityJSON(t, spec))
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("executed/saved legacy validator or gate coverage changed: got %+v want %+v err %v", got, want, err)
				}
			}
		})
	}
}
