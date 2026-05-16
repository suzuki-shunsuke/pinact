// Package di provides dependency injection for the pinact CLI.
// It creates and wires together all the dependencies needed to run the pinact commands.
package di

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/go-error-with-exit-code/ecerror"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/github"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

// formatSarif is the only -format value supported by pinact.
const formatSarif = "sarif"

type ghesServices struct {
	repoService *github.RepositoriesServiceImpl
	gitService  *github.GitServiceImpl
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
	cfg.Separator = getSeparator(cfg, flags, getEnv)
	if err := validateSeparator(cfg.Separator); err != nil {
		return err
	}

	param, err := buildParam(flags)
	if err != nil {
		return err
	}
	services, err := setupGHESServices(ctx, gh, cfg, flags, secrets.GHESToken)
	if err != nil {
		return err
	}

	ctrl := run.New(services.repoService, services.gitService, fs, cfg, param)
	return ctrl.Run(ctx, logger.Logger) //nolint:wrapcheck
}

func getSeparator(cfg *config.Config, flags *Flags, getEnv func(string) string) string {
	if flags.Separator != "" {
		return flags.Separator
	}
	if cfg.Separator != "" {
		return cfg.Separator
	}
	if s := getEnv("PINACT_SEPARATOR"); s != "" {
		return s
	}
	return " # "
}

var separatorPattern = regexp.MustCompile(` +# +(?:tag=)?`)

func validateSeparator(sep string) error {
	if !separatorPattern.MatchString(sep) {
		return fmt.Errorf("separator must match the regular expression `%s`", separatorPattern.String())
	}
	return nil
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

func buildParam(flags *Flags) (*run.ParamRun, error) {
	if err := validateFlagCombo(flags); err != nil {
		return nil, err
	}
	includes, err := compileRegexps(flags.Include)
	if err != nil {
		return nil, fmt.Errorf("parse include: %w", err)
	}
	excludes, err := compileRegexps(flags.Exclude)
	if err != nil {
		return nil, fmt.Errorf("parse exclude: %w", err)
	}
	branchToTags, err := compileRegexps(flags.BranchToTag)
	if err != nil {
		return nil, fmt.Errorf("parse branch-to-tag: %w", err)
	}
	param := &run.ParamRun{
		WorkflowFilePaths: flags.Args,
		ConfigFilePath:    flags.Config,
		CWD:               flags.CWD,
		IsVerify:          flags.VerifyComment,
		Check:             flags.Check,
		Update:            flags.Update,
		Diff:              flags.Diff,
		NoAPI:             flags.NoAPI,
		Fix:               true,
		IsGitHubActions:   flags.IsGitHubActions,
		Stderr:            os.Stderr,
		Stdout:            os.Stdout,
		Includes:          includes,
		Excludes:          excludes,
		BranchToTags:      branchToTags,
		MinAge:            flags.MinAge,
		Now:               time.Now(),
		Format:            flags.Format,
	}
	// Fix default is true. The legacy -check and -diff flags act as aliases for
	// -fix=false (v4 spec). -format sarif also implies fix=false by default.
	// Explicit -fix wins over all of these.
	switch {
	case flags.FixCount > 0:
		param.Fix = flags.Fix
	case param.Check || param.Diff:
		param.Fix = false
	case param.Format == formatSarif:
		param.Fix = false
	}
	return param, nil
}

// validateFlagCombo enforces invalid CLI flag combinations defined by the v4 spec.
//
// Returned errors are wrapped with exit code 3 (the "unexpected / misuse"
// class in the v4 exit code spec) so the CLI surfaces it via ecerror.
func validateFlagCombo(flags *Flags) error {
	if err := validateUpdateFix(flags); err != nil {
		return err
	}
	if flags.NoAPI {
		return validateNoAPI(flags)
	}
	return nil
}

// validateUpdateFix rejects -update -fix=false (modification is implied by
// -update). -format sarif acts as "report without writing", so it is allowed
// to coexist with -update -fix=false.
func validateUpdateFix(flags *Flags) error {
	if flags.Update && flags.FixCount > 0 && !flags.Fix && flags.Format != formatSarif {
		return ecerror.Wrap(
			errors.New("-update cannot be combined with -fix=false (use -format sarif to report update candidates without writing files)"),
			run.ExitCodeAPIError,
		)
	}
	return nil
}

// validateNoAPI rejects -no-api combinations that cannot be satisfied without
// a GitHub API call: discovering the latest version (-update), comparing the
// pinned SHA against its version comment (-verify-comment), and resolving any
// non-SHA reference to a SHA (the default -fix=true).
func validateNoAPI(flags *Flags) error {
	if flags.Update {
		return ecerror.Wrap(
			errors.New("-no-api cannot be combined with -update (update needs the GitHub API to discover the latest version)"),
			run.ExitCodeAPIError,
		)
	}
	if flags.VerifyComment {
		return ecerror.Wrap(
			errors.New("-no-api cannot be combined with -verify-comment (verify needs the GitHub API to compare the SHA)"),
			run.ExitCodeAPIError,
		)
	}
	// -no-api with the default fix mode would silently skip every action it
	// cannot resolve via the syntactic check. Require an explicit opt-out via
	// -fix=false or -format sarif.
	fixExplicitlyFalse := flags.FixCount > 0 && !flags.Fix
	if !fixExplicitlyFalse && flags.Format != formatSarif {
		return ecerror.Wrap(
			errors.New("-no-api requires -fix=false (or -format sarif)"),
			run.ExitCodeAPIError,
		)
	}
	return nil
}
