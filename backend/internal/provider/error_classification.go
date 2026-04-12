package provider

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
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

func parseRetryAfter(header http.Header) time.Duration {
	value := strings.TrimSpace(header.Get("Retry-After"))
	if value == "" {
		return 0
	}
	seconds, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func normalizeCredentialError(providerKey string, err error) error {
	if failure, ok := AsFailure(err); ok {
		failure.ProviderKey = providerKey
		return failure
	}
	return NewFailure(providerKey, FailureCodeCredentialUnavailable, err.Error(), false, err)
}
