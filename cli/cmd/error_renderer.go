package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/url"
	"strings"
	"time"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

var runtimeOutputJSON bool

type cliError struct {
	Code    string
	Message string
	Err     error
}

func (e *cliError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *cliError) Unwrap() error {
	return e.Err
}

type structuredErrorEnvelope struct {
	Error structuredError `json:"error"`
}

type structuredError struct {
	Code     string         `json:"code"`
	Message  string         `json:"message"`
	Status   int            `json:"status,omitempty"`
	Details  map[string]any `json:"details"`
	NextStep string         `json:"next_step,omitempty"`
	// Retryable is always emitted (never omitted) so agents can branch on it
	// without an existence check: true for rate limiting (429), server-side
	// failures (5xx), and transport errors; false for everything else. The
	// process exit code mirrors it — retryable failures exit exitRetryable.
	Retryable bool `json:"retryable"`
}

// RenderError writes a machine-readable error envelope when JSON output was
// explicitly requested. It returns the process exit code and whether it
// rendered or intentionally suppressed output for a silent ExitCodeError.
func RenderError(err error, w io.Writer) (int, bool) {
	code := exitCodeForError(err)
	if err == nil || !structuredErrorOutputRequested() {
		return code, false
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) && exitErr.Silent() {
		return code, true
	}

	_ = json.NewEncoder(w).Encode(structuredErrorEnvelope{
		Error: classifyStructuredError(err),
	})
	return code, true
}

func structuredErrorOutputRequested() bool {
	return flagJSON || flagOutput == "json" || runtimeOutputJSON
}

func setRuntimeOutputFormat(format string) {
	runtimeOutputJSON = format == "json"
}

// exitCodeForError maps an error to the process exit code. Command-specific
// ExitCodeErrors always win (compare gate, ci run, prompt-eval). Everything
// else lands in a small global band (documented in documentedExitCodes and
// `agentclash schema`) so agents and CI can branch on the failure class
// without parsing output:
//
//	exitValidationError (64)  usage / validation errors
//	exitNotFound        (66)  the referenced resource or file does not exist
//	exitRetryableFailure(75)  transient — mirrors the envelope's retryable:true
//	exitAuthDenied      (77)  authentication or permission failures
//	1                         anything unclassified
func exitCodeForError(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	var apiErr *cliapi.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.StatusCode == 401 || apiErr.StatusCode == 403:
			return exitAuthDenied
		case apiErr.StatusCode == 404:
			return exitNotFound
		case apiErr.Retryable():
			return exitRetryableFailure
		default:
			return 1
		}
	}

	var localErr *cliError
	if errors.As(err, &localErr) {
		// RequireWorkspace normally os.Exit(2)s itself; keep the documented
		// code if the same error ever travels the error-return path instead.
		if localErr.Code == "missing_workspace" {
			return 2
		}
		return exitValidationError
	}

	if errors.Is(err, fs.ErrNotExist) {
		return exitNotFound
	}
	if errors.Is(err, fs.ErrPermission) {
		return exitAuthDenied
	}

	var urlErr *url.Error
	var netErr net.Error
	if errors.As(err, &urlErr) || errors.As(err, &netErr) || strings.Contains(err.Error(), "request failed:") {
		return exitRetryableFailure
	}
	if strings.HasPrefix(err.Error(), "loading config:") {
		return exitValidationError
	}
	return 1
}

func classifyStructuredError(err error) structuredError {
	details := map[string]any{}

	var apiErr *cliapi.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.Code
		if code == "" {
			code = "api_error"
		}
		message := apiErr.Message
		if message == "" {
			message = apiErr.Error()
		}
		// Carry the machine-readable quota/plan fields the API returned so an
		// agent can branch on them instead of re-parsing the prose message.
		if apiErr.PlanKey != "" {
			details["plan_key"] = apiErr.PlanKey
		}
		if apiErr.UpgradeTarget != "" {
			details["upgrade_target"] = apiErr.UpgradeTarget
		}
		if apiErr.Limit != nil {
			details["limit"] = *apiErr.Limit
		}
		if apiErr.Used != nil {
			details["used"] = *apiErr.Used
		}
		if apiErr.Remaining != nil {
			details["remaining"] = *apiErr.Remaining
		}
		if apiErr.ResetAt != nil {
			details["reset_at"] = apiErr.ResetAt.UTC().Format(time.RFC3339)
		}
		if apiErr.ExpiresAt != nil {
			details["expires_at"] = apiErr.ExpiresAt.UTC().Format(time.RFC3339)
		}
		if apiErr.RetryAfterSeconds != nil {
			details["retry_after_seconds"] = *apiErr.RetryAfterSeconds
		}
		return structuredError{
			Code:      code,
			Message:   message,
			Status:    apiErr.StatusCode,
			Details:   details,
			NextStep:  apiErrorNextStep(apiErr),
			Retryable: apiErr.Retryable(),
		}
	}

	var localErr *cliError
	if errors.As(err, &localErr) {
		return structuredError{Code: nonEmpty(localErr.Code, "invalid_argument"), Message: localErr.Error(), Details: details}
	}

	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return structuredError{Code: "command_failed", Message: exitErr.Error(), Details: details}
	}

	if errors.Is(err, fs.ErrNotExist) {
		return structuredError{Code: "file_not_found", Message: err.Error(), Details: details}
	}
	if errors.Is(err, fs.ErrPermission) {
		return structuredError{Code: "permission_denied", Message: err.Error(), Details: details}
	}

	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return structuredError{Code: "request_failed", Message: err.Error(), Details: details, Retryable: true}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return structuredError{Code: "request_failed", Message: err.Error(), Details: details, Retryable: true}
	}

	message := err.Error()
	if strings.Contains(message, "request failed:") {
		return structuredError{Code: "request_failed", Message: message, Details: details, Retryable: true}
	}
	if strings.HasPrefix(message, "loading config:") {
		return structuredError{Code: "invalid_config", Message: message, Details: details}
	}

	return structuredError{Code: "invalid_argument", Message: message, Details: details}
}

// apiErrorNextStep returns a short, actionable hint for common API failures,
// mirroring the doctor `next_step` convention. Empty when no specific hint
// applies, so the envelope's next_step stays omitted.
func apiErrorNextStep(e *cliapi.APIError) string {
	switch {
	case e.IsBillingGate():
		switch {
		case e.UpgradeTarget != "":
			return "Open the organization billing page in the AgentClash web app to upgrade, or wait for the quota to reset."
		case e.ResetAt != nil:
			return "Usage limit reached with no upgrade path configured — wait for the quota to reset (see details.reset_at), or open the organization billing page to change plans."
		default:
			return "Open the organization billing page in the AgentClash web app to update billing."
		}
	case e.StatusCode == 401:
		return "Run `agentclash auth login` (or set AGENTCLASH_TOKEN) and retry."
	case e.StatusCode == 403:
		return "Check workspace access with `agentclash workspace list`; you may lack permission for this resource."
	case e.StatusCode == 404:
		return "Verify the resource ID; list available resources with the matching `... list` command."
	default:
		return ""
	}
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
