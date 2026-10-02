package vibe

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/agentclash/agentclash/backend/internal/vibe/inputs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// A portable reference describes a dependency, never access to a source file.
type ReferenceRequirement struct {
	Key    string `json:"key"`
	Hash   string `json:"content_hash"`
	Usage  string `json:"usage"`
	Name   string `json:"name,omitempty"`
	Format string `json:"format,omitempty"`
	Pages  int    `json:"page_count,omitempty"`
}

type portableDefinition struct {
	Format               string                 `json:"format"`
	Title                string                 `json:"title,omitempty"`
	AgentPrompt          string                 `json:"agent_prompt"`
	Evaluation           json.RawMessage        `json:"evaluation"`
	InputContract        *InputContract         `json:"input_contract,omitempty"`
	RequiredCapabilities []string               `json:"required_capabilities,omitempty"`
	ScopeNote            string                 `json:"scope_note,omitempty"`
	Sample               string                 `json:"sample,omitempty"`
	References           []ReferenceRequirement `json:"references,omitempty"`
	Models               Models                 `json:"models"`
}

// Legacy evidence archives restore the selected definition, never its runs,
// policy grants or private file IDs. Plain packs retain their original bytes.
func importDefinition(content []byte, l Limits) (Artifact, error) {
	b, err := ImportJSON(content, l)
	if err != nil {
		return Artifact{}, err
	}
	var envelope struct {
		Format string `json:"format"`
	}
	if err = json.Unmarshal(b, &envelope); err != nil {
		return Artifact{}, err
	}
	a := Artifact{Blueprint: b, ScopeNote: prototypeScope}
	switch envelope.Format {
	case "":
		return a, nil
	case "agentclash-vibe-v1", "agentclash-vibe-v2":
		var d portableDefinition
		if err = Decode(b, l, &d); err != nil {
			return a, err
		}
		if d.Format == "agentclash-vibe-v1" && strings.TrimSpace(d.AgentPrompt) == "" {
			return a, fault("invalid_pack", "The agent instructions are missing.")
		}
		if len(d.Models.Assistant) > 256 || len(d.Models.Target) > 256 || len(d.Models.Evaluator) > 256 {
			return a, fault("invalid_pack", "A model preference is too long.")
		}
		a.Title, a.AgentPrompt, a.Blueprint = d.Title, d.AgentPrompt, d.Evaluation
		a.InputContract, a.RequiredCapabilities = d.InputContract, d.RequiredCapabilities
		a.Sample, a.MissingReferences = d.Sample, d.References
		if d.ScopeNote != "" {
			a.ScopeNote = d.ScopeNote
		}
	case "agentclash-evaluation-v1":
		var d struct {
			Format     string          `json:"format"`
			ArtifactID uuid.UUID       `json:"artifact_id"`
			Artifacts  []Artifact      `json:"artifacts"`
			Scope      string          `json:"scope"`
			Sample     *string         `json:"sample"`
			Rules      json.RawMessage `json:"rules"`
			Runs       json.RawMessage `json:"runs"`
		}
		if err = Decode(b, l, &d); err != nil {
			return a, err
		}
		found := false
		for _, selected := range d.Artifacts {
			if selected.ID != d.ArtifactID {
				continue
			}
			inheritExecutionContext(&a, &selected)
			a.Title, a.Blueprint, a.Sample = selected.Title, selected.Blueprint, selected.Sample
			keys := map[string]bool{}
			for _, reference := range a.MissingReferences {
				keys[reference.Key] = true
			}
			for _, binding := range selected.ReferenceInputs {
				n := 1
				for keys["reference-"+strconv.Itoa(n)] {
					n++
				}
				key := "reference-" + strconv.Itoa(n)
				keys[key] = true
				a.MissingReferences = append(a.MissingReferences, ReferenceRequirement{Key: key, Hash: binding.Hash, Usage: "reference"})
			}
			a.ReferenceInputs = nil
			found = true
			break
		}
		if !found {
			return a, fault("invalid_pack", "The export does not identify an agent version.")
		}
	default:
		return a, fault("invalid_pack", "This download is a historical archive, not an agent definition or test pack.")
	}
	if len(a.Title) > 200 || len(a.AgentPrompt) > l.MessageBytes || len(a.ScopeNote) > l.MessageBytes || len(a.Sample) > 200 || len(a.RequiredCapabilities) > 16 || len(a.MissingReferences) > 4 {
		return a, fault("invalid_pack", "The agent definition exceeds its size limits.")
	}
	for _, capability := range a.RequiredCapabilities {
		if len(capability) > 128 {
			return a, fault("invalid_pack", "A capability name is too long.")
		}
	}
	if a.InputContract != nil {
		if a.InputContract.Version != 1 || len(a.InputContract.Formats) > 4 || len(a.InputContract.Label) > 200 {
			return a, fault("invalid_pack", "The input contract is invalid.")
		}
		for _, format := range a.InputContract.Formats {
			if len(format) > 128 {
				return a, fault("invalid_pack", "The input format is invalid.")
			}
		}
	}
	seen := map[string]bool{}
	for _, reference := range a.MissingReferences {
		hash, e := hex.DecodeString(reference.Hash)
		if reference.Key == "" || len(reference.Key) > 128 || seen[reference.Key] || e != nil || len(hash) != 32 || reference.Usage != "reference" || len(reference.Name) > 200 || reference.Pages < 0 || reference.Pages > inputs.MaxPages || reference.Format != "" && reference.Format != "text" && reference.Format != "pdf" {
			return a, fault("invalid_pack", "A reference requirement is invalid.")
		}
		seen[reference.Key] = true
	}
	if a.ScopeNote == "" {
		a.ScopeNote = prototypeScope
	}
	for _, capability := range a.RequiredCapabilities {
		if capability != "text_generation" && capability != "pdf_text_extraction" {
			a.UnavailableReason = "This definition needs " + capability + ", which cannot run here. Its instructions and tests are preserved for export."
			break
		}
	}
	if a.InputContract != nil {
		for _, format := range a.InputContract.Formats {
			if format != "text" && format != "pdf_text" {
				a.UnavailableReason = "This definition needs " + format + " input, which cannot be processed here. Its instructions and tests are preserved."
				break
			}
		}
	}
	return a, nil
}

