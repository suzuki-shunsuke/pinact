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

	"github.com/suzuki-shunsuke/pinact/v4/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v4/pkg/di"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

// warnDeprecatedFlags writes a warning to stderr for v3 flag usages whose v4
// behavior differs from v3 in a way the user might not expect.
//
// -check, -verify (-v), and -diff (true) keep working as silent aliases for
// their v4 equivalents (see di.buildParam), so no warning is emitted for them.
//
// -diff=false is the one exception: in v3 it suppressed diff output, but in
// v4 detail output is always printed. The flag value is silently ignored and
// a warning surfaces the difference.
func warnDeprecatedFlags(cmd *cli.Command, w io.Writer) {
	if cmd.IsSet("diff") && !cmd.Bool("diff") {
		fmt.Fprintln(w, "WARN: -diff=false is ignored in v4: detail output is always printed")
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
			// Setting -min-age (either explicitly on the CLI or via
			// PINACT_MIN_AGE, which urfave wires into the same flag through
			// Sources) is an explicit signal that the user wants the passive
			// audit to run, so auto-enable -verify-min-age. Machine-wide
			// defaults that should NOT enable the audit belong in the global
			// config file's min_age.value.
			if cmd.IsSet("min-age") {
				flags.VerifyMinAge = true
			}
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
				Aliases:     []string{"verify", "v"},
				Usage:       "Verify that the version comment matches the pinned SHA",
				Destination: &flags.VerifyComment,
			},
			&cli.BoolFlag{
				Name:        "verify-min-age",
				Usage:       "Audit every pinned action against the min-age threshold (calls the GitHub API). Auto-enabled when -min-age is set on the CLI",
				Destination: &flags.VerifyMinAge,
			},
			&cli.BoolFlag{
				Name:        "no-api",
				Usage:       "Skip GitHub API calls. Only the syntactic pin check (40-character SHA) is performed",
				Destination: &flags.NoAPI,
			},
			&cli.BoolFlag{
				Name:        "check",
				Usage:       "Alias for -fix=false. For offline check use -fix=false -no-api",
				Destination: &flags.Check,
			},
			&cli.BoolFlag{
				Name:        "update",
				Aliases:     []string{"u"},
				Usage:       "Update actions to latest versions",
				Destination: &flags.Update,
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
				Usage:       "Alias for -fix=false. Note: -diff=false is ignored because detail output is always printed in v4",
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
				Usage:       "Minimum release age threshold in days. Setting this (either via CLI or PINACT_MIN_AGE) implicitly enables -verify-min-age",
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
			&cli.StringFlag{
				Name:        "diff-file",
				Usage:       "Path to a unified diff. Only the `+` lines of the diff are processed (use `-` to read the diff from stdin). Useful in PR CI to limit pinact to lines changed by the PR",
				Destination: &flags.DiffFile,
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
