package github

import (
	"context"
	"net/http"
	"os"

	"github.com/google/go-github/v71/github"
	"github.com/sirupsen/logrus"
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

func New(ctx context.Context, logE *logrus.Entry) *Client {
	return github.NewClient(getHTTPClientForGitHub(ctx, logE, getGitHubToken()))
}

func getGitHubToken() string {
	return os.Getenv("GITHUB_TOKEN")
}

func checkKeyringEnabled() bool {
	return os.Getenv("PINACT_KEYRING_ENABLED") == "true"
}

func getHTTPClientForGitHub(ctx context.Context, logE *logrus.Entry, token string) *http.Client {
	if token == "" {
		if checkKeyringEnabled() {
			return oauth2.NewClient(ctx, NewKeyringTokenSource(logE))
		}
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
}
