package di

import (
	"github.com/suzuki-shunsuke/pinact/v4/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/config"
)

// Flags holds all command-line flags for the run command.
type Flags struct {
	*gflag.GlobalFlags

	// v4 flags
	// -verify and -v are urfave/cli aliases for -verify-comment, so they
	// share VerifyComment and do not need a separate field.
	VerifyComment bool
	VerifyMinAge  bool
	NoAPI         bool

	// -check and -diff are silent aliases for -fix=false in v4. They keep
	// their own destinations so buildParam can translate them.
	Check     bool
	Diff      bool
	Update    bool
	Fix       bool
	Format    string
	Separator string

	IsGitHubActions bool
	FallbackEnabled bool
	KeyringEnabled  bool
	GHTKNEnabled    bool

	GitHubRepository string
	GitHubAPIURL     string
	GitHubEventPath  string
	GHESAPIURL       string

	CWD string

	FixCount    int
	MinAge      int
	Include     []string
	Exclude     []string
	BranchToTag []string
	Args        []string
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
