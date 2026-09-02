// Package run implements the 'pinact run' command, the core functionality of pinact.
// This package orchestrates the main pinning process for GitHub Actions and reusable workflows,
// including version resolution, SHA pinning, update operations, and pull request review creation.
// It handles various modes of operation (check, diff, fix, update, review) and integrates
// with GitHub Actions CI environment for automated processing. The package also manages
// include/exclude patterns for selective action processing and coordinates with the
// controller layer to perform the actual file modifications.
package run

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/di"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

const description = `If no argument is passed, pinact searches GitHub Actions workflow files from .github/workflows.

$ pinact run

You can also pass workflow file paths as arguments.

e.g.

$ pinact run .github/actions/foo/action.yaml .github/actions/bar/action.yaml
`

// warnDeprecatedFlags writes a warning to stderr for v3 flag usages whose v4
// behavior differs from v3 in a way the user might not expect.
//
// --check, --verify (-v), and --diff (true) keep working as silent aliases for
// their v4 equivalents (see di.buildParam), so no warning is emitted for them.
//
// --diff=false is the one exception: in v3 it suppressed diff output, but in
// v4 detail output is always printed. The flag value is silently ignored and
// a warning surfaces the difference.
func warnDeprecatedFlags(cmd *cobra.Command, diff bool, w io.Writer) {
	if cmd.Flags().Changed("diff") && !diff {
		fmt.Fprintln(w, "WARN: --diff=false is ignored in v4: detail output is always printed")
	}
}

// New creates a new run command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for pinning GitHub Actions versions.
func New(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *cobrautil.Env) *cobra.Command {
	r := &runner{}
	return r.Command(logger, globalFlags, env)
}

type runner struct{}

// normalizeFlagName maps the multi-character flag aliases urfave/cli supported onto
// the flags they name. pflag shorthands are a single character, so --verify and
// --sep have nowhere else to live; the single-character aliases are registered as
// ordinary shorthands.
func normalizeFlagName(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "verify":
		name = "verify-comment"
	case "sep":
		name = "separator"
	}
	return pflag.NormalizedName(name)
}

// validate checks the flag values cobra can't, in place of urfave/cli's per-flag
// Validator. It runs before anything else in the action, so an invalid value is
// still reported before any work is done.
func validate(flags *di.Flags) error {
	if flags.Format != "" && flags.Format != "sarif" {
		return errors.New("--format must be 'sarif'")
	}
	if flags.MinAge < 0 {
		return errors.New("--min-age must be a non-negative integer")
	}
	return nil
}

// Command builds and returns the run CLI command configuration.
// It defines all flags, options, and the action handler for the run subcommand.
// This command handles the core pinning functionality with various modes
// like check, diff, fix, update, and review.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags, env *cobrautil.Env) *cobra.Command {
	flags := &di.Flags{GlobalFlags: globalFlags}
	cmd := &cobra.Command{
		Use:   "run [<workflow file>...]",
		Short: "Pin GitHub Actions versions",
		Long:  description,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validate(flags); err != nil {
				return err
			}
			warnDeprecatedFlags(cmd, flags.Diff, env.Stderr)
			// Setting --min-age (either explicitly on the CLI or via
			// PINACT_MIN_AGE, which cobrautil.ApplyEnvs sets on the same flag
			// and so marks as changed) is an explicit signal that the user
			// wants the passive audit to run, so auto-enable --verify-min-age.
			// Machine-wide defaults that should NOT enable the audit belong in
			// the global config file's min_age.value.
			if cmd.Flags().Changed("min-age") {
				flags.VerifyMinAge = true
			}
			// urfave/cli counted how often --fix was given; di.Run only asks
			// whether it was given at all, which is what Changed reports.
			if cmd.Flags().Changed("fix") {
				flags.FixCount = 1
			}
			flags.Args = args
			pwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get the current directory: %w", err)
			}
			flags.CWD = pwd
			di.SetEnv(flags, env.Getenv)
			secrets := &di.Secrets{}
			secrets.SetFromEnv(env.Getenv)
			return di.Run(cmd.Context(), logger, flags, secrets, env.Getenv)
		},
	}
	fs := cmd.Flags()
	fs.SetNormalizeFunc(normalizeFlagName)
	fs.BoolVarP(&flags.VerifyComment, "verify-comment", "v", false,
		"Verify that the version comment matches the pinned SHA")
	fs.BoolVar(&flags.VerifyMinAge, "verify-min-age", false,
		"Audit every pinned action against the min-age threshold (calls the GitHub API). Auto-enabled when --min-age is set on the CLI")
	fs.BoolVar(&flags.NoAPI, "no-api", false,
		"Skip GitHub API calls. Only the syntactic pin check (40-character SHA) is performed")
	fs.BoolVar(&flags.Check, "check", false,
		"Alias for --fix=false. For offline check use --fix=false --no-api")
	fs.BoolVarP(&flags.Update, "update", "u", false, "Update actions to latest versions")
	fs.BoolVar(&flags.Fix, "fix", false,
		"Fix code. By default, this is true. If --check or --diff is true, this is false by default")
	fs.BoolVar(&flags.Diff, "diff", false,
		"Alias for --fix=false. Note: --diff=false is ignored because detail output is always printed in v4")
	fs.StringVar(&flags.Format, "format", "",
		"Output format. Currently only 'sarif' is supported. If sarif is specified, results are output in SARIF format to stdout")
	fs.StringSliceVarP(&flags.Include, "include", "i", nil, "A regular expression to fix actions")
	fs.StringSliceVarP(&flags.Exclude, "exclude", "e", nil, "A regular expression to exclude actions")
	fs.StringSliceVar(&flags.BranchToTag, "branch-to-tag", nil,
		"A regular expression to convert non-semver versions (e.g. branch names) to the latest stable tag. Anchor with ^$ for exact match")
	fs.IntVarP(&flags.MinAge, "min-age", "m", 0,
		"Minimum release age threshold in days. Setting this (either via CLI or PINACT_MIN_AGE) implicitly enables --verify-min-age")
	cobrautil.Envs(fs, "min-age", "PINACT_MIN_AGE")
	fs.StringVar(&flags.Separator, "separator", "", "Separator between version and tag comment")
	// No backquotes in the usage: pflag reads the first backquoted word as the
	// name of the flag's argument, which turned "`+`" into the placeholder.
	fs.StringVar(&flags.DiffFile, "diff-file", "",
		`Path to a unified diff. Only the '+' lines of the diff are processed (use '-' to read the diff from stdin). Useful in PR CI to limit pinact to lines changed by the PR`)
	return cmd
}
