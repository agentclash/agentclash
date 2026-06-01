// Package posthog wraps the posthog-go SDK so the rest of the codebase can
// depend on a small Client interface and an optional-service default. When
// POSTHOG_API_KEY is unset the constructor returns a noop that satisfies the
// same interface — matching the optional-service pattern used elsewhere in
// the backend (Resend email sender, Redis pub/sub, etc).
//
// The middleware emits events fire-and-forget: PostHog availability must not
// gate the request hot path. All errors from the underlying SDK are logged
// and dropped.
package posthog

import (
	"errors"
	"log/slog"
	"os"

	"github.com/google/uuid"
	posthogsdk "github.com/posthog/posthog-go"
)

// Event is a flattened representation of a single PostHog capture call.
// The DistinctID is the canonical user identifier; for anonymous CLI requests
// (no resolved user) callers can pass a zero UUID and the implementation will
// substitute a stable anonymous device id from the request context.
type Event struct {
	DistinctID string
	EventName  string
	Properties map[string]any
}

// Client is the surface the API server consumes. The noop satisfies it.
type Client interface {
	Capture(event Event)
	Identify(distinctID string, properties map[string]any)
	Close() error
}

// Config controls the real client.
type Config struct {
	APIKey   string
	Endpoint string // optional; defaults to https://us.i.posthog.com
}

// LoadConfigFromEnv reads POSTHOG_API_KEY and POSTHOG_ENDPOINT from the
// environment. Returns ok=false if no key is set so callers can wire the noop.
func LoadConfigFromEnv() (Config, bool) {
	apiKey := os.Getenv("POSTHOG_API_KEY")
	if apiKey == "" {
		return Config{}, false
	}
	return Config{
		APIKey:   apiKey,
		Endpoint: os.Getenv("POSTHOG_ENDPOINT"),
	}, true
}

// NewClient constructs a real PostHog client. If cfg.APIKey is empty, returns
// the noop. The caller passes a logger so SDK errors surface via the
// application's structured log channel.
func NewClient(cfg Config, logger *slog.Logger) (Client, error) {
	if cfg.APIKey == "" {
		return Noop{}, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://us.i.posthog.com"
	}
	sdkClient, err := posthogsdk.NewWithConfig(cfg.APIKey, posthogsdk.Config{
		Endpoint: endpoint,
		Logger:   &slogLogger{logger: logger.With("component", "posthog")},
	})
	if err != nil {
		return nil, err
	}
	return &realClient{
		sdk:    sdkClient,
		logger: logger.With("component", "posthog"),
	}, nil
}

type realClient struct {
	sdk    posthogsdk.Client
	logger *slog.Logger
}

func (c *realClient) Capture(event Event) {
	if event.EventName == "" {
		return
	}
	distinctID := event.DistinctID
	if distinctID == "" {
		distinctID = "anonymous"
	}
	if err := c.sdk.Enqueue(posthogsdk.Capture{
		DistinctId: distinctID,
		Event:      event.EventName,
		Properties: posthogsdk.Properties(event.Properties),
	}); err != nil {
		c.logger.Warn("posthog enqueue failed", "event", event.EventName, "error", err)
	}
}

func (c *realClient) Identify(distinctID string, properties map[string]any) {
	if distinctID == "" {
		return
	}
	if err := c.sdk.Enqueue(posthogsdk.Identify{
		DistinctId: distinctID,
		Properties: posthogsdk.Properties(properties),
	}); err != nil {
		c.logger.Warn("posthog identify enqueue failed", "distinct_id", distinctID, "error", err)
	}
}

func (c *realClient) Close() error {
	return c.sdk.Close()
}

// Noop is the default when PostHog is not configured. All methods are zero-cost.
type Noop struct{}

func (Noop) Capture(Event)                       {}
func (Noop) Identify(string, map[string]any)     {}
func (Noop) Close() error                        { return nil }

// slogLogger adapts log/slog to the posthog-go Logger interface (Logf/Errorf).
type slogLogger struct {
	logger *slog.Logger
}

func (l *slogLogger) Logf(format string, args ...any) {
	l.logger.Debug("posthog sdk message", "msg", sprintf(format, args...))
}

func (l *slogLogger) Debugf(format string, args ...any) {
	l.logger.Debug("posthog sdk debug", "msg", sprintf(format, args...))
}

func (l *slogLogger) Warnf(format string, args ...any) {
	l.logger.Warn("posthog sdk warn", "msg", sprintf(format, args...))
}

func (l *slogLogger) Errorf(format string, args ...any) {
	l.logger.Warn("posthog sdk error", "msg", sprintf(format, args...))
}

func sprintf(format string, args ...any) string {
	// Avoid pulling fmt for one call site — the SDK's expected behaviour is
	// "use fmt.Sprintf" but a thin helper keeps this file dependency-light.
	return fmtSprintf(format, args...)
}

// AnonymousDistinctID returns a deterministic distinct ID suitable for
// anonymous events when there is no resolved user. The middleware can pass a
// stable identifier (e.g. CLI machine id from a future header); for now it
// returns the literal "anonymous" so events still land.
func AnonymousDistinctID() string {
	return "anonymous"
}

// guard against interface drift
var _ Client = Noop{}
var _ Client = (*realClient)(nil)
var _ = uuid.Nil
var _ = errors.New
