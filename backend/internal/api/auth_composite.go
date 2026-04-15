package api

import "net/http"

// CompositeAuthenticator routes CLI bearer tokens to the CLI token authenticator
// while preserving the primary authenticator's normal behavior for all other requests.
type CompositeAuthenticator struct {
	primary  Authenticator
	cliToken Authenticator
}

func NewCompositeAuthenticator(primary, cliToken Authenticator) *CompositeAuthenticator {
	return &CompositeAuthenticator{primary: primary, cliToken: cliToken}
}

func (c *CompositeAuthenticator) Authenticate(r *http.Request) (Caller, error) {
	token, ok := bearerToken(r)
	if ok && IsCLIToken(token) {
		return c.cliToken.Authenticate(r)
	}
	return c.primary.Authenticate(r)
}
