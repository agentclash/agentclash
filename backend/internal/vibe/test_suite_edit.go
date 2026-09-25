package vibe

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/uuid"
)

// Expansion is a typed UI action, not just an instruction to the model. Existing
// cases and all grading metadata must remain byte-equivalent as JSON values.
func validateCoverageExpansion(p Plan, candidate *Artifact, policy PolicySnapshot) error {
	if p.Submission.AdditionalExamples == 0 {
		return nil
	}
	if p.Artifact == nil || candidate == nil || p.Conversation == nil || p.Conversation.Policy == nil {
		return fmt.Errorf("adding coverage needs existing tests and rules")
	}
	if candidate.AgentPrompt != p.Artifact.AgentPrompt || !sameJSON(raw(policy.Rules), raw(p.Conversation.Policy.Rules)) {
		return fmt.Errorf("keep the original instructions and all rules unchanged")
	}
	var before, after map[string]json.RawMessage
	if err := json.Unmarshal(p.Artifact.Blueprint, &before); err != nil {
		return err
	}
	if err := json.Unmarshal(candidate.Blueprint, &after); err != nil {
		return err
	}
	var oldCases, newCases []json.RawMessage
	if err := json.Unmarshal(before["cases"], &oldCases); err != nil {
		return err
	}
	if err := json.Unmarshal(after["cases"], &newCases); err != nil {
		return err
	}
	delete(before, "cases")
	delete(after, "cases")
	if !sameJSON(raw(before), raw(after)) || len(newCases) != len(oldCases)+p.Submission.AdditionalExamples {
		return fmt.Errorf("add exactly %d examples without changing grading or pack metadata", p.Submission.AdditionalExamples)
	}
	kept := map[string]json.RawMessage{}
	for _, c := range newCases {
		var key struct {
			Key string `json:"key"`
		}
		if json.Unmarshal(c, &key) != nil || key.Key == "" || kept[key.Key] != nil {
			return fmt.Errorf("each example needs a unique stable key")
		}
		kept[key.Key] = c
	}
	for _, c := range oldCases {
		var key struct {
			Key string `json:"key"`
		}
		_ = json.Unmarshal(c, &key)
		if !sameJSON(c, kept[key.Key]) {
			return fmt.Errorf("keep existing case %s and its expected answer unchanged", key.Key)
		}
	}
	return nil
}

// Explicit editor changes get the same message ownership as generated proposals.
func AppendTestRevision(s *Session, artifact Artifact, description string) {
	requestID, replyID := uuid.New(), uuid.New()
	artifact.SourceMessageID, artifact.ProposalMessageID, artifact.Dismissed = requestID, &replyID, false
	s.Document.Messages = append(s.Document.Messages,
		Message{ID: requestID, Role: "user", Content: description, CreatedAt: timestamp(), Origin: "edit"},
		Message{ID: replyID, Role: "assistant", Content: "Ready to test these changes.", CreatedAt: timestamp(), Origin: "edit", ArtifactID: &artifact.ID})
	s.Document.Artifacts = append(s.Document.Artifacts, artifact)
}

// PatchTestSuite edits explicit case fields in place, preserving unknown pack
// metadata, judges, validators, stable keys, and every untouched case.
func PatchTestSuite(blueprint json.RawMessage, changes []CaseChange, criteria *string, l Limits) (json.RawMessage, error) {
	return patchTestSuite(blueprint, changes, criteria, l, uuid.Nil)
}

