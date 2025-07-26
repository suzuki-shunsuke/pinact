package github

import (
	"context"
	"net/http"
	"os"

	"github.com/google/go-github/v74/github"
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

func New(ctx context.Context, logE *logrus.Entry) *Client {
	return github.NewClient(getHTTPClientForGitHub(ctx, logE, getGitHubToken()))
}

func Ptr[T any](v T) *T {
	return github.Ptr(v)
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
			return oauth2.NewClient(ctx, ghtoken.NewTokenSource(logE, KeyService))
		}
		return http.DefaultClient
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	))
}
