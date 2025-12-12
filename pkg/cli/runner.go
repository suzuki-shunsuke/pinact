// Package cli provides the command-line interface layer for pinact.
// This package serves as the main entry point for all CLI operations,
// handling command parsing, flag processing, and routing to appropriate subcommands.
// It orchestrates the overall CLI structure using urfave/cli framework and delegates
// actual business logic to controller packages.
package cli

import (
	"context"

	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/initcmd"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/token"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

// Run creates and executes the main pinact CLI application.
// It configures the command structure with global flags and subcommands,
// then runs the CLI with the provided arguments.
func Run(ctx context.Context, logger *slogutil.Logger, env *urfave.Env) error {
	globalFlags := &flag.GlobalFlags{}
	return urfave.Command(env, &cli.Command{ //nolint:wrapcheck
		Name:  "pinact",
		Usage: "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
		Flags: globalFlags.Flags(),
		Commands: []*cli.Command{
			initcmd.New(logger, globalFlags),
			run.New(logger, globalFlags, env),
			migrate.New(logger, globalFlags),
			token.New(logger),
		},
	}).Run(ctx, env.Args)
}
