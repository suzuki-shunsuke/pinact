package github

import (
	"context"
	"net/http"
	"os"

	"github.com/google/go-github/v70/github"
	"golang.org/x/oauth2"
)

type (
	ListOptions       = github.ListOptions
	Reference         = github.Reference
	Response          = github.Response
	RepositoryTag     = github.RepositoryTag
	RepositoryRelease = github.RepositoryRelease
	Client            = github.Client
	GitObject         = github.GitObject
	Commit            = github.Commit
)

func New(ctx context.Context) *Client {
	return github.NewClient(getHTTPClientForGitHub(ctx, getGitHubToken()))
}

func getGitHubToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

func getHTTPClientForGitHub(ctx context.Context, token string) *http.Client {
	if token == "" {
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
}
