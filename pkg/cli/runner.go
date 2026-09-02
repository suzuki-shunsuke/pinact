// Package cli provides the command-line interface layer for pinact.
// This package serves as the main entry point for all CLI operations,
// handling command parsing, flag processing, and routing to appropriate subcommands.
// It orchestrates the overall CLI structure using cobra and delegates
// actual business logic to controller packages.
package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/suzuki-shunsuke/cobra-util/cobrautil"
	"github.com/suzuki-shunsuke/cobra-util/jsonschema"
	"github.com/suzuki-shunsuke/go-error-with-exit-code/ecerror"
	schema "github.com/suzuki-shunsuke/pinact/v5/json-schema"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/docs"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/initcmd"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/run"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/tokencmd"
	"github.com/suzuki-shunsuke/slog-error/slogerr"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
)

// Run creates and executes the main pinact CLI application.
// It configures the command structure with global flags and subcommands,
// then runs the CLI with the provided arguments.
func Run(ctx context.Context, logger *slogutil.Logger, env *cobrautil.Env) error {
	globalFlags := &gflag.GlobalFlags{}
	// The context reaches the commands through ExecuteContext below, which is what
	// cobra hands to the action as cmd.Context(); building the tree needs none.
	cmd := newCommand(logger, env, globalFlags) //nolint:contextcheck
	// The program name is not an argument to cobra, and cobrautil.Command has
	// already stripped it from what the command will parse.
	if len(env.Args) > 1 {
		if err := checkSingleDashLongFlags(cmd, env.Args[1:]); err != nil {
			return withHelp(err)
		}
	}
	if err := cmd.ExecuteContext(ctx); err != nil {
		return withHelp(err)
	}
	return nil
}

// withHelp adds the hint about the documentation to an error while keeping any exit
// code it carries. Every error pinact reports goes through here, because an error is
// where a coding agent arrives without knowing that pinact ships its documentation
// in the binary; without this it troubleshoots from the source code or the website
// instead.
//
// The routine outcomes of `pinact run` are unaffected: they are reported as
// cobrautil.ErrSilent carrying the exit code, whose message is empty, so Main logs
// nothing for them and the hint is attached to nothing.
//
// The code has to be read before slogerr.With and attached again afterwards, because
// slogerr.With collapses the chain to the innermost error holding attributes and
// drops every wrapper around it, including the one holding the code. Exit codes are
// what tells pinning from a GitHub API failure, so losing one turns a 2 into a 1.
func withHelp(err error) error {
	code := ecerror.GetExitCode(err)
	err = slogerr.With(err, "help",
		"Run `pinact docs list` to see documentation and `pinact docs show <name>` to read it; this may help resolve the error.")
	if code == 1 {
		// 1 is also what an error carrying no code exits with, so there is nothing to
		// preserve.
		return err //nolint:wrapcheck // The error is the command's own, only annotated.
	}
	return ecerror.Wrap(err, code)
}

// newCommand builds the command tree. It is separate from Run so that a test can run
// the real tree with its output captured, which Run can't offer: the commands write
// to os.Stdout.
func newCommand(logger *slogutil.Logger, env *cobrautil.Env, globalFlags *gflag.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pinact",
		Short: "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
		Long: `Pin GitHub Actions and Reusable Workflows.

pinact pins the actions and reusable workflows a workflow uses to full commit
SHAs with a version comment, updates them, and verifies that a comment names the
version its SHA actually is.

See each subcommand's help with 'pinact help <command>'.
See https://github.com/suzuki-shunsuke/pinact for details.

If you are a coding agent, run 'pinact docs list' to list the documentation and
'pinact docs show <name>' to read it before answering questions about pinact or
troubleshooting its errors.`,
	}
	globalFlags.Set(cmd.PersistentFlags())
	cmd.AddCommand(
		initcmd.New(logger, globalFlags, env),
		run.New(logger, globalFlags, env),
		migrate.New(logger, globalFlags),
		tokencmd.New(logger),
		docs.New(),
	)
	// json-schema is added on its own because With takes the name of the program
	// from cmd, which is what the example in the command's help says.
	jsonschema.With(cmd, schema.Schema)
	return cobrautil.Command(env, cmd, &cobrautil.Options{
		AfterVersion: func() {
			hintDocsOnVersion(logger, globalFlags)
		},
	})
}

// hintDocsOnVersion makes `pinact -v`, `pinact --version`, and `pinact version` log
// docs.Hint. Checking the version is often the only pinact command a coding agent
// runs before it starts answering, so without this it never learns that
// `pinact docs` exists and reads the source code or the website instead.
//
// The hint goes to stderr as a log rather than to stdout so that it doesn't break
// scripts that parse the version, and it's logged at the info level so that
// `--log-level warn` silences it.
//
// PINACT_LOG_LEVEL needs no special handling: cobrautil prints the version from the
// root command's action, which runs after the environment variables have been
// applied to the flags, so the log level is already resolved here.
func hintDocsOnVersion(logger *slogutil.Logger, globalFlags *gflag.GlobalFlags) {
	// An invalid log level is ignored here because the version output must succeed
	// regardless. Subcommands report it when they set the level.
	_ = logger.SetLevel(globalFlags.LogLevel)
	logger.Info(docs.Hint())
}
