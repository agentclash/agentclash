package connectors

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/agentclash/agentclash/runtime/provider"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// Client holds network capabilities only. Secrets must be resolved and approval
// checked by the canonical owner immediately before invoking DoJSON.
type Client struct {
	ResolveSecret func(context.Context) (string, error)
	HTTP          *http.Client
	Resolver      provider.EndpointResolver
	Dialer        interface {
		DialContext(context.Context, string, string) (net.Conn, error)
	}
}

func (c Client) DoJSON(ctx context.Context, endpoint, auth, secret string, payload []byte, deadline time.Time) ([]byte, error) {
	if len(payload) > MaxBodyBytes {
		return nil, errors.New("connector request exceeds 1 MiB")
	}
	if deadline.IsZero() || !deadline.After(time.Now()) {
		return nil, errors.New("connector deadline elapsed")
	}
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	client, err := provider.ConnectorHTTPClient(ctx, endpoint, c.HTTP, c.Resolver, c.Dialer)
	if err != nil {
		return nil, err
	}
	if c.ResolveSecret != nil {
		secret, err = c.ResolveSecret(ctx)
		if err != nil {
			return nil, err
		}
	}

	if len(secret) == 0 || len(secret) > 65536 || strings.ContainsAny(secret, "\r\n") {
		return nil, errors.New("invalid workspace credential")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, errors.New("invalid connector request")
	}
	req.Header.Set("Content-Type", "application/json")
	switch auth {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+secret)
	case "api_key":
		req.Header.Set("X-API-Key", secret)
	default:
		return nil, errors.New("unsupported connector authentication")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, errors.New("connector outcome unknown; remote execution may have started")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("connector returned a non-success response; execution outcome is unknown")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxBodyBytes+1))
	if err != nil {
		return nil, errors.New("connector response incomplete; execution outcome is unknown")
	}
	if _, err = Decode(body); err != nil {
		return nil, err
	}
	return body, nil
}
func (c Client) InvokeHTTPJSON(ctx context.Context, config Config, secret string, payload json.RawMessage, deadline time.Time) (json.RawMessage, error) {
	body, err := config.Request(payload)
	if err != nil {
		return nil, err
	}
	response, err := c.DoJSON(ctx, config.EndpointURL, config.Auth, secret, body, deadline)
	if err != nil {
		return nil, err
	}
	decoded, err := Decode(response)
	if err != nil {
		return nil, err
	}
	value, err := Pointer(decoded, config.ResponsePointer)
	if err != nil {
		return nil, err
	}
	return json.Marshal(value)
}
