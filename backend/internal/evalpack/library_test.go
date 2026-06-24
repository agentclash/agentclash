package evalpack

import (
	"encoding/json"
	"testing"

	"github.com/agentclash/agentclash/backend/internal/scoring"
)

// TestStarterPieceLibraryDefinitionsAreWellFormed guards the in-code piece
// library the builder clones from: every piece must have the required
// identifiers and a definition that decodes into the struct its kind implies.
func TestStarterPieceLibraryDefinitionsAreWellFormed(t *testing.T) {
	pieces := StarterPieceLibrary()
	if len(pieces) == 0 {
		t.Fatal("starter piece library is empty")
	}

	slugs := map[string]struct{}{}
	for _, p := range pieces {
		t.Run(p.Kind+"/"+p.Slug, func(t *testing.T) {
			if p.Slug == "" || p.Name == "" || p.Description == "" {
				t.Error("piece missing slug/name/description")
			}
			if _, dup := slugs[p.Kind+"/"+p.Slug]; dup {
				t.Errorf("duplicate piece %s/%s", p.Kind, p.Slug)
			}
			slugs[p.Kind+"/"+p.Slug] = struct{}{}

			switch p.Kind {
			case ProvenanceKindValidator:
				var v scoring.ValidatorDeclaration
				if err := json.Unmarshal(p.Definition, &v); err != nil {
					t.Fatalf("validator definition: %v", err)
				}
				if v.Key == "" || !v.Type.IsValid() {
					t.Errorf("validator piece has empty key or invalid type: %+v", v)
				}
			case ProvenanceKindJudge:
				var j scoring.LLMJudgeDeclaration
				if err := json.Unmarshal(p.Definition, &j); err != nil {
					t.Fatalf("judge definition: %v", err)
				}
				if j.Key == "" || !j.Mode.IsValid() {
					t.Errorf("judge piece has empty key or invalid mode: %+v", j)
				}
				if j.Model == "" && len(j.Models) == 0 {
					t.Errorf("judge piece %q sets no model", j.Key)
				}
			case ProvenanceKindChallenge:
				var c ChallengeDefinition
				if err := json.Unmarshal(p.Definition, &c); err != nil {
					t.Fatalf("challenge definition: %v", err)
				}
				if c.Key == "" || c.Title == "" {
					t.Errorf("challenge piece missing key/title: %+v", c)
				}
			case ProvenanceKindInputSet:
				var s InputSetDefinition
				if err := json.Unmarshal(p.Definition, &s); err != nil {
					t.Fatalf("input set definition: %v", err)
				}
				if s.Key == "" || s.Name == "" {
					t.Errorf("input set piece missing key/name: %+v", s)
				}
			default:
				t.Errorf("unknown piece kind %q", p.Kind)
			}
		})
	}
}
