package di

// Secrets holds sensitive tokens for GitHub API authentication.
type Secrets struct {
	GitHubToken          string
	GHESToken            string
	GitHubTokenForReview string
	GHESTokenForReview   string
}

// SetFromEnv sets secrets from environment variables.
func (s *Secrets) SetFromEnv(getEnv func(string) string) {
	s.GitHubToken = getEnv("PINACT_GITHUB_TOKEN")
	if s.GitHubToken == "" {
		s.GitHubToken = getEnv("GITHUB_TOKEN")
	}
	s.GitHubTokenForReview = getEnv("PINACT_GITHUB_TOKEN_FOR_REVIEW")
	s.GHESTokenForReview = getEnv("PINACT_GHES_TOKEN_FOR_REVIEW")
	for _, envName := range []string{"PINACT_GHES_TOKEN", "GHES_TOKEN", "GITHUB_TOKEN_ENTERPRISE", "GITHUB_ENTERPRISE_TOKEN"} {
		if token := getEnv(envName); token != "" {
			s.GHESToken = token
			return
		}
	}
}

// SetEnv populates flags from environment variables.
func SetEnv(flags *Flags, getEnv func(string) string) {
	flags.GitHubRepository = getEnv("GITHUB_REPOSITORY")
	flags.GitHubAPIURL = getEnv("GITHUB_API_URL")
	flags.GitHubEventPath = getEnv("GITHUB_EVENT_PATH")
	flags.GHESAPIURL = getEnv("GHES_API_URL")
	trueS := "true"
	flags.IsGitHubActions = getEnv("GITHUB_ACTIONS") == trueS
	flags.FallbackEnabled = getEnv("PINACT_GHES_FALLBACK") == trueS
	flags.KeyringEnabled = getEnv("PINACT_KEYRING_ENABLED") == trueS
}
