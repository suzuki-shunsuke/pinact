// Package updaterepos implements the update-repos command.
package updaterepos

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/suzuki-shunsuke/pinact/v4/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/controller/updaterepos"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/di"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/github"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

const formatJSON = "json"

func New(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command {
	r := &runner{}
	return r.Command(logger, globalFlags, env)
}

type runner struct {
	organizations, repositories, includeRepos, excludeRepos, includeActions, excludeActions, branchToTags []string
	includeForks, includeArchived, dryRun, keepWorkdir, update, draft                                     bool
	branch, baseBranch, prTitle, prBody, format                                                           string
	concurrency, minAge                                                                                   int
}

func (r *runner) Command(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command {
	return &cli.Command{
		Name:  "update-repos",
		Usage: "Pin GitHub Actions across repositories and create pull requests",
		Action: func(ctx context.Context, _ *cli.Command) error {
			return r.action(ctx, logger, globalFlags, env)
		},
		Flags: r.flags(),
	}
}

func (r *runner) flags() []cli.Flag {
	return []cli.Flag{
		&cli.StringSliceFlag{Name: "org", Destination: &r.organizations},
		&cli.StringSliceFlag{Name: "repo", Destination: &r.repositories},
		&cli.StringSliceFlag{Name: "include-repo", Destination: &r.includeRepos},
		&cli.StringSliceFlag{Name: "exclude-repo", Destination: &r.excludeRepos},
		&cli.BoolFlag{Name: "include-forks", Destination: &r.includeForks},
		&cli.BoolFlag{Name: "include-archived", Destination: &r.includeArchived},
		&cli.StringFlag{Name: "branch", Destination: &r.branch},
		&cli.StringFlag{Name: "base-branch", Destination: &r.baseBranch},
		&cli.StringFlag{Name: "pr-title", Destination: &r.prTitle},
		&cli.StringFlag{Name: "pr-body", Destination: &r.prBody},
		&cli.BoolFlag{Name: "draft", Destination: &r.draft},
		&cli.BoolFlag{Name: "dry-run", Destination: &r.dryRun},
		&cli.BoolFlag{Name: "keep-workdir", Destination: &r.keepWorkdir},
		&cli.IntFlag{Name: "concurrency", Value: updaterepos.DefaultConcurrency, Destination: &r.concurrency},
		&cli.BoolFlag{Name: "update", Destination: &r.update},
		&cli.StringSliceFlag{Name: "include-action", Destination: &r.includeActions},
		&cli.StringSliceFlag{Name: "exclude-action", Destination: &r.excludeActions},
		&cli.StringSliceFlag{Name: "branch-to-tag", Destination: &r.branchToTags},
		&cli.IntFlag{Name: "min-age", Destination: &r.minAge},
		&cli.StringFlag{Name: "format", Destination: &r.format},
	}
}

func (r *runner) action(ctx context.Context, logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) error {
	param, err := r.param()
	if err != nil {
		return err
	}
	secrets := &di.Secrets{}
	secrets.SetFromEnv(env.Getenv)
	diFlags := &di.Flags{GlobalFlags: globalFlags}
	di.SetEnv(diFlags, env.Getenv)
	token, err := github.Token(ctx, logger.Logger, secrets.GitHubToken, diFlags.KeyringEnabled)
	if err != nil {
		return fmt.Errorf("get GitHub token: %w", err)
	}
	if token == "" {
		return errors.New("a GitHub token is required to push branches and create pull requests")
	}
	client, err := newGitHubClient(ctx, logger, diFlags, token)
	if err != nil {
		return err
	}
	param.Token = token
	param.GHESToken = secrets.GHESToken
	param.Flags = diFlags
	controller := updaterepos.New(client, param, os.Stdout, os.Stderr)
	if _, err := controller.Run(ctx, logger.Logger); err != nil {
		return fmt.Errorf("update repositories: %w", err)
	}
	return nil
}

func (r *runner) param() (updaterepos.Param, error) {
	if len(r.organizations) == 0 && len(r.repositories) == 0 {
		return updaterepos.Param{}, errors.New("at least one --org or --repo is required")
	}
	if r.concurrency < 1 {
		return updaterepos.Param{}, errors.New("--concurrency must be at least 1")
	}
	if r.format != "" && r.format != formatJSON {
		return updaterepos.Param{}, errors.New("--format must be 'json'")
	}
	filters, err := r.compileFilters()
	if err != nil {
		return updaterepos.Param{}, err
	}
	return updaterepos.Param{
		Organizations:       r.organizations,
		Repositories:        r.repositories,
		IncludeRepositories: filters.includeRepos,
		ExcludeRepositories: filters.excludeRepos,
		IncludeForks:        r.includeForks,
		IncludeArchived:     r.includeArchived,
		Branch:              r.branch,
		BaseBranch:          r.baseBranch,
		PRTitle:             r.prTitle,
		PRBody:              r.prBody,
		Draft:               r.draft,
		DryRun:              r.dryRun,
		KeepWorkdir:         r.keepWorkdir,
		Concurrency:         r.concurrency,
		Update:              r.update,
		IncludeActions:      filters.includeActions,
		ExcludeActions:      filters.excludeActions,
		BranchToTags:        filters.branchToTags,
		MinAge:              r.minAge,
		Format:              r.format,
	}, nil
}

type filterRegexps struct {
	includeRepos, excludeRepos, includeActions, excludeActions, branchToTags []*regexp.Regexp
}

func (r *runner) compileFilters() (filterRegexps, error) {
	fields := [][]string{r.includeRepos, r.excludeRepos, r.includeActions, r.excludeActions, r.branchToTags}
	compiled := make([][]*regexp.Regexp, len(fields))
	for i, values := range fields {
		re, err := compileRegexps(values)
		if err != nil {
			return filterRegexps{}, err
		}
		compiled[i] = re
	}
	return filterRegexps{
		includeRepos: compiled[0], excludeRepos: compiled[1],
		includeActions: compiled[2], excludeActions: compiled[3],
		branchToTags: compiled[4],
	}, nil
}

func newGitHubClient(ctx context.Context, logger *slogutil.Logger, flags *di.Flags, token string) (*github.Client, error) {
	if apiURL := flags.GetAPIURL(); apiURL != "" {
		client, err := github.NewWithBaseURL(ctx, apiURL, token)
		if err != nil {
			return nil, fmt.Errorf("create GitHub Enterprise client: %w", err)
		}
		return client, nil
	}
	client, err := github.New(ctx, logger.Logger, token, false)
	if err != nil {
		return nil, fmt.Errorf("create GitHub client: %w", err)
	}
	return client, nil
}

func compileRegexps(values []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, len(values))
	for i, value := range values {
		re, err := regexp.Compile(value)
		if err != nil {
			return nil, fmt.Errorf("compile regexp %q: %w", value, err)
		}
		out[i] = re
	}
	return out, nil
}
