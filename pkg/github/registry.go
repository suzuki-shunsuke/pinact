package github

import (
	"context"
	"fmt"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

// ClientRegistry manages GitHub clients for github.com and a GHES instance.
// It maintains a default client for github.com and optionally a client
// for a configured GHES instance.
type ClientRegistry struct {
	defaultClient *Client
	ghesClient    *Client
	ghesConfig    *config.GHES
}

// NewClientRegistry creates a new ClientRegistry with clients for github.com
// and optionally a GHES instance.
func NewClientRegistry(ctx context.Context, defaultClient *Client, ghes *config.GHES) (*ClientRegistry, error) {
	registry := &ClientRegistry{
		defaultClient: defaultClient,
		ghesConfig:    ghes,
	}

	if ghes != nil {
		token := GetGHESToken()
		client, err := NewWithBaseURL(ctx, ghes.BaseURL, token)
		if err != nil {
			return nil, fmt.Errorf("create GHES client for %s: %w", ghes.BaseURL, err)
		}
		registry.ghesClient = client
	}

	return registry, nil
}

// GetGHESClient returns the GHES client if configured, or nil if not configured.
func (r *ClientRegistry) GetGHESClient() *Client {
	return r.ghesClient
}

// ResolveHost determines whether a repository should use GHES.
// It checks the owner against the GHES configuration and returns true
// if the repository should use GHES, false for github.com.
func (r *ClientRegistry) ResolveHost(owner string) bool {
	if r.ghesConfig == nil {
		return false
	}
	return r.ghesConfig.Match(owner)
}
