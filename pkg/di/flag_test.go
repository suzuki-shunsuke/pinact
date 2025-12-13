package di_test

import (
	"testing"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/di"
)

func TestFlags_GetAPIURL(t *testing.T) {
	t.Parallel()
	data := []struct {
		name         string
		ghesAPIURL   string
		githubAPIURL string
		exp          string
	}{
		{name: "empty", ghesAPIURL: "", githubAPIURL: "", exp: ""},
		{name: "ghes api url set", ghesAPIURL: "https://ghes.example.com/api/v3", githubAPIURL: "", exp: "https://ghes.example.com/api/v3"},
		{name: "github api url is default", ghesAPIURL: "", githubAPIURL: "https://api.github.com", exp: ""},
		{name: "github api url is custom", ghesAPIURL: "", githubAPIURL: "https://custom.github.com/api/v3", exp: "https://custom.github.com/api/v3"},
		{name: "ghes api url takes precedence", ghesAPIURL: "https://ghes.example.com/api/v3", githubAPIURL: "https://custom.github.com/api/v3", exp: "https://ghes.example.com/api/v3"},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			flags := &di.Flags{GHESAPIURL: d.ghesAPIURL, GitHubAPIURL: d.githubAPIURL}
			if got := flags.GetAPIURL(); got != d.exp {
				t.Errorf("wanted %q, got %q", d.exp, got)
			}
		})
	}
}

func TestFlags_GHESFromEnv(t *testing.T) {
	t.Parallel()
	data := []struct {
		name            string
		ghesAPIURL      string
		fallbackEnabled bool
		expNil          bool
		expAPIURL       string
		expFallback     bool
	}{
		{name: "no api url", ghesAPIURL: "", expNil: true},
		{name: "with ghes api url", ghesAPIURL: "https://ghes.example.com/api/v3", expNil: false, expAPIURL: "https://ghes.example.com/api/v3", expFallback: false},
		{name: "with fallback enabled", ghesAPIURL: "https://ghes.example.com/api/v3", fallbackEnabled: true, expNil: false, expAPIURL: "https://ghes.example.com/api/v3", expFallback: true},
	}
	for _, d := range data {
		t.Run(d.name, func(t *testing.T) {
			t.Parallel()
			flags := &di.Flags{GHESAPIURL: d.ghesAPIURL, FallbackEnabled: d.fallbackEnabled}
			got := flags.GHESFromEnv()
			if d.expNil {
				if got != nil {
					t.Errorf("expected nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatal("expected non-nil, got nil")
			}
			if got.APIURL != d.expAPIURL {
				t.Errorf("APIURL: wanted %q, got %q", d.expAPIURL, got.APIURL)
			}
			if got.Fallback != d.expFallback {
				t.Errorf("Fallback: wanted %v, got %v", d.expFallback, got.Fallback)
			}
		})
	}
}
