package temporalutil

import (
	"crypto/tls"
	"os"

	temporalsdk "go.temporal.io/sdk/client"
)

// NewClient creates a Temporal client, automatically enabling TLS and API key
// auth when TEMPORAL_API_KEY is set (required for Temporal Cloud).
func NewClient(hostPort, namespace string) (temporalsdk.Client, error) {
	opts := temporalsdk.Options{
		HostPort:  hostPort,
		Namespace: namespace,
	}

	apiKey := os.Getenv("TEMPORAL_API_KEY")
	if apiKey != "" {
		opts.ConnectionOptions = temporalsdk.ConnectionOptions{
			TLS: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}
		opts.Credentials = temporalsdk.NewAPIKeyStaticCredentials(apiKey)
	}

	return temporalsdk.Dial(opts)
}
