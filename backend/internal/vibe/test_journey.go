package vibe

import ()

type testSuiteProposal struct {
	Title           string         `json:"title"`
	Summary         string         `json:"summary"`
	Scenarios       []TestScenario `json:"scenarios"`
	SuccessCriteria string         `json:"success_criteria"`
}

func (a Artifact) IsTestSuite() bool { return a.Kind == "test_suite" }
