package cmd

// ErrorCode documents a stable error code the CLI itself can synthesize in
// the structured error envelope (`error.code`). Backend error codes pass
// through verbatim and are not enumerable here — agents should treat this
// registry as the CLI-local vocabulary and branch on `error.code` strings,
// never on message prose. Published via `agentclash schema`.
type ErrorCode struct {
	Code        string `json:"code" yaml:"code"`
	Description string `json:"description" yaml:"description"`
	Retryable   bool   `json:"retryable,omitempty" yaml:"retryable,omitempty"`
}

var documentedErrorCodes = []ErrorCode{
	{Code: "invalid_argument", Description: "Invalid flags, arguments, or input; also the default classification for unrecognized local errors."},
	{Code: "invalid_config", Description: "The CLI configuration file failed to load or parse."},
	{Code: "file_not_found", Description: "A referenced local file does not exist."},
	{Code: "permission_denied", Description: "A local file or directory was not accessible."},
	{Code: "request_failed", Description: "The HTTP request could not complete (network/transport failure).", Retryable: true},
	{Code: "api_error", Description: "The server returned an error envelope without a code."},
	{Code: "command_failed", Description: "A command finished with a command-specific non-zero exit (see exit_codes)."},
	{Code: "missing_workspace", Description: "No workspace resolved; pass --workspace, set AGENTCLASH_WORKSPACE, or run 'agentclash link'."},
	{Code: "follow_timeout", Description: "A --follow loop gave up after --timeout; the underlying job keeps running server-side.", Retryable: true},
	{Code: "stream_reconnect_exhausted", Description: "The SSE event stream dropped repeatedly without delivering events; resume with `run events --since <id>`.", Retryable: true},
}
