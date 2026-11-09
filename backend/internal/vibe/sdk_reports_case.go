package vibe

import (
	"encoding/json"
	"fmt"
)

// This is an explicit review projection, not observed-tool normalization or a
// replacement SDK grade. Original metrics (including scores) stay in source_json.
type sdkSourceCase struct {
	Case struct {
		ID       string `json:"case_id"`
		Name     string `json:"name"`
		Input    string `json:"input"`
		Expected string `json:"expected"`
	} `json:"case"`
	Status      string          `json:"status"`
	AgentResult *sdkAgentResult `json:"agent_result"`
	Metrics     []struct {
		Key    string `json:"key"`
		Name   string `json:"name"`
		Passed bool   `json:"passed"`
		Reason string `json:"reason"`
		Error  string `json:"error"`
	} `json:"metrics"`
}
type sdkAgentResult struct {
	Input    string `json:"input"`
	Output   string `json:"output"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
}

func sdkSourceCases(r SDKReport) ([]sdkSourceCase, error) {
	if r.SourceKind == "agent_result" {
		var agent sdkAgentResult
		if err := json.Unmarshal([]byte(r.SourceJSON), &agent); err != nil {
			return nil, err
		}
		c := sdkSourceCase{AgentResult: &agent}
		c.Case.ID = r.Cases[0].SourceCaseID
		c.Case.Input = agent.Input
		return []sdkSourceCase{c}, nil
	}
	var doc struct {
		Cases []sdkSourceCase `json:"cases"`
	}
	err := json.Unmarshal([]byte(r.SourceJSON), &doc)
	return doc.Cases, err
}
func SDKReportCase(r SDKReport, sourceID, caseKey string) (CaseResult, error) {
	var selected *SDKCaseBinding
	for i, c := range r.Cases {
		if (sourceID == "" || c.SourceCaseID == sourceID) && (caseKey == "" || c.CaseKey == caseKey) {
			if selected != nil {
				return CaseResult{}, fault("invalid_request", "Select the exact source case for this repeated case key.")
			}
			selected = &r.Cases[i]
		}
	}
	if selected == nil {
		return CaseResult{}, fault("not_found", "Source case is unavailable.")
	}
	cases, err := sdkSourceCases(r)
	if err != nil {
		return CaseResult{}, err
	}
	for _, source := range cases {
		if source.Case.ID != selected.SourceCaseID {
			continue
		}
		c := CaseResult{CaseKey: selected.CaseKey, Version: r.Binding.ArtifactID.String(), Title: source.Case.Name, Input: raw(source.Case.Input), Expected: source.Case.Expected, Verdict: Unknown, Checks: []CheckResult{}}
		switch source.Status {
		case "passed":
			c.Verdict = Pass
		case "failed":
			c.Verdict = Fail
		}
		if source.AgentResult != nil {
			c.Input = raw(source.AgentResult.Input)
			c.Output = source.AgentResult.Output
			for i, m := range source.AgentResult.Messages {
				c.Messages = append(c.Messages, EvidenceMessage{ID: fmt.Sprintf("sdk-message-%d", i), Role: m.Role, Content: m.Content})
			}
		}
		for _, m := range source.Metrics {
			v := Fail
			if m.Passed {
				v = Pass
			}
			check := CheckResult{Key: m.Key, Verdict: v, Evidence: m.Reason}
			if m.Error != "" {
				check.Verdict = Unknown
				check.Error = &Fault{Code: "imported_error", Message: m.Error}
			}
			c.Checks = append(c.Checks, check)
		}
		c.ExpectedChecks = len(c.Checks)
		return c, nil
	}
	return CaseResult{}, fault("not_found", "Original source case is unavailable.")
}
