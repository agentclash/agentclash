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

// Global failure-class band, sysexits(3)-aligned so the values cannot collide
// with the small command-scoped codes below (1–31). exitCodeForError maps every
// non-command-specific failure into this band; agents and CI can branch on the
// class without parsing output.
const (
	exitValidationError  = 64 // EX_USAGE — usage / validation errors
	exitNotFound         = 66 // EX_NOINPUT — referenced resource or file does not exist
	exitRetryableFailure = 75 // EX_TEMPFAIL — transient; mirrors envelope retryable:true
	exitAuthDenied       = 77 // EX_NOPERM — authentication or permission failure
)

var documentedExitCodes = []ExitCode{
	{Code: 0, Name: "success", Description: "Command completed successfully."},
	{Code: 1, Name: "error", Description: "Generic, unexpected failure when no command-specific code applies."},
	{Code: 2, Name: "missing_workspace", Description: "No workspace resolved; pass --workspace, set AGENTCLASH_WORKSPACE, or run 'agentclash link'."},

	// Global failure-class band (sysexits-aligned). Applies to every command
	// unless a command-specific code below overrides it.
	{Code: exitValidationError, Name: "validation_error", Description: "Invalid arguments, flags, input files, or configuration (sysexits EX_USAGE)."},
	{Code: exitNotFound, Name: "not_found", Description: "The referenced resource or file does not exist (HTTP 404 or a missing local file; sysexits EX_NOINPUT)."},
	{Code: exitRetryableFailure, Name: "retryable_failure", Description: "Transient failure — rate limiting (429), server errors (5xx), or a network/transport error. Mirrors `error.retryable: true` in the JSON envelope; safe to retry, honoring details.retry_after_seconds when present (sysexits EX_TEMPFAIL)."},
	{Code: exitAuthDenied, Name: "auth_denied", Description: "Authentication or permission failure (HTTP 401/403 or a local permission error; sysexits EX_NOPERM)."},

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
