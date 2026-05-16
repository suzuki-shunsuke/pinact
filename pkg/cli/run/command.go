// Package run implements the 'pinact run' command, the core functionality of pinact.
// This package orchestrates the main pinning process for GitHub Actions and reusable workflows,
// including version resolution, SHA pinning, update operations, and pull request review creation.
// It handles various modes of operation (check, diff, fix, update, review) and integrates
// with GitHub Actions CI environment for automated processing. The package also manages
// include/exclude patterns for selective action processing and coordinates with the
// controller layer to perform the actual file modifications.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/di"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

// warnDeprecatedFlags writes a deprecation warning to stderr for each v3-only
// flag the user passed. The flags themselves still function as aliases for
// their v4 equivalents (see di.buildParam). These warnings are scheduled to be
// removed when the aliases are dropped in a future major release.
func warnDeprecatedFlags(cmd *cli.Command, w io.Writer) {
	if cmd.IsSet("check") {
		fmt.Fprintln(w, "WARN: --check is deprecated; use -fix=false (and -no-api for offline check) instead")
	}
	if cmd.IsSet("diff") {
		fmt.Fprintln(w, "WARN: --diff is deprecated; details are now always printed, so -fix=false alone is enough")
	}
	if cmd.IsSet("verify") {
		fmt.Fprintln(w, "WARN: --verify is deprecated; use -verify-comment instead")
	}
}

// New creates a new run command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for pinning GitHub Actions versions.
func New(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command {
	r := &runner{}
	return r.Command(logger, globalFlags, env)
}

type runner struct{}

// Command builds and returns the run CLI command configuration.
// It defines all flags, options, and the action handler for the run subcommand.
// This command handles the core pinning functionality with various modes
// like check, diff, fix, update, and review.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *urfave.Env) *cli.Command { //nolint:funlen
	flags := &di.Flags{GlobalFlags: globalFlags}
	return &cli.Command{
		Name:  "run",
		Usage: "Pin GitHub Actions versions",
		Description: `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			warnDeprecatedFlags(cmd, env.Stderr)
			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get the current directory: %w", err)
			}
			flags.CWD = pwd
			di.SetEnv(flags, env.Getenv)
			secrets := &di.Secrets{}
			secrets.SetFromEnv(env.Getenv)
			return di.Run(ctx, logger, flags, secrets, env.Getenv)
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:        "verify-comment",
				Usage:       "Verify that the version comment matches the pinned SHA (the v4 name for --verify)",
				Destination: &flags.VerifyComment,
			},
			&cli.BoolFlag{
				Name:        "verify",
				Aliases:     []string{"v"},
				Usage:       "[DEPRECATED] Alias for -verify-comment; will be removed in a future major release",
				Destination: &flags.Verify,
			},
			&cli.BoolFlag{
				Name:        "no-api",
				Usage:       "Skip GitHub API calls. In v4.0 only the syntactic pin check is performed; cache support (v4.1+) will enable offline fix/verify",
				Destination: &flags.NoAPI,
			},
			&cli.BoolFlag{
				Name:        "check",
				Usage:       "[DEPRECATED] Alias for -fix=false; will be removed in a future major release. For offline check use -fix=false -no-api",
				Destination: &flags.Check,
			},
			&cli.BoolFlag{
				Name:        "update",
				Aliases:     []string{"u"},
				Usage:       "Update actions to latest versions",
				Destination: &flags.Update,
			},
			&cli.BoolFlag{
				Name:        "review",
				Usage:       "Create reviews",
				Destination: &flags.Review,
			},
			&cli.BoolFlag{
				Name:        "fix",
				Usage:       "Fix code. By default, this is true. If -check or -diff is true, this is false by default",
				Destination: &flags.Fix,
				Config: cli.BoolConfig{
					Count: &flags.FixCount,
				},
			},
			&cli.BoolFlag{
				Name:        "diff",
				Usage:       "[DEPRECATED] Alias for -fix=false; will be removed in a future major release. Details are now always printed",
				Destination: &flags.Diff,
			},
			&cli.StringFlag{
				Name:        "format",
				Usage:       "Output format. Currently only 'sarif' is supported. If sarif is specified, results are output in SARIF format to stdout",
				Destination: &flags.Format,
				Validator: func(s string) error {
					if s != "" && s != "sarif" {
						return errors.New("--format must be 'sarif'")
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "repo-owner",
				Usage:       "GitHub repository owner",
				Sources:     cli.EnvVars("GITHUB_REPOSITORY_OWNER"),
				Destination: &flags.RepoOwner,
			},
			&cli.StringFlag{
				Name:        "repo-name",
				Usage:       "GitHub repository name",
				Destination: &flags.RepoName,
			},
			&cli.StringFlag{
				Name:        "sha",
				Usage:       "Commit SHA to be reviewed",
				Destination: &flags.SHA,
			},
			&cli.IntFlag{
				Name:        "pr",
				Usage:       "GitHub pull request number",
				Destination: &flags.PR,
			},
			&cli.StringSliceFlag{
				Name:        "include",
				Aliases:     []string{"i"},
				Usage:       "A regular expression to fix actions",
				Destination: &flags.Include,
			},
			&cli.StringSliceFlag{
				Name:        "exclude",
				Aliases:     []string{"e"},
				Usage:       "A regular expression to exclude actions",
				Destination: &flags.Exclude,
			},
			&cli.StringSliceFlag{
				Name:        "branch-to-tag",
				Usage:       "A regular expression to convert non-semver versions (e.g. branch names) to the latest stable tag. Anchor with ^$ for exact match",
				Destination: &flags.BranchToTag,
			},
			&cli.IntFlag{
				Name:        "min-age",
				Aliases:     []string{"m"},
				Usage:       "Skip versions released within the specified number of days (requires -u or --branch-to-tag)",
				Destination: &flags.MinAge,
				Sources:     cli.EnvVars("PINACT_MIN_AGE"),
				Validator: func(i int) error {
					if i < 0 {
						return errors.New("--min-age must be a non-negative integer")
					}
					return nil
				},
			},
			&cli.StringFlag{
				Name:        "separator",
				Aliases:     []string{"sep"},
				Usage:       "Separator between version and tag comment",
				Destination: &flags.Separator,
			},
		},
		Arguments: []cli.Argument{
			&cli.StringArgs{
				Name:        "files",
				Max:         -1,
				Destination: &flags.Args,
			},
		},
	}
}
