package scoring

func humanPreferenceDimensionScore(dim DimensionDeclaration, evidence extractedEvidence) (*float64, string, OutputState) {
	if evidence.humanPreferenceScore == nil {
		return nil, "human preference evidence is unavailable", OutputStateUnavailable
	}
	score := *evidence.humanPreferenceScore
	return &score, "", OutputStateAvailable
}
