package provider

import (
	"context"
	"net"
	"net/http"
)

// ConnectorHTTPClient reuses provider DNS validation and pinned TLS transport.
// A fresh client is required for every dispatch; redirects are never followed.
// Injected resolver/dialer capabilities support deterministic local fixtures,
// while the transport itself always enforces origin and TLS verification.
func ConnectorHTTPClient(ctx context.Context, endpoint string, base *http.Client, resolver EndpointResolver, dialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}) (*http.Client, error) {
	g := endpointGuard{resolver: resolver, dialer: dialer}.withDefaults()
	validated, err := validateBaseURL(ctx, endpoint, g.resolver)
	if err != nil {
		return nil, err
	}
	client, err := g.clientFor(validated, base)
	if err != nil {
		return nil, err
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Transport = guardedRoundTripper{next: client.Transport, origin: endpointOriginFromURL(validated.parsedURL)}
	return client, nil
}
