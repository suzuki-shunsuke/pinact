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
	"net/http"
	"os"

	"github.com/google/go-github/v76/github"
	"github.com/sirupsen/logrus"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/keyring/ghtoken"
	"golang.org/x/oauth2"
)

type (
	ListOptions        = github.ListOptions
	Reference          = github.Reference
	Response           = github.Response
	RepositoryTag      = github.RepositoryTag
	RepositoryRelease  = github.RepositoryRelease
	Client             = github.Client
	GitObject          = github.GitObject
	Commit             = github.Commit
	PullRequestComment = github.PullRequestComment
)

// New creates a new GitHub API client with authentication.
// It configures the client with appropriate HTTP client based on available
// authentication methods (environment token or keyring).
//
// Parameters:
//   - ctx: context for OAuth2 token source
//   - logE: logrus entry for structured logging
//
// Returns a configured GitHub API client.
func New(ctx context.Context, logE *logrus.Entry) *Client {
	return github.NewClient(getHTTPClientForGitHub(ctx, logE, getGitHubToken()))
}

// Ptr returns a pointer to the provided value.
// This is a convenience function that delegates to github.Ptr for
// creating pointers to values, commonly needed for GitHub API structs.
//
// Parameters:
//   - v: value to get a pointer to
//
// Returns a pointer to the value.
func Ptr[T any](v T) *T {
	return github.Ptr(v)
}

// getGitHubToken retrieves the GitHub token from environment variables.
// It reads the GITHUB_TOKEN environment variable for authentication.
//
// Returns the GitHub token string or empty string if not set.
func getGitHubToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

// checkKeyringEnabled checks if keyring authentication is enabled.
// It examines the PINACT_KEYRING_ENABLED environment variable to determine
// if OS keyring should be used for token storage and retrieval.
//
// Returns true if keyring is enabled, false otherwise.
func checkKeyringEnabled() bool {
	return os.Getenv("PINACT_KEYRING_ENABLED") == "true"
}

// getHTTPClientForGitHub creates an HTTP client configured for GitHub API access.
// It handles authentication using environment token, keyring, or falls back
// to unauthenticated access. The client is configured with OAuth2 for authenticated requests.
//
// Parameters:
//   - ctx: context for OAuth2 token source
//   - logE: logrus entry for structured logging
//   - token: GitHub token for authentication (empty string for alternative auth)
//
// Returns an HTTP client configured for GitHub API access.
func getHTTPClientForGitHub(ctx context.Context, logE *logrus.Entry, token string) *http.Client {
	if token == "" {
		if checkKeyringEnabled() {
			return oauth2.NewClient(ctx, ghtoken.NewTokenSource(logE, KeyService))
		}
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
}
