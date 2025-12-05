// Package migrate implements the 'pinact migrate' command.
// This package handles the migration of pinact configuration files between
// different schema versions. It ensures smooth upgrades when pinact introduces
// new configuration formats or features, allowing users to automatically
// update their .pinact.yaml files to the latest schema version.
package migrate

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/migrate"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	"github.com/urfave/cli/v3"
)

type runner struct {
	logger      *slog.Logger
	logLevelVar *slog.LevelVar
}

// New creates a new migrate command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for migrating pinact configuration files.
//
// Parameters:
//   - logger: slog logger for structured logging
//   - logLevelVar: slog level variable for dynamic log level changes
//
// Returns a pointer to the configured CLI command.
func New(logger *slog.Logger, logLevelVar *slog.LevelVar) *cli.Command {
	r := runner{
		logger:      logger,
		logLevelVar: logLevelVar,
	}
	return r.Command()
}

// Command builds and returns the migrate CLI command configuration.
// It defines the command name, usage description, and action handler
// for the migrate subcommand.
//
// Returns a pointer to the configured CLI command.
func (r *runner) Command() *cli.Command {
	return &cli.Command{
		Name:  "migrate",
		Usage: "Migrate .pinact.yaml",
		Description: `Migrate the version of .pinact.yaml

$ pinact migrate
`,
		Action: r.action,
	}
}

// action executes the migrate command logic.
// It configures logging, creates the filesystem interface and controller,
// then performs the configuration file migration.
//
// Parameters:
//   - _: context (unused in this implementation)
//   - c: CLI command containing parsed flags and arguments
//
// Returns an error if migration fails or logging configuration fails.
func (r *runner) action(_ context.Context, c *cli.Command) error {
	if err := slogutil.SetLevel(r.logLevelVar, c.String("log-level")); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	fs := afero.NewOsFs()
	ctrl := migrate.New(fs, config.NewFinder(fs), &migrate.Param{
		ConfigFilePath: c.String("config"),
	})

	return ctrl.Migrate(r.logger) //nolint:wrapcheck
}
