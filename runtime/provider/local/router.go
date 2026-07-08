package local

import (
	"net/http"

	"github.com/agentclash/agentclash/runtime/provider"
)

// NewLocalRouter constructs a provider.Router wired to the local credential chain
// (process env → provider_keys.yaml → OS keychain). It never uses hosted
// workspace secrets.
func NewLocalRouter(httpClient *http.Client, opts ChainOptions) provider.Router {
	return provider.NewDefaultRouter(httpClient, NewChainResolver(opts))
}

// NewDefaultLocalRouter uses NewDefaultChainResolver().
func NewDefaultLocalRouter(httpClient *http.Client) provider.Router {
	return NewLocalRouter(httpClient, ChainOptions{Keychain: OSKeychain{}})
}
