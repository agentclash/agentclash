package cmd

// ExitCode documents a process exit code the CLI can return. It exists so the
// machine-readable `agentclash schema` output can tell agents/CI what each exit
// code means without scraping help text.
//
// The authoritative int values live next to the commands that use them
// (compare.go gateExit*, ci_run.go ciRunExit*, prompt_eval.go promptEvalExit*).
// This registry references those same consts, and schema_test.go asserts the two
// stay in lockstep — so the documented contract can never silently drift.
//
// Codes are intentionally overloaded across commands (e.g. 3 means "insufficient
// evidence" for `compare gate` but "gate failed" for `prompt-eval`), so entries
// are scoped by Commands rather than keyed uniquely by Code.
type ExitCode struct {
	Code        int      `json:"code" yaml:"code"`
	Name        string   `json:"name" yaml:"name"`
	Description string   `json:"description" yaml:"description"`
	Commands    []string `json:"commands,omitempty" yaml:"commands,omitempty"`
}

var documentedExitCodes = []ExitCode{
	{Code: 0, Name: "success", Description: "Command completed successfully."},
	{Code: 1, Name: "error", Description: "Generic, unexpected failure when no command-specific code applies."},
	{Code: 2, Name: "missing_workspace", Description: "No workspace resolved; pass --workspace, set AGENTCLASH_WORKSPACE, or run 'agentclash link'."},

	// `compare gate` — CI regression gate.
	{Code: gateExitFail, Name: "compare_gate_fail", Description: "compare gate: a regression was detected.", Commands: []string{"compare gate"}},
	{Code: gateExitWarn, Name: "compare_gate_warn", Description: "compare gate: a warning-level regression was detected.", Commands: []string{"compare gate"}},
	{Code: gateExitInsufficientEvidence, Name: "compare_gate_insufficient_evidence", Description: "compare gate: not enough evidence to decide.", Commands: []string{"compare gate"}},

	// `prompt-eval`.
	{Code: promptEvalExitGate, Name: "prompt_eval_gate_failed", Description: "prompt-eval: the evaluation gate failed.", Commands: []string{"prompt-eval"}},
	{Code: promptEvalExitExecution, Name: "prompt_eval_execution_error", Description: "prompt-eval: an error occurred while executing the evaluation.", Commands: []string{"prompt-eval"}},
	{Code: promptEvalExitInvalid, Name: "prompt_eval_invalid_input", Description: "prompt-eval: invalid manifest or arguments.", Commands: []string{"prompt-eval"}},

	// `ci run`.
	{Code: ciRunExitInvalidManifest, Name: "ci_run_invalid_manifest", Description: "ci run: the CI manifest is invalid.", Commands: []string{"ci run"}},
	{Code: ciRunExitAPI, Name: "ci_run_api_error", Description: "ci run: an API error occurred.", Commands: []string{"ci run"}},
	{Code: ciRunExitTimeout, Name: "ci_run_timeout", Description: "ci run: timed out waiting for a candidate run.", Commands: []string{"ci run"}},
	{Code: ciRunExitRunFailed, Name: "ci_run_run_failed", Description: "ci run: a candidate run finished in a failed state.", Commands: []string{"ci run"}},
}
