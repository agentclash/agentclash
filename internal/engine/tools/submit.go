package tools

// SubmitSolution is the meta-tool agents call to declare they're done.
type SubmitSolution struct{}

func (t *SubmitSolution) Name() string        { return "submit_solution" }
func (t *SubmitSolution) Category() Category  { return CatMeta }
func (t *SubmitSolution) Description() string {
	return "Declare that you have solved the challenge. Call this when your fix is complete and tests pass. This ends your run."
}
func (t *SubmitSolution) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"explanation": map[string]any{
				"type":        "string",
				"description": "Brief explanation of what you fixed and why",
			},
		},
		"required": []string{"explanation"},
	}
}

func (t *SubmitSolution) Execute(_ string, args map[string]any) (string, error) {
	explanation, _ := args["explanation"].(string)
	return "SOLUTION_SUBMITTED: " + explanation, nil
}
