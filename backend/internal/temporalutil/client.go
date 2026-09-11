package temporalutil

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"

	temporalsdk "go.temporal.io/sdk/client"
)

// ClientOption mutates Temporal client options before Dial.
type ClientOption func(*temporalsdk.Options)

// WithMetricsHandler attaches a Temporal MetricsHandler (Fleet 14 OTel bridge).
func WithMetricsHandler(handler temporalsdk.MetricsHandler) ClientOption {
	return func(opts *temporalsdk.Options) {
		if handler != nil {
			opts.MetricsHandler = handler
		}
	}
}

// NewClient creates a Temporal client using the connection settings validated at
// startup. It does not reread environment variables or certificate files.
func NewClient(hostPort, namespace string, connection ConnectionConfig, options ...ClientOption) (temporalsdk.Client, error) {
	opts, err := clientOptions(hostPort, namespace, connection, options...)
	if err != nil {
		return nil, err
	}
	return temporalsdk.Dial(opts)
}

func clientOptions(hostPort, namespace string, connection ConnectionConfig, options ...ClientOption) (temporalsdk.Options, error) {
	if !connection.configured {
		return temporalsdk.Options{}, fmt.Errorf("%w: connection settings must be loaded before creating a client", ErrInvalidConfig)
	}
	host, port, err := net.SplitHostPort(hostPort)
	numericPort, portErr := strconv.Atoi(port)
	if err != nil || !validServerName(host) || portErr != nil || numericPort < 1 || numericPort > 65535 {
		return temporalsdk.Options{}, fmt.Errorf("%w: TEMPORAL_HOST_PORT must be a valid host:port address", ErrInvalidConfig)
	}
	if namespace == "" || strings.IndexFunc(namespace, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return temporalsdk.Options{}, fmt.Errorf("%w: TEMPORAL_NAMESPACE must be nonempty and contain no whitespace or control characters", ErrInvalidConfig)
	}
	opts := temporalsdk.Options{}
	for _, opt := range options {
		if opt != nil {
			opt(&opts)
		}
	}
	// Preserve optional metrics/tuning while keeping the validated endpoint and
	// transport authoritative. Clone TLS because the SDK can configure its copy.
	opts.HostPort = hostPort
	opts.Namespace = namespace
	opts.Credentials = connection.credentials
	opts.ConnectionOptions.TLSDisabled = connection.tlsConfig == nil
	opts.ConnectionOptions.TLS = nil
	if connection.tlsConfig != nil {
		opts.ConnectionOptions.TLS = connection.tlsConfig.Clone()
	}
	return opts, nil
}