// Binding holds the same session lock used by file deletion and project edits.
// Revisions are immutable: a failed or stale attachment publishes no version.
func (s *Service) BindReference(ctx context.Context, actor string, session uuid.UUID, revision int64, artifact uuid.UUID, key string, binding inputs.Binding) error {
	return s.Store.edit(ctx, actor, session, revision, func(ctx context.Context, tx pgx.Tx, v *Session) error {
		var original *Artifact
		for i := range v.Document.Artifacts {
			if v.Document.Artifacts[i].ID == artifact {
				original = &v.Document.Artifacts[i]
				break
			}
		}
		if original == nil {
			return fault("artifact_required", "Choose the imported agent version.")
		}
		index := -1
		for i, reference := range original.MissingReferences {
			if reference.Key == key {
				index = i
				break
			}
		}
		if index < 0 {
			return fault("invalid_input", "This reference is already attached or unavailable.")
		}
		required := original.MissingReferences[index]
		if binding.Usage != "reference" || binding.Hash != required.Hash {
			return fault("input_unavailable", "This is a different document. Attach the original reference; changing the rules requires a separate version and check decision.")
		}
		record, err := inputs.Resolve(ctx, tx, session, binding)
		if err != nil || required.Format != "" && required.Format != record.Kind || required.Pages > 0 && required.Pages != len(record.Pages) {
			return fault("input_unavailable", "This material is unavailable, changed or needs acknowledgement of unread pages.")
		}

		next := *original
		inheritExecutionContext(&next, original)
		next.ID, next.ParentID, next.Accepted, next.CreatedAt = uuid.New(), &original.ID, false, timestamp()
		next.MissingReferences = append(next.MissingReferences[:index:index], next.MissingReferences[index+1:]...)
		alreadyBound := false
		for _, existing := range next.ReferenceInputs {
			if existing.ID == binding.ID {
				alreadyBound = true
				break
			}
		}
		if !alreadyBound {
			next.ReferenceInputs = append(next.ReferenceInputs, binding)
		}
		v.Document.Artifacts = append(v.Document.Artifacts, next)
		v.Document.ActiveArtifactID = &next.ID
		return nil
	})
}
