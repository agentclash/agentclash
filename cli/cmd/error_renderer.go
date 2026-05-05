package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/url"
	"strings"

	cliapi "github.com/agentclash/agentclash/cli/internal/api"
)

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
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Status  int            `json:"status,omitempty"`
	Details map[string]any `json:"details"`
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
	return flagJSON || flagOutput == "json"
}

func exitCodeForError(err error) int {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) {
		return exitErr.Code
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
		return structuredError{Code: code, Message: message, Status: apiErr.StatusCode, Details: details}
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
		return structuredError{Code: "request_failed", Message: err.Error(), Details: details}
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return structuredError{Code: "request_failed", Message: err.Error(), Details: details}
	}

	message := err.Error()
	if strings.Contains(message, "request failed:") {
		return structuredError{Code: "request_failed", Message: message, Details: details}
	}
	if strings.HasPrefix(message, "loading config:") {
		return structuredError{Code: "invalid_config", Message: message, Details: details}
	}

	return structuredError{Code: "invalid_argument", Message: message, Details: details}
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
