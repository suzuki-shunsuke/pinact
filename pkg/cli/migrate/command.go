// Package migrate implements the 'pinact migrate' command.
// This package handles the migration of pinact configuration files between
// different schema versions. It ensures smooth upgrades when pinact introduces
// new configuration formats or features, allowing users to automatically
// update their .pinact.yaml files to the latest schema version.
package migrate

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/cli/flag"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/migrate"
	"github.com/suzuki-shunsuke/slog-util/slogutil"
	urfavecli "github.com/urfave/cli/v3"
)

type runner struct{}

// New creates a new migrate command for the CLI.
// It initializes a runner with the provided logger and returns
// the configured CLI command for migrating pinact configuration files.
// Returns a pointer to the configured CLI command.
func New(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *urfavecli.Command {
	r := runner{}
	return r.Command(logger, globalFlags)
}

// Command builds and returns the migrate CLI command configuration.
// It defines the command name, usage description, and action handler
// for the migrate subcommand.
//
// Returns a pointer to the configured CLI command.
func (r *runner) Command(logger *slogutil.Logger, globalFlags *flag.GlobalFlags) *urfavecli.Command {
	return &urfavecli.Command{
		Name:  "migrate",
		Usage: "Migrate .pinact.yaml",
		Description: `Migrate the version of .pinact.yaml

$ pinact migrate
`,
		Action: func(_ context.Context, _ *urfavecli.Command) error {
			return r.action(logger, globalFlags)
		},
	}
}

// action executes the migrate command logic.
// It configures logging, creates the filesystem interface and controller,
// then performs the configuration file migration.
// Returns an error if migration fails or logging configuration fails.
func (r *runner) action(logger *slogutil.Logger, flags *flag.GlobalFlags) error {
	if err := logger.SetLevel(flags.LogLevel); err != nil {
		return fmt.Errorf("set log level: %w", err)
	}
	fs := afero.NewOsFs()
	ctrl := migrate.New(fs, config.NewFinder(fs), &migrate.Param{
		ConfigFilePath: flags.Config,
	})

	return ctrl.Migrate(logger.Logger) //nolint:wrapcheck
}
