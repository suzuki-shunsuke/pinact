package di

import (
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
)

// Flags holds all command-line flags for the run command.
type Flags struct {
	*flag.GlobalFlags

	Verify bool
	Check  bool
	Update bool
	Review bool
	Fix    bool
	Diff   bool
	Format string

	IsGitHubActions bool
	FallbackEnabled bool
	KeyringEnabled  bool
	GHTKNEnabled    bool

	RepoOwner string
	RepoName  string
	SHA       string

	GitHubRepository string
	GitHubAPIURL     string
	GitHubEventPath  string
	GHESAPIURL       string

	PWD string

	FixCount int
	PR       int
	MinAge   int
	Include  []string
	Exclude  []string
	Args     []string
}

const defaultGitHubAPIURL = "https://api.github.com"

// GetAPIURL returns the GHES API URL from environment variables.
func (f *Flags) GetAPIURL() string {
	if f.GHESAPIURL != "" {
		return f.GHESAPIURL
	}
	if f.GitHubAPIURL == "" || f.GitHubAPIURL == defaultGitHubAPIURL {
		return ""
	}
	return f.GitHubAPIURL
}

// GHESFromEnv creates a GHES configuration from environment variables.
func (f *Flags) GHESFromEnv() *config.GHES {
	apiURL := f.GetAPIURL()
	if apiURL == "" {
		return nil
	}
	return &config.GHES{
		APIURL:   apiURL,
		Fallback: f.FallbackEnabled,
	}
}

// MergeFromEnv merges environment variable values into GHES configuration.
func (f *Flags) MergeFromEnv(g *config.GHES) {
	if g == nil {
		return
	}
	if g.APIURL == "" {
		g.APIURL = f.GetAPIURL()
	}
	// Environment variable can enable fallback (but not disable it if already set in config)
	if !g.Fallback && f.FallbackEnabled {
		g.Fallback = true
	}
}
