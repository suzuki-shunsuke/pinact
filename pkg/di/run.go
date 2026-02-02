// Package di provides dependency injection for the pinact CLI.
// It creates and wires together all the dependencies needed to run the pinact commands.
package di

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

type ghesServices struct {
	repoService *github.RepositoriesServiceImpl
	gitService  *github.GitServiceImpl
	prService   *github.PullRequestsServiceImpl
}

// Run executes the main run command logic.
// It configures logging, processes GitHub Actions context, parses includes/excludes,
// sets up the controller, and executes the pinning operation.
func Run(ctx context.Context, logger *slogutil.Logger, flags *Flags, secrets *Secrets, getEnv func(string) string) error {
	if flags.IsGitHubActions {
		color.NoColor = false
	}
	if err := logger.SetLevel(flags.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}

	gh := github.New(ctx, logger.Logger, secrets.GitHubToken, flags.KeyringEnabled, flags.GHTKNEnabled)
	fs := afero.NewOsFs()

	cfg, err := readConfig(fs, flags.Config)
	if err != nil {
		return err
	}
	if flags.Separator != "" {
		cfg.Separator = flags.Separator
	}
	if cfg.Separator == "" {
		cfg.Separator = getEnv("PINACT_SEPARATOR")
	}
	if !strings.Contains(cfg.Separator, "#") {
		return errors.New("separator must contain '#'")
	}
	if !strings.HasPrefix(cfg.Separator, " ") {
		return errors.New("separator must start with space ' '")
	}

	review := setupReview(fs, logger, flags)

	param, err := buildParam(flags, review)
	if err != nil {
		return err
	}
	services, err := setupGHESServices(ctx, gh, cfg, flags, secrets.GHESToken)
	if err != nil {
		return err
	}

	ctrl := run.New(services.repoService, services.prService, services.gitService, fs, cfg, param)
	return ctrl.Run(ctx, logger.Logger) //nolint:wrapcheck
}

func readConfig(fs afero.Fs, configFilePath string) (*config.Config, error) {
	cfgFinder := config.NewFinder(fs)
	cfgReader := config.NewReader(fs)
	configPath, err := cfgFinder.Find(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("find configuration file: %w", err)
	}
	cfg := &config.Config{}
	if err := cfgReader.Read(cfg, configPath); err != nil {
		return nil, fmt.Errorf("read configuration file: %w", err)
	}
	return cfg, nil
}

func compileRegexps(opts []string) ([]*regexp.Regexp, error) {
	regexps := make([]*regexp.Regexp, len(opts))
	for i, include := range opts {
		r, err := regexp.Compile(include)
		if err != nil {
			return nil, fmt.Errorf("compile a regexp: %w", slogerr.With(err, "regexp", include))
		}
		regexps[i] = r
	}
	return regexps, nil
}

func buildParam(flags *Flags, review *run.Review) (*run.ParamRun, error) {
	includes, err := compileRegexps(flags.Include)
	if err != nil {
		return nil, fmt.Errorf("parse include: %w", err)
	}
	excludes, err := compileRegexps(flags.Exclude)
	if err != nil {
		return nil, fmt.Errorf("parse exclude: %w", err)
	}
	param := &run.ParamRun{
		WorkflowFilePaths: flags.Args,
		ConfigFilePath:    flags.Config,
		PWD:               flags.PWD,
		IsVerify:          flags.Verify,
		Check:             flags.Check,
		Update:            flags.Update,
		Diff:              flags.Diff,
		Fix:               true,
		IsGitHubActions:   flags.IsGitHubActions,
		Stderr:            os.Stderr,
		Stdout:            os.Stdout,
		Review:            review,
		Includes:          includes,
		Excludes:          excludes,
		MinAge:            flags.MinAge,
		Now:               time.Now(),
		Format:            flags.Format,
	}
	if flags.FixCount > 0 {
		param.Fix = flags.Fix
	} else if param.Check || param.Diff {
		param.Fix = false
	}
	return param, nil
}
