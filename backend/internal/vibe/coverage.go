package vibe

// RuleCoverage records review links, not a completeness or quality score. An
// unlinked rule is a suggestion to discuss, never permission to add a test.
type RuleCoverage struct {
	RuleID    string   `json:"rule_id"`
	Statement string   `json:"statement"`
	CaseKeys  []string `json:"case_keys"`
}

func ruleCoverage(d Document, a Artifact) []RuleCoverage {
	if !a.IsTestSuite() || !a.Accepted || !validArtifactPolicy(d, a) {
		return nil
	}
	policy := policyFor(d, &a)
	if _, err := verifiedPolicySources(d, *policy); err != nil {
		return nil
	}
	for _, change := range d.PendingPolicyChanges {
		if change.Status == "pending" && (change.ArtifactID == nil || *change.ArtifactID == a.ID) {
			return nil
		}
	}
	rows := make([]RuleCoverage, 0, len(policy.Rules))
	for _, rule := range policy.Rules {
		row := RuleCoverage{RuleID: rule.ID, Statement: rule.Statement, CaseKeys: []string{}}
		for _, example := range a.Validation.Cases {
			if example.Status != SuiteSupported {
				continue
			}
			for _, id := range example.RuleIDs {
				if id == rule.ID {
					row.CaseKeys = append(row.CaseKeys, example.CaseKey)
					break
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}
