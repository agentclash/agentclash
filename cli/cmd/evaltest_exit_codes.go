package cmd

// Stable exit codes for `agentclash evaltest run`.
// See schemas/evaltest/README.md for the contract.
const (
	evaltestExitPass           = 0
	evaltestExitAssertionFail  = 1
	evaltestExitConfigError    = 2
	evaltestExitProviderError  = 3
	evaltestExitInternalError  = 4
)
