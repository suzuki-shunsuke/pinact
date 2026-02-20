// Package github provides GitHub API client integration and authentication.
// This package abstracts GitHub API operations, handling client creation with
// proper authentication through environment variables or OS keyring storage.
// It manages OAuth2 token-based authentication, provides type aliases for
// commonly used GitHub API types, and configures HTTP clients for API calls.
// The package supports both authenticated and unauthenticated API access,
// with automatic fallback mechanisms for different authentication sources.
package github

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v83/github"
	"github.com/suzuki-shunsuke/ghtkn-go-sdk/ghtkn"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/keyring/ghtoken"
	"golang.org/x/oauth2"
)

type (
	ListOptions        = github.ListOptions
	Reference          = github.Reference
	Response           = github.Response
	Repository         = github.Repository
	RepositoryTag      = github.RepositoryTag
	RepositoryRelease  = github.RepositoryRelease
	Client             = github.Client
	GitObject          = github.GitObject
	Commit             = github.Commit
	CommitAuthor       = github.CommitAuthor
	Timestamp          = github.Timestamp
	PullRequestComment = github.PullRequestComment
)

// New creates a new GitHub API client with authentication.
// It configures the client with appropriate HTTP client based on available
// authentication methods (environment token or keyring).
func New(ctx context.Context, logger *slog.Logger, token string, keyringEnabled, ghtknEnabled bool) *Client {
	return github.NewClient(getHTTPClientForGitHub(ctx, logger, token, keyringEnabled, ghtknEnabled))
}

// Ptr returns a pointer to the provided value.
// This is a convenience function that delegates to github.Ptr for
// creating pointers to values, commonly needed for GitHub API structs.
func Ptr[T any](v T) *T {
	return github.Ptr(v)
}

// getHTTPClientForGitHub creates an HTTP client configured for GitHub API access.
// It handles authentication using environment token, keyring, or falls back
// to unauthenticated access. The client is configured with OAuth2 for authenticated requests.
func getHTTPClientForGitHub(ctx context.Context, logger *slog.Logger, token string, keyringEnabled, ghtknEnabled bool) *http.Client {
	ts := getTokenSourceForGitHub(logger, token, keyringEnabled, ghtknEnabled)
	if ts == nil {
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, ts)
}

func getTokenSourceForGitHub(logger *slog.Logger, token string, keyringEnabled, ghtknEnabled bool) oauth2.TokenSource {
	if token != "" {
		return oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
	}
	if keyringEnabled {
		return ghtoken.NewTokenSource(logger, KeyService)
	}
	if ghtknEnabled {
		return ghtkn.New().TokenSource(logger, &ghtkn.InputGet{})
	}
	return nil
}

// NewWithBaseURL creates a new GitHub API client with a custom base URL.
// This is used for GitHub Enterprise Server instances.
func NewWithBaseURL(ctx context.Context, baseURL, token string) (*Client, error) {
	httpClient := getHTTPClientForGitHubWithToken(ctx, token)
	return github.NewClient(httpClient).WithEnterpriseURLs(baseURL, baseURL) //nolint:wrapcheck
}

// getHTTPClientForGitHubWithToken creates an HTTP client with a specific token.
// Unlike getHTTPClientForGitHub, this does not fall back to keyring.
func getHTTPClientForGitHubWithToken(ctx context.Context, token string) *http.Client {
	if token == "" {
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
}
