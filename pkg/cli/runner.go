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
	schema "github.com/suzuki-shunsuke/pinact/v5/json-schema"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/docs"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/gflag"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/initcmd"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/run"
	"github.com/suzuki-shunsuke/pinact/v5/pkg/cli/tokencmd"
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
			return err
		}
	}
	return cmd.ExecuteContext(ctx) //nolint:wrapcheck
}

// newCommand builds the command tree. It is separate from Run so that a test can run
// the real tree with its output captured, which Run can't offer: the commands write
// to os.Stdout.
func newCommand(logger *slogutil.Logger, env *cobrautil.Env, globalFlags *gflag.GlobalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pinact",
		Short: "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
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
