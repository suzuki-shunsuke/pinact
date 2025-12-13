package di

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
)

func setupGHESServices(ctx context.Context, gh *github.Client, cfg *config.Config, logger *slog.Logger, flags *Flags, token string) (*ghesServices, error) {
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

	var ghesRepoService run.RepositoriesService
	var ghesGitService run.GitService
	var ghesPRService run.PullRequestsService
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
		ghesFallback = ghesConfig.Fallback
	}

	resolver := run.NewClientResolver(
		gh.Repositories, gh.Git,
		ghesRepoService, ghesGitService,
		ghesFallback,
		logger,
	)

	repoService := &run.RepositoriesServiceImpl{
		Tags:     map[string]*run.ListTagsResult{},
		Releases: map[string]*run.ListReleasesResult{},
		Commits:  map[string]*run.GetCommitSHA1Result{},
	}
	repoService.SetResolver(resolver)

	gitService := &run.GitServiceImpl{
		Commits: map[string]*run.GetCommitResult{},
	}
	gitService.SetResolver(resolver)

	prService := &run.PullRequestsServiceImpl{}
	prService.SetServices(gh.PullRequests, ghesPRService)

	return &ghesServices{
		repoService: repoService,
		gitService:  gitService,
		prService:   prService,
	}, nil
}
