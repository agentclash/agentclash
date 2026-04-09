package provider

import (
	"net"
	"strings"
)

func classifyTransportError(providerKey string, err error) error {
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return NewFailure(providerKey, FailureCodeTimeout, "provider request timed out", true, err)
	}
	if strings.Contains(strings.ToLower(err.Error()), "context deadline exceeded") {
		return NewFailure(providerKey, FailureCodeTimeout, "provider request timed out", true, err)
	}
	return NewFailure(providerKey, FailureCodeUnavailable, "provider request failed", true, err)
}

func normalizeCredentialError(providerKey string, err error) error {
	if failure, ok := AsFailure(err); ok {
		failure.ProviderKey = providerKey
		return failure
	}
	return NewFailure(providerKey, FailureCodeCredentialUnavailable, err.Error(), false, err)
}
