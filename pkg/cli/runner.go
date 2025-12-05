// Package cli provides the command-line interface layer for pinact.
// This package serves as the main entry point for all CLI operations,
// handling command parsing, flag processing, and routing to appropriate subcommands.
// It orchestrates the overall CLI structure using urfave/cli framework and delegates
// actual business logic to controller packages.
package cli

import (
	"context"
	"log/slog"

	"github.com/suzuki-shunsuke/go-stdutil"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/initcmd"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/migrate"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/run"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/token"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/urfave"
	"github.com/urfave/cli/v3"
)

// Run creates and executes the main pinact CLI application.
// It configures the command structure with global flags and subcommands,
// then runs the CLI with the provided arguments.
//
// Parameters:
//   - ctx: context for cancellation and timeout control
//   - logger: slog logger for structured logging
//   - logLevelVar: slog level variable for dynamic log level changes
//   - ldFlags: linker flags containing build information
//   - args: command line arguments to parse and execute
//
// Returns an error if command parsing or execution fails.
func Run(ctx context.Context, logger *slog.Logger, logLevelVar *slog.LevelVar, ldFlags *stdutil.LDFlags, args ...string) error {
	return urfave.Command(ldFlags, &cli.Command{ //nolint:wrapcheck
		Name:  "pinact",
		Usage: "Pin GitHub Actions versions. https://github.com/suzuki-shunsuke/pinact",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "log level",
				Sources: cli.EnvVars("PINACT_LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name: "config",
				Aliases: []string{
					"c",
				},
				Usage:   "configuration file path",
				Sources: cli.EnvVars("PINACT_CONFIG"),
			},
		},
		Commands: []*cli.Command{
			initcmd.New(logger, logLevelVar),
			run.New(logger, logLevelVar),
			migrate.New(logger, logLevelVar),
			token.New(logger),
		},
	}).Run(ctx, args)
}
