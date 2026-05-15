package di

import (
	"context"
	"fmt"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

// setupGHESServices creates GitHub API services with GHES (GitHub Enterprise Server) support.
// It configures a ClientResolver that routes API requests to either GHES or github.com
// based on the configuration. When GHES is enabled with fallback, repositories are first
// checked on GHES and fall back to github.com if not found.
// If reviewToken/ghesReviewToken is provided and flags.Review is true, the review token is used for PR comments.
func setupGHESServices(ctx context.Context, gh *github.Client, cfg *config.Config, flags *Flags, token, reviewToken, ghesReviewToken string) (*ghesServices, error) { //nolint:funlen
	ghesConfig := cfg.GHES
	if ghesConfig == nil {
		ghesConfig = flags.GHESFromEnv()
	} else {
		// Merge environment variables into config file settings
		flags.MergeFromEnv(ghesConfig)
	}
	if err := ghesConfig.Validate(); err != nil {
		return nil, fmt.Errorf("validate GHES configuration: %w", err)
	}

	var ghesRepoService github.RepositoriesService
	var ghesGitService github.GitService
	var ghesPRService github.PullRequestsService
	var ghesFallback bool

	if ghesConfig.IsEnabled() {
		registry, err := github.NewClientRegistry(ctx, gh, ghesConfig, token)
		if err != nil {
			return nil, fmt.Errorf("create GitHub client registry: %w", err)
		}
		client := registry.GetGHESClient()
		ghesRepoService = client.Repositories
		ghesGitService = client.Git
		ghesPRService = client.PullRequests
		// Use GHES review token for PR comments if provided
		if ghesReviewToken != "" && flags.Review {
			reviewClient, err := github.NewWithBaseURL(ctx, ghesConfig.APIURL, ghesReviewToken)
			if err != nil {
				return nil, fmt.Errorf("create GHES review client: %w", err)
			}
			ghesPRService = reviewClient.PullRequests
		}
		ghesFallback = ghesConfig.Fallback
	}

	resolver := github.NewClientResolver(
		gh.Repositories, gh.Git,
		ghesRepoService, ghesGitService,
		ghesFallback,
	)

	repoService := &github.RepositoriesServiceImpl{
		Tags:     map[string]*github.ListTagsResult{},
		Releases: map[string]*github.ListReleasesResult{},
		Commits:  map[string]*github.GetCommitSHA1Result{},
	}
	repoService.SetResolver(resolver)

	gitService := &github.GitServiceImpl{
		Commits: map[string]*github.GetCommitResult{},
	}
	gitService.SetResolver(resolver)

	prService := &github.PullRequestsServiceImpl{}
	defaultPRService := gh.PullRequests
	// Use review token for PR comments if provided
	if reviewToken != "" && flags.Review {
		reviewClient := github.New(ctx, nil, reviewToken, false)
		defaultPRService = reviewClient.PullRequests
	}
	prService.SetServices(defaultPRService, ghesPRService)

	return &ghesServices{
		repoService: repoService,
		gitService:  gitService,
		prService:   prService,
	}, nil
}
