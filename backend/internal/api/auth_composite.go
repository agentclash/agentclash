package api

import (
	"fmt"
	"net/http"
)

// CompositeAuthenticator tries multiple authenticators based on the token format.
// CLI tokens (prefixed with "clitok_") are routed to the CLITokenAuthenticator.
// All other tokens are routed to the primary authenticator (WorkOS JWT).
type CompositeAuthenticator struct {
	primary  Authenticator // WorkOS JWT authenticator
	cliToken Authenticator // CLI token authenticator
}

func NewCompositeAuthenticator(primary, cliToken Authenticator) *CompositeAuthenticator {
	return &CompositeAuthenticator{primary: primary, cliToken: cliToken}
}

func (c *CompositeAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	token := extractBearerToken(r)
	if token == "" {
		return Caller{}, fmt.Errorf("%w: missing authorization header", ErrUnauthenticated)
	}

	if IsCLIToken(token) {
		return c.cliToken.Authenticate(r)
	}
	return c.primary.Authenticate(r)
}
