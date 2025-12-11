package github

import (
	"context"
	"fmt"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

// ClientRegistry manages multiple GitHub clients for different hosts.
// It maintains a default client for github.com and separate clients
// for each configured GHES instance.
type ClientRegistry struct {
	defaultClient *Client
	ghesClients   map[string]*Client // key: host
	ghesConfigs   []*config.GHES
}

// NewClientRegistry creates a new ClientRegistry with clients for github.com
// and all configured GHES instances.
//
// Parameters:
//   - ctx: context for OAuth2 token source
//   - defaultClient: pre-configured client for github.com
//   - ghesConfigs: GHES configurations from the config file
//
// Returns a configured ClientRegistry or an error if GHES client creation fails.
func NewClientRegistry(ctx context.Context, defaultClient *Client, ghesConfigs []*config.GHES) (*ClientRegistry, error) {
	registry := &ClientRegistry{
		defaultClient: defaultClient,
		ghesClients:   make(map[string]*Client),
		ghesConfigs:   ghesConfigs,
	}

	for _, ghes := range ghesConfigs {
		token := GetGHESToken(ghes.Host)
		client, err := NewWithBaseURL(ctx, "https://"+ghes.Host, token)
		if err != nil {
			return nil, fmt.Errorf("create GHES client for %s: %w", ghes.Host, err)
		}
		registry.ghesClients[ghes.Host] = client
	}

	return registry, nil
}

// GetClient returns the appropriate GitHub client for the given host.
// If host is empty, returns the default github.com client.
//
// Parameters:
//   - host: hostname of the GitHub instance (empty for github.com)
//
// Returns the GitHub client for the specified host.
func (r *ClientRegistry) GetClient(host string) *Client {
	if host == "" {
		return r.defaultClient
	}
	if client, ok := r.ghesClients[host]; ok {
		return client
	}
	return r.defaultClient
}

// ResolveHost determines which GitHub host should be used for an action.
// It checks the action name against all GHES configurations and returns
// the matching host, or empty string for github.com.
//
// Parameters:
//   - actionName: action name to check (format: owner/repo)
//
// Returns the GHES host if matched, empty string for github.com.
func (r *ClientRegistry) ResolveHost(actionName string) string {
	for _, ghes := range r.ghesConfigs {
		if ghes.Match(actionName) {
			return ghes.Host
		}
	}
	return ""
}

// GetClientForAction returns the appropriate GitHub client for an action.
// It resolves the host based on the action name and returns the corresponding client.
//
// Parameters:
//   - actionName: action name to get client for (format: owner/repo)
//
// Returns the GitHub client for the action's host.
func (r *ClientRegistry) GetClientForAction(actionName string) *Client {
	host := r.ResolveHost(actionName)
	return r.GetClient(host)
}
