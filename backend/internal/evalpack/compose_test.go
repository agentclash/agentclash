package evalpack

import (
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
	"github.com/google/uuid"
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func f64(v float64) *float64 { return &v }

// TestComposeBundleInlinePiecesValidatesAsNativePack composes a minimal native
// pack entirely from inline pieces and asserts the assembled Bundle passes the
// existing ValidateBundle path — i.e. the compose step round-trips into a real,
// runnable pack.
func TestComposeBundleInlinePiecesValidatesAsNativePack(t *testing.T) {
	challenge := ChallengeDefinition{Key: "task", Title: "Task", Category: "general", Difficulty: "easy", Instructions: "Do the thing."}
	inputSet := InputSetDefinition{Key: "default", Name: "Default", Cases: []CaseDefinition{{ChallengeKey: "task", CaseKey: "c1", Payload: map[string]any{}}}}
	validator := scoring.ValidatorDeclaration{Key: "has_output", Type: scoring.ValidatorTypeRegexMatch, Target: "final_output", ExpectedFrom: "literal:.+"}

	comp := Composition{
		Pack:       PackMetadata{Slug: "demo-pack", Name: "Demo Pack", Family: "general"},
		Version:    CompositionVersion{Number: 1, ExecutionMode: ExecutionModeNative},
		Challenges: []PieceRef{{Inline: mustJSON(t, challenge)}},
		InputSets:  []PieceRef{{Inline: mustJSON(t, inputSet)}},
		Validators: []PieceRef{{Inline: mustJSON(t, validator)}},
		Scorecard: CompositionScorecard{
			Name:      "demo-eval",
			JudgeMode: scoring.JudgeModeDeterministic,
			Strategy:  scoring.ScoringStrategyBinary,
			Dimensions: []scoring.DimensionDeclaration{{
				Key:           "correctness",
				Source:        scoring.DimensionSourceValidators,
				Validators:    []string{"has_output"},
				PassThreshold: f64(1.0),
			}},
		},
	}

	bundle, err := ComposeBundle(comp, nil)
	if err != nil {
		t.Fatalf("ComposeBundle: %v", err)
	}
	if got := len(bundle.Challenges); got != 1 {
		t.Fatalf("challenges = %d, want 1", got)
	}
	if got := len(bundle.Version.EvaluationSpec.Validators); got != 1 {
		t.Fatalf("validators = %d, want 1", got)
	}
	if bundle.Version.EvaluationSpec.Validators[0].Key != "has_output" {
		t.Fatalf("validator key = %q, want has_output", bundle.Version.EvaluationSpec.Validators[0].Key)
	}
	if err := ValidateBundle(bundle); err != nil {
		t.Fatalf("ValidateBundle on composed pack: %v", err)
	}

	card := SpecCardSummary(bundle)
	if card.ChallengeCount != 1 || card.CaseCount != 1 || card.ValidatorCount != 1 {
		t.Fatalf("spec card counts = %+v", card)
	}
	if len(card.Dimensions) != 1 || card.Dimensions[0].Key != "correctness" {
		t.Fatalf("spec card dimensions = %+v", card.Dimensions)
	}
}

// TestComposeBundleResolvesReferencedPiece confirms a library-referenced piece
// (by id) is resolved from the supplied ResolvedPieces map.
func TestComposeBundleResolvesReferencedPiece(t *testing.T) {
	pieceID := uuid.New()
	validator := scoring.ValidatorDeclaration{Key: "v1", Type: scoring.ValidatorTypeContains, Target: "final_output", ExpectedFrom: "literal:hello"}

	comp := Composition{
		Pack:       PackMetadata{Slug: "p", Name: "P", Family: "general"},
		Version:    CompositionVersion{Number: 1, ExecutionMode: ExecutionModeNative},
		Validators: []PieceRef{{RefID: &pieceID}},
	}

	ids := comp.ReferencedPieceIDs()
	if len(ids) != 1 || ids[0] != pieceID {
		t.Fatalf("ReferencedPieceIDs = %v, want [%s]", ids, pieceID)
	}

	bundle, err := ComposeBundle(comp, ResolvedPieces{pieceID: mustJSON(t, validator)})
	if err != nil {
		t.Fatalf("ComposeBundle: %v", err)
	}
	if got := len(bundle.Version.EvaluationSpec.Validators); got != 1 || bundle.Version.EvaluationSpec.Validators[0].Key != "v1" {
		t.Fatalf("validators = %+v", bundle.Version.EvaluationSpec.Validators)
	}
}

// TestComposeBundleMissingReferenceErrors confirms an unresolved reference is a
// hard error (so compile surfaces it instead of silently dropping the piece).
func TestComposeBundleMissingReferenceErrors(t *testing.T) {
	pieceID := uuid.New()
	comp := Composition{Validators: []PieceRef{{RefID: &pieceID}}}
	if _, err := ComposeBundle(comp, ResolvedPieces{}); err == nil {
		t.Fatal("expected error for unresolved piece reference, got nil")
	}
}
