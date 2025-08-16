// Package migrate implements the 'pinact migrate' command.
// This package handles the migration of pinact configuration files between
// different schema versions. It ensures smooth upgrades when pinact introduces
// new configuration formats or features, allowing users to automatically
// update their .pinact.yaml files to the latest schema version.
package migrate

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/config"
	"github.com/suzuki-shunsuke/pinact/v3/pkg/controller/migrate"
	"github.com/suzuki-shunsuke/urfave-cli-v3-util/log"
	"github.com/urfave/cli/v3"
)

type runner struct {
	logE *logrus.Entry
}

func New(logE *logrus.Entry) *cli.Command {
	r := runner{
		logE: logE,
	}
	return r.Command()
}

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

func (r *runner) action(_ context.Context, c *cli.Command) error {
	if err := log.Set(r.logE, c.String("log-level"), "auto"); err != nil {
		return fmt.Errorf("configure logger: %w", err)
	}
	fs := afero.NewOsFs()
	ctrl := migrate.New(fs, config.NewFinder(fs), &migrate.Param{
		ConfigFilePath: c.String("config"),
	})

	return ctrl.Migrate(r.logE) //nolint:wrapcheck
}