func patchTestSuite(blueprint json.RawMessage, changes []CaseChange, criteria *string, l Limits, operation uuid.UUID) (json.RawMessage, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(blueprint, &root); err != nil {
		return nil, err
	}
	var cases []map[string]json.RawMessage
	if err := json.Unmarshal(root["cases"], &cases); err != nil || len(cases) == 0 {
		return nil, fmt.Errorf("these tests cannot be edited here; their original coverage is preserved")
	}
	var original any
	_ = json.Unmarshal(blueprint, &original)
	var judges []struct {
		ContextFrom []string `json:"context_from"`
	}
	_ = json.Unmarshal(root["judges"], &judges)
	usesExpected := false
	for _, judge := range judges {
		for _, reference := range judge.ContextFrom {
			if reference == ExpectedBehaviorReference {
				usesExpected = true
			}
		}
	}
	seen := map[string]bool{}
	for ordinal, change := range changes {
		if change.Expected != nil && !usesExpected {
			return nil, fmt.Errorf("this pack does not grade the editable expected-behavior field; edit its original grading rules to preserve the evaluation")
		}
		if seen[change.CaseKey] && change.Action != "add" {
			return nil, fmt.Errorf("change each case only once")
		}
		seen[change.CaseKey] = true
		index := -1
		for i, c := range cases {
			var key string
			_ = json.Unmarshal(c["key"], &key)
			if key == change.CaseKey {
				index = i
				break
			}
		}
		switch change.Action {
		case "add":
			if change.CaseKey != "" || change.Input == nil || change.Expected == nil {
				return nil, fmt.Errorf("new cases need input and expected behavior, with an empty case_key")
			}
			key := "case-" + uuid.NewString()[:8]
			if operation != uuid.Nil {
				key = "case-" + deterministicID(operation, fmt.Sprintf("case:%d", ordinal)).String()
			}
			cases = append(cases, map[string]json.RawMessage{"key": raw(key), "payload": raw(map[string]any{"question": *change.Input}), "expectations": raw([]map[string]any{{"key": ExpectedBehaviorKey, "kind": "text", "value": *change.Expected}})})
			index = len(cases) - 1
		case "update":
			if index < 0 || change.Input == nil && change.Expected == nil {
				return nil, fmt.Errorf("update needs an existing case and changed fields")
			}
		case "remove":
			if index < 0 || change.Input != nil || change.Expected != nil {
				return nil, fmt.Errorf("remove needs only an existing case_key")
			}
			cases = append(cases[:index], cases[index+1:]...)
			continue
		default:
			return nil, fmt.Errorf("case action must be add, update or remove")
		}
		if change.Input != nil {
			if strings.TrimSpace(*change.Input) == "" || len(*change.Input) > l.MessageBytes {
				return nil, fmt.Errorf("test input must be nonempty and fit the message size")
			}
			var payload map[string]json.RawMessage
			if err := json.Unmarshal(cases[index]["payload"], &payload); err != nil || payload["question"] == nil {
				return nil, fmt.Errorf("this case has a structured input; edit its original pack without removing fields")
			}
			payload["question"] = raw(*change.Input)
			cases[index]["payload"] = raw(payload)
		}
		if change.Expected != nil {
			if strings.TrimSpace(*change.Expected) == "" || len(*change.Expected) > l.MessageBytes {
				return nil, fmt.Errorf("expected behavior must be nonempty and fit the message size")
			}
			var expectations []map[string]json.RawMessage
			if err := json.Unmarshal(cases[index]["expectations"], &expectations); err != nil {
				return nil, fmt.Errorf("this case has no editable expected-behavior declaration")
			}
			found := false
			for i, e := range expectations {
				var key, kind string
				_ = json.Unmarshal(e["key"], &key)
				_ = json.Unmarshal(e["kind"], &kind)
				if key == ExpectedBehaviorKey && kind == "text" {
					expectations[i]["value"] = raw(*change.Expected)
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("this case uses additional grading rules; edit the original pack to preserve them")
			}
			cases[index]["expectations"] = raw(expectations)
		}
	}
	if len(cases) == 0 || len(cases) > l.Cases {
		return nil, fmt.Errorf("the updated suite must contain between 1 and %d tests", l.Cases)
	}
	root["cases"] = raw(cases)
	if criteria != nil {
		if strings.TrimSpace(*criteria) == "" || len(*criteria) > l.MessageBytes {
			return nil, fmt.Errorf("shared rule must be nonempty and bounded")
		}
		var judges []map[string]json.RawMessage
		if err := json.Unmarshal(root["judges"], &judges); err != nil || len(judges) != 1 {
			return nil, fmt.Errorf("this pack has additional grading rules; edit the original pack to preserve coverage")
		}
		var assertion string
		_ = json.Unmarshal(judges[0]["assertion"], &assertion)
		if !strings.HasPrefix(assertion, ScenarioCriteriaPrefix) {
			return nil, fmt.Errorf("this pack has a custom grading rule; edit it in the pack builder")
		}
		judges[0]["assertion"] = raw(ScenarioCriteriaPrefix + *criteria)
		root["judges"] = raw(judges)
	}
	out := raw(root)
	var updated any
	_ = json.Unmarshal(out, &updated)
	if reflect.DeepEqual(original, updated) {
		return nil, fmt.Errorf("the requested edit did not change the tests")
	}
	return out, nil
}
