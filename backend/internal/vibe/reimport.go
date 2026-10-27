package vibe

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/google/uuid"
)

type ReimportRequest struct {
	Revision   int64     `json:"revision"`
	ArtifactID uuid.UUID `json:"artifact_id"`
	Content    string    `json:"content"`
	Apply      bool      `json:"apply,omitempty"`
	ReviewHash string    `json:"review_hash,omitempty"`
}
type SemanticChange struct {
	Path   string `json:"path"`
	Kind   string `json:"kind"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type ReimportReview struct {
	ReviewHash string           `json:"review_hash"`
	Changes    []SemanticChange `json:"changes"`
	Partial    bool             `json:"partial"`
	Applied    bool             `json:"applied"`
}

// Review binds the exact uploaded bytes and current project revision. Applying
// creates another immutable artifact; publication and CI baselines are separate.
func (s *Service) Reimport(ctx context.Context, actor string, id uuid.UUID, in ReimportRequest) (ReimportReview, error) {
	out := ReimportReview{Changes: []SemanticChange{}}
	v, err := s.Store.GetSession(ctx, actor, id)
	if err != nil {
		return out, err
	}
	if v.Revision != in.Revision {
		return out, fault("revision_conflict", "Reload this project before reviewing the import.")
	}
	var base *Artifact
	for i := range v.Document.Artifacts {
		if v.Document.Artifacts[i].ID == in.ArtifactID {
			base = &v.Document.Artifacts[i]
		}
	}
	if base == nil {
		return out, fault("artifact_required", "Choose the exact version to compare.")
	}
	b, prompt, _, err := importDefinition([]byte(in.Content), s.Config.Limits(v.Anonymous))
	if err != nil {
		return out, err
	}
	var envelope struct {
		ArtifactID uuid.UUID  `json:"artifact_id"`
		Artifacts  []Artifact `json:"artifacts"`
		Manifest   struct {
			Completeness string `json:"completeness"`
		} `json:"manifest"`
	}
	normalized, err := ImportJSON([]byte(in.Content), s.Config.Limits(v.Anonymous))
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(normalized, &envelope); err != nil {
		return out, err
	}
	out.Partial = envelope.Manifest.Completeness == "partial"
	if envelope.ArtifactID != uuid.Nil {
		matches := 0
		for _, a := range envelope.Artifacts {
			if a.ID == envelope.ArtifactID {
				matches++
			}
		}
		if matches != 1 {
			return out, fault("invalid_import", "The export must identify exactly one source version.")
		}
		if envelope.ArtifactID != base.ID {
			original, e := s.Store.GetOriginal(ctx, actor, id, base.ID)
			var previous struct {
				ArtifactID uuid.UUID `json:"artifact_id"`
			}
			if e != nil || json.Unmarshal(original.Content, &previous) != nil || previous.ArtifactID != envelope.ArtifactID {
				return out, fault("invalid_import", "This export belongs to another version. Import it as a separate test instead.")
			}
		}
	}
	before, err := semanticDefinition(base.Blueprint, base.AgentPrompt)
	if err != nil {
		return out, err
	}
	after, err := semanticDefinition(b, prompt)
	if err != nil {
		return out, err
	}
	semanticDiff("", before, after, &out.Changes)
	for _, change := range out.Changes {
		if out.Partial && change.Kind == "removed" && strings.Contains(change.Path, "/cases/") && !strings.Contains(strings.SplitN(change.Path, "/cases/", 2)[1], "/") {
			return out, fault("invalid_import", "A partial export cannot remove omitted cases. Import it separately or provide the complete definition.")
		}
	}
	out.ReviewHash = Hash(raw(struct {
		Session  uuid.UUID
		Artifact uuid.UUID
		Revision int64
		Content  string
	}{id, in.ArtifactID, in.Revision, Hash([]byte(in.Content))}))
	if in.Apply {
		if in.ReviewHash != out.ReviewHash {
			return out, fault("revision_conflict", "Review these exact changes before creating a new version.")
		}
		if err = s.importVersion(ctx, actor, id, in.Revision, []byte(in.Content), &in.ArtifactID); err != nil {
			return out, err
		}
		out.Applied = true
	}
	return out, nil
}

func semanticDefinition(b []byte, prompt string) (any, error) {
	var value any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&value); err != nil {
		return nil, err
	}
	normalized, err := semanticValue(value)
	return map[string]any{"evaluation": normalized, "agent_prompt": prompt}, err
}

// Authored keys identify cases, validators and inputs. Nested input-set and
// challenge keys retain the existing publication tuple; positions do not.
func semanticValue(value any) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, child := range v {
			n, err := semanticValue(child)
			if err != nil {
				return nil, err
			}
			out[k] = n
		}
		return out, nil
	case []any:
		keyed := map[string]any{}
		allKeyed := len(v) > 0
		out := make([]any, len(v))
		for i, child := range v {
			n, err := semanticValue(child)
			if err != nil {
				return nil, err
			}
			out[i] = n
			m, ok := child.(map[string]any)
			if !ok {
				allKeyed = false
				continue
			}
			key := ""
			for _, field := range []string{"case_key", "item_key", "key", "id"} {
				if s, ok := m[field].(string); ok && s != "" {
					key = s
					break
				}
			}
			if c, ok := m["challenge_key"].(string); ok {
				key = c + ":" + key
			}
			if key == "" {
				allKeyed = false
				continue
			}
			if _, exists := keyed[key]; exists {
				return nil, fault("invalid_import", "Duplicate authored identity in the evaluation.")
			}
			keyed[key] = n
		}
		if allKeyed {
			return keyed, nil
		}
		return out, nil
	default:
		return v, nil
	}
}
func semanticDiff(path string, before, after any, out *[]SemanticChange) {
	if reflect.DeepEqual(before, after) {
		return
	}
	a, aok := before.(map[string]any)
	b, bok := after.(map[string]any)
	if aok && bok {
		keys := map[string]bool{}
		for k := range a {
			keys[k] = true
		}
		for k := range b {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			p := path + "/" + strings.ReplaceAll(strings.ReplaceAll(k, "~", "~0"), "/", "~1")
			av, ae := a[k]
			bv, be := b[k]
			if !ae {
				*out = append(*out, SemanticChange{p, "added", nil, bv})
			} else if !be {
				*out = append(*out, SemanticChange{p, "removed", av, nil})
			} else {
				semanticDiff(p, av, bv, out)
			}
		}
		return
	}
	*out = append(*out, SemanticChange{path, "changed", before, after})
}
