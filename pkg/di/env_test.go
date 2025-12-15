package di_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/di"
)

func TestSecrets_SetFromEnv(t *testing.T) {
	t.Parallel()
	data := []struct {
		name           string
		env            map[string]string
		expGitHubToken string
		expGHESToken   string
	}{
		{
			name:           "empty",
			env:            map[string]string{},
			expGitHubToken: "",
			expGHESToken:   "",
		},
		{
			name:           "github token only",
			env:            map[string]string{"GITHUB_TOKEN": "gh_token"},
			expGitHubToken: "gh_token",
			expGHESToken:   "",
		},
		{
			name:           "ghes token",
			env:            map[string]string{"GITHUB_TOKEN": "gh_token", "GHES_TOKEN": "ghes_token"},
			expGitHubToken: "gh_token",
			expGHESToken:   "ghes_token",
		},
		{
			name:           "github token enterprise",
			env:            map[string]string{"GITHUB_TOKEN": "gh_token", "GITHUB_TOKEN_ENTERPRISE": "enterprise_token"},
			expGitHubToken: "gh_token",
			expGHESToken:   "enterprise_token",
		},
		{
			name:           "github enterprise token",
			env:            map[string]string{"GITHUB_TOKEN": "gh_token", "GITHUB_ENTERPRISE_TOKEN": "enterprise_token2"},
			expGitHubToken: "gh_token",
			expGHESToken:   "enterprise_token2",
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			s := &di.Secrets{}
			s.SetFromEnv(func(key string) string {
				return d.env[key]
			})
			if s.GitHubToken != d.expGitHubToken {
				t.Errorf("GitHubToken: wanted %q, got %q", d.expGitHubToken, s.GitHubToken)
			}
			if s.GHESToken != d.expGHESToken {
				t.Errorf("GHESToken: wanted %q, got %q", d.expGHESToken, s.GHESToken)
			}
		})
	}
}

func TestSetEnv(t *testing.T) {
	t.Parallel()
	data := []struct {
		name               string
		env                map[string]string
		expGitHubRepo      string
		expIsGitHubActions bool
	}{
		{
			name:               "empty",
			env:                map[string]string{},
			expGitHubRepo:      "",
			expIsGitHubActions: false,
		},
		{
			name: "all values set",
			env: map[string]string{
				"GITHUB_REPOSITORY": "owner/repo",
				"GITHUB_ACTIONS":    "true",
			},
			expGitHubRepo:      "owner/repo",
			expIsGitHubActions: true,
		},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			flags := &di.Flags{}
			di.SetEnv(flags, func(key string) string {
				return d.env[key]
			})
			if flags.GitHubRepository != d.expGitHubRepo {
				t.Errorf("GitHubRepository: wanted %q, got %q", d.expGitHubRepo, flags.GitHubRepository)
			}
			if flags.IsGitHubActions != d.expIsGitHubActions {
				t.Errorf("IsGitHubActions: wanted %v, got %v", d.expIsGitHubActions, flags.IsGitHubActions)
			}
		})
	}
}
