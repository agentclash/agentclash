package e2b

import (
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/Atharva-Kanherkar/agentclash/backend/internal/sandbox"
)

func normalizeHTTPError(statusCode int, body string, notFoundErr error) error {
	switch statusCode {
	case http.StatusNotFound:
		if notFoundErr != nil {
			return notFoundErr
		}
		return fmt.Errorf("e2b resource not found: %s", body)
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("e2b authentication failed: %s", body)
	default:
		return fmt.Errorf("e2b request failed with status %d: %s", statusCode, body)
	}
}

func normalizeRPCError(err error) error {
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return err
	}
	switch connectErr.Code() {
	case connect.CodeNotFound:
		return sandbox.ErrFileNotFound
	default:
		return fmt.Errorf("e2b rpc failed: %w", err)
	}
}
