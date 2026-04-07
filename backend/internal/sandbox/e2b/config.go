package e2b

import (
	"strings"
	"time"
)

const (
	defaultAPIBaseURL     = "https://api.e2b.app"
	defaultRequestTimeout = 30 * time.Second
	defaultDomain         = "e2b.app"
	defaultEnvdPort       = 49983
	defaultSandboxUser    = "root"
)

type Config struct {
	APIKey         string
	TemplateID     string
	APIBaseURL     string
	RequestTimeout time.Duration
}

func (c Config) apiBaseURL() string {
	if strings.TrimSpace(c.APIBaseURL) == "" {
		return defaultAPIBaseURL
	}
	return strings.TrimRight(strings.TrimSpace(c.APIBaseURL), "/")
}

func (c Config) requestTimeout() time.Duration {
	if c.RequestTimeout <= 0 {
		return defaultRequestTimeout
	}
	return c.RequestTimeout
}
